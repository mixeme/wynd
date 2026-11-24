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

/** Мост выбранного `.wynd` (план 50, 5.4): только главный фрейм выбранного origin. */
final class ArchiveBridge {
    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private boolean destroyed;

    static ArchiveBridge attach(Activity activity, WebView web, ServerOrigin origin) {
        ArchiveBridge bridge = new ArchiveBridge(activity, web, origin);
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return bridge;
        WebViewCompat.addWebMessageListener(web, "__wyndArchive",
                Collections.singleton(origin.url().substring(0, origin.url().length() - 1)),
                bridge::onMessage);
        return bridge;
    }

    private ArchiveBridge(Activity activity, WebView web, ServerOrigin origin) {
        this.activity = activity;
        this.web = web;
        this.origin = origin;
    }

    private void onMessage(WebView view, WebMessageCompat message, Uri source, boolean mainFrame,
                           JavaScriptReplyProxy reply) {
        if (destroyed || !mainFrame || !origin.contains(source.toString())
                || !origin.contains(web.getUrl())
                || message.getType() != WebMessageCompat.TYPE_STRING) return;
        String raw = message.getData();
        if (raw == null || raw.length() > 2048) return;
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
                case "status":
                    response.put("archive", ArchiveStore.status(activity, origin));
                    break;
                case "open":
                    response.put("archive", ArchiveStore.open(activity, origin));
                    break;
                case "forget":
                    ArchiveStore.forget(activity);
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
                // Страница получит тайм-аут.
            }
        }
    }

    private void post(JavaScriptReplyProxy reply, String answer, int id) {
        if (destroyed || id <= 0 || activity.isFinishing() || activity.isDestroyed()) return;
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        try {
            reply.postMessage(answer);
        } catch (RuntimeException ignored) {
            // Страница могла уйти.
        }
    }

    void destroy() {
        destroyed = true;
        worker.shutdownNow();
    }
}
