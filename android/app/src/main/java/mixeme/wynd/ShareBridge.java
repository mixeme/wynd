package mixeme.wynd;

import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.util.Log;
import android.webkit.WebView;
import androidx.webkit.JavaScriptReplyProxy;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONException;
import org.json.JSONObject;
import java.util.Collections;

/**
 * Системный лист «Поделиться» для ссылок страницы (C159): в WebView нет
 * navigator.share, поэтому страница зовёт мост, а лист показывает система.
 */
final class ShareBridge {
    private static final String TAG = "WyndAndroid";
    private static final int MAX_TITLE = 200;
    private final Activity activity;
    private final WebView view;
    private final ServerOrigin server;
    private boolean destroyed;

    private ShareBridge(Activity activity, WebView view, ServerOrigin server) {
        this.activity = activity;
        this.view = view;
        this.server = server;
    }

    static ShareBridge attach(Activity activity, WebView view, ServerOrigin origin) {
        ShareBridge bridge = new ShareBridge(activity, view, origin);
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return bridge;
        String rule = origin.url().substring(0, origin.url().length() - 1);
        WebViewCompat.addWebMessageListener(view, "__wyndShare", Collections.singleton(rule),
                (web, message, source, main, reply) -> bridge.receive(message, source.toString(), main, reply));
        return bridge;
    }

    private void receive(WebMessageCompat message, String source, boolean main, JavaScriptReplyProxy reply) {
        if (destroyed || !main || !server.contains(source) || !server.contains(view.getUrl())
                || activity.isFinishing() || activity.isDestroyed()
                || message.getType() != WebMessageCompat.TYPE_STRING) return;
        String raw = message.getData();
        if (raw == null || raw.length() > 4096) return;
        try {
            JSONObject request = new JSONObject(raw);
            int id = request.getInt("id");
            if (id < 1) return;
            String url = request.optString("url");
            // Только обычная веб-ссылка: лист не должен передавать чужим
            // приложениям file:, content: или intent: адреса страницы.
            if (!ServerOrigin.isExternalWebLink(url)) {
                answer(id, false, reply);
                return;
            }
            String title = request.optString("title");
            if (title.length() > MAX_TITLE) title = title.substring(0, MAX_TITLE);
            answer(id, share(url, title), reply);
        } catch (JSONException | RuntimeException ignored) {
            // Чужое или повреждённое сообщение не открывает лист.
        }
    }

    private void answer(int id, boolean shared, JavaScriptReplyProxy reply) {
        try { reply.postMessage(new JSONObject().put("id", id).put("shared", shared).toString()); }
        catch (JSONException | RuntimeException ignored) { /* Страница уже ушла. */ }
    }

    private boolean share(String url, String title) {
        Intent send = new Intent(Intent.ACTION_SEND).setType("text/plain")
                .putExtra(Intent.EXTRA_TEXT, url);
        if (!title.isEmpty()) send.putExtra(Intent.EXTRA_SUBJECT, title);
        try {
            activity.startActivity(Intent.createChooser(send, null));
            return true;
        } catch (ActivityNotFoundException | SecurityException e) {
            Log.w(TAG, "No share sheet available");
            return false;
        }
    }

    void destroy() { destroyed = true; }
}
