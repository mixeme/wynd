package mixeme.wynd;

import android.app.Activity;
import android.net.Uri;
import android.util.Base64;
import android.webkit.WebView;

import androidx.webkit.JavaScriptReplyProxy;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.File;
import java.io.FileOutputStream;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Мост очереди (план 50, этап 5): страница отдаёт запись и токен, native
 * хранит её и отправляет через WorkManager. 5.1 — запись без вложений;
 * 5.2 — файлы, постеры и обложки: begin → chunk* → commit. Только главный
 * фрейм выбранного origin; токен и тело дальше private-каталога не уходят.
 */
final class NativeQueueBridge {
    private static final int MAX_CHUNK = 128 * 1024;
    private static final int MAX_RAW = 300_000;
    private static final long PENDING_TTL_MS = 10 * 60 * 1000;

    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final NativeQueue queue;
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private final Map<String, Pending> pending = new HashMap<>();
    private final NativeQueue.Listener listener = this::changed;
    private boolean destroyed;

    private static final class Pending {
        final String id;
        final JSONObject request;
        final List<NativeQueue.Part> files;
        final List<NativeQueue.Part> aux;
        final long startedAt;

        Pending(String id, JSONObject request, List<NativeQueue.Part> files,
                List<NativeQueue.Part> aux) {
            this.id = id;
            this.request = request;
            this.files = files;
            this.aux = aux;
            this.startedAt = System.currentTimeMillis();
        }

        List<NativeQueue.Part> parts() {
            List<NativeQueue.Part> all = new ArrayList<>(files.size() + aux.size());
            all.addAll(files);
            all.addAll(aux);
            return all;
        }
    }

    static NativeQueueBridge attach(Activity activity, WebView web, ServerOrigin origin) {
        NativeQueue queue = NativeQueue.get(activity);
        NativeQueueBridge bridge = new NativeQueueBridge(activity, web, origin, queue);
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return bridge;
        WebViewCompat.addWebMessageListener(web, "__wyndQueue",
                Collections.singleton(origin.url().substring(0, origin.url().length() - 1)),
                bridge::onMessage);
        queue.setListener(bridge.listener);
        return bridge;
    }

    private NativeQueueBridge(Activity activity, WebView web, ServerOrigin origin, NativeQueue queue) {
        this.activity = activity;
        this.web = web;
        this.origin = origin;
        this.queue = queue;
    }

    private void onMessage(WebView view, WebMessageCompat message, Uri source, boolean mainFrame,
                           JavaScriptReplyProxy reply) {
        if (destroyed || !mainFrame || !origin.contains(source.toString())
                || !origin.contains(web.getUrl())
                || message.getType() != WebMessageCompat.TYPE_STRING) return;
        String raw = message.getData();
        if (raw == null || raw.length() > MAX_RAW) return;
        worker.execute(() -> handle(raw, reply));
    }

    private void handle(String raw, JavaScriptReplyProxy reply) {
        int id = 0;
        try {
            JSONObject request = new JSONObject(raw);
            id = request.getInt("id");
            if (id <= 0) return;
            String operation = request.getString("op");
            JSONObject response = new JSONObject().put("id", id);
            switch (operation) {
                case "enqueue":
                    response.put("native_id", queue.add(request));
                    NativeQueueWorker.schedule(activity);
                    break;
                case "begin":
                    begin(request, response);
                    break;
                case "chunk":
                    chunk(request);
                    response.put("ok", true);
                    break;
                case "commit":
                    commit(request, response);
                    break;
                case "abort":
                    abort(request.getString("token"));
                    response.put("ok", true);
                    break;
                case "cancel":
                    queue.cancel(request.getString("client_id"));
                    response.put("ok", true);
                    break;
                case "status":
                    response = queue.status().put("id", id);
                    break;
                case "clear":
                    queue.clear(request.getString("origin"));
                    response.put("ok", true);
                    break;
                default:
                    throw new IllegalArgumentException("unknown");
            }
            final String answer = response.toString();
            final int replyId = id;
            activity.runOnUiThread(() -> post(reply, answer, replyId));
        } catch (Exception error) {
            final int replyId = id;
            try {
                final String answer = new JSONObject().put("id", replyId)
                        .put("error", "invalid").toString();
                activity.runOnUiThread(() -> post(reply, answer, replyId));
            } catch (Exception ignored) {
                // Ответ уже не собрать — страница получит тайм-аут.
            }
        }
    }

