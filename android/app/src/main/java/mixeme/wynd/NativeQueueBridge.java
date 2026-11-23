package mixeme.wynd;

import android.app.Activity;
import android.net.Uri;
import android.webkit.WebView;

import androidx.webkit.JavaScriptReplyProxy;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;

import org.json.JSONObject;

import java.util.Collections;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Мост очереди (план 50, этап 5.1): страница отдаёт запись без вложений и
 * токен, native хранит её и отправляет через WorkManager. Только главный
 * фрейм выбранного origin; токен и тело дальше private-файла не уходят.
 */
final class NativeQueueBridge {
    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final NativeQueue queue;
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private final NativeQueue.Listener listener = this::changed;
    private boolean destroyed;

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
        if (raw == null || raw.length() > 300_000) return;
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
                final String answer = new JSONObject().put("id", replyId).put("error", "invalid").toString();
                activity.runOnUiThread(() -> post(reply, answer, replyId));
            } catch (Exception ignored) {
                // Ответ уже не собрать — страница получит тайм-аут.
            }
        }
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
        queue.clearListener(listener);
        worker.shutdownNow();
    }
}