    private void begin(JSONObject request, JSONObject response) throws Exception {
        expirePending();
        JSONArray filesJson = request.optJSONArray("files");
        JSONArray auxJson = request.optJSONArray("aux");
        List<NativeQueue.Part> files = new ArrayList<>();
        List<NativeQueue.Part> aux = new ArrayList<>();
        if (filesJson != null) {
            for (int i = 0; i < filesJson.length(); i++) {
                files.add(NativeQueue.Part.main(filesJson.getJSONObject(i), i));
            }
        }
        if (auxJson != null) {
            for (int i = 0; i < auxJson.length(); i++) {
                aux.add(NativeQueue.Part.aux(auxJson.getJSONObject(i), i));
            }
        }
        // Проверка типа/mediа/токена до записи файлов на диск.
        NativeQueue.Record.fromWithFiles(request, files, aux, BuildConfig.DEBUG);
        String token = queue.begin();
        pending.put(token, new Pending(token, request, files, aux));
        response.put("token", token);
    }

    private void chunk(JSONObject request) throws Exception {
        String token = request.getString("token");
        Pending handoff = pending.get(token);
        if (handoff == null) throw new IllegalArgumentException("token");
        int index = request.getInt("index");
        long offset = request.getLong("offset");
        if (index < 0 || index >= handoff.parts().size()) throw new IllegalArgumentException("index");
        NativeQueue.Part part = handoff.parts().get(index);
        byte[] bytes = Base64.decode(request.getString("data"), Base64.DEFAULT);
        if (bytes.length == 0 || bytes.length > MAX_CHUNK
                || offset < 0 || offset + bytes.length > part.size) {
            throw new IllegalArgumentException("chunk");
        }
        File file = queue.partFile(handoff.id, part);
        if (file == null || file.length() != offset) throw new IllegalArgumentException("offset");
        try (FileOutputStream output = new FileOutputStream(file, true)) {
            output.write(bytes);
        }
    }

    private void commit(JSONObject request, JSONObject response) throws Exception {
        String token = request.getString("token");
        Pending handoff = pending.get(token);
        if (handoff == null) throw new IllegalArgumentException("token");
        for (NativeQueue.Part part : handoff.parts()) {
            File file = queue.partFile(handoff.id, part);
            if (file == null || !file.isFile() || file.length() != part.size) {
                throw new IllegalArgumentException("incomplete");
            }
        }
        try {
            String nativeId = queue.commit(handoff.id, handoff.request,
                    handoff.files, handoff.aux);
            pending.remove(token);
            response.put("native_id", nativeId);
            NativeQueueWorker.schedule(activity);
        } catch (Exception error) {
            abort(token);
            throw error;
        }
    }

    private void abort(String token) {
        Pending handoff = pending.remove(token);
        if (handoff != null) queue.abort(handoff.id);
    }

    private void expirePending() {
        long cutoff = System.currentTimeMillis() - PENDING_TTL_MS;
        List<String> stale = new ArrayList<>();
        for (Map.Entry<String, Pending> entry : pending.entrySet()) {
            if (entry.getValue().startedAt < cutoff) stale.add(entry.getKey());
        }
        for (String token : stale) abort(token);
    }

    private void post(JavaScriptReplyProxy reply, String answer, int id) {
        if (destroyed || id <= 0 || activity.isFinishing() || activity.isDestroyed()) return;
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        try {
            reply.postMessage(answer);
        } catch (RuntimeException ignored) {
            // Страница могла уйти; запись остаётся в очереди.
        }
    }

    private void changed() {
        if (destroyed) return;
        web.post(() -> {
            if (!destroyed && !activity.isFinishing() && !activity.isDestroyed()
                    && origin.contains(web.getUrl())) {
                web.evaluateJavascript("window.dispatchEvent(new Event('wynd:native-queue'))", null);
            }
        });
    }

    void destroy() {
        destroyed = true;
        for (String token : new ArrayList<>(pending.keySet())) abort(token);
        queue.clearListener(listener);
        worker.shutdownNow();
    }
}
