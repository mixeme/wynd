package mixeme.wynd;

import android.os.SystemClock;
import android.webkit.WebView;
import androidx.webkit.JavaScriptReplyProxy;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONObject;
import java.util.Collections;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Выбор проверенного origin. Входы и данные WebView остаются при прежних адресах. */
final class InterfaceBridge {
    private final MainActivity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private volatile long generation;
    private long checkedAt;
    private String ticket;
    private ServerOrigin checked;
    private boolean destroyed, resumed;

    InterfaceBridge(MainActivity activity, WebView web, ServerOrigin origin) {
        this.activity = activity; this.web = web; this.origin = origin;
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        WebViewCompat.addWebMessageListener(web, "__wyndInterface",
                Collections.singleton(origin.url().substring(0, origin.url().length() - 1)),
                (view, message, source, main, reply) -> {
                    if (!main || !origin.contains(source.toString()) || !trusted()
                            || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 8192) return;
                    try {
                        JSONObject request = new JSONObject(raw); int id = request.getInt("id");
                        if (id < 1) return;
                        switch (request.getString("method")) {
                            case "status": respond(reply, id, new JSONObject().put("origin", origin.url()), ""); break;
                            case "cancel": invalidate(); respond(reply, id, new JSONObject(), ""); break;
                            case "check": check(request.getString("address"), id, reply); break;
                            case "select": select(request, id, reply); break;
                            default: respond(reply, id, null, "unsupported");
                        }
                    } catch (Exception ignored) { /* Повреждённый запрос не меняет источник. */ }
                });
    }

    private boolean trusted() {
        return resumed && !destroyed && !activity.isFinishing() && !activity.isDestroyed()
                && origin.contains(web.getUrl());
    }

    private void check(String address, int id, JavaScriptReplyProxy reply) {
        invalidate(); final long requestGeneration = generation;
        worker.execute(() -> {
            if (Thread.currentThread().isInterrupted() || requestGeneration != generation) return;
            ServerOrigin candidate = null; JSONObject card = null;
            try {
                candidate = ServerOrigin.parse(address, BuildConfig.DEBUG);
                card = InterfaceProbe.check(candidate);
            } catch (Exception ignored) { /* Один отказ для адреса, сети и чужого сайта (7.44). */ }
            final ServerOrigin resultOrigin = candidate; final JSONObject result = card;
            activity.runOnUiThread(() -> {
                if (destroyed || requestGeneration != generation) return;
                if (result == null) { respond(reply, id, null, "unavailable"); return; }
                checked = resultOrigin; ticket = UUID.randomUUID().toString(); checkedAt = SystemClock.elapsedRealtime();
                try { respond(reply, id, result.put("ticket", ticket), ""); }
                catch (Exception ignored) { invalidate(); }
            });
        });
    }

    private void select(JSONObject request, int id, JavaScriptReplyProxy reply) throws Exception {
        if (checked == null || ticket == null || !ticket.equals(request.optString("ticket"))
                || SystemClock.elapsedRealtime() - checkedAt > 120000) {
            invalidate(); respond(reply, id, null, "expired"); return;
        }
        String path = request.optString("path", "/");
        if (!validPath(path)) { respond(reply, id, null, "path"); return; }
        if (activity.interfaceBusy()) { respond(reply, id, null, "busy"); return; }
        ServerOrigin next = checked;
        if (!activity.getSharedPreferences("interface", 0).edit().putString("origin", next.url()).commit()) {
            respond(reply, id, null, "storage"); return;
        }
        invalidate();
        respond(reply, id, new JSONObject().put("selected", true), "");
        activity.openInterface(next, path);
    }

    // Только корень или существующие пути приглашения; токен не идёт в probe и preferences.
    static boolean validPath(String path) {
        return path.equals("/") || path.length() <= 4096
                && path.matches("/(invite|join)/[A-Za-z0-9_-]+(?:\\?[A-Za-z0-9_~%=&.+-]*)?");
    }

    private void respond(JavaScriptReplyProxy reply, int id, JSONObject result, String error) {
        if (!trusted() || !WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        try { reply.postMessage(new JSONObject().put("protocol", 1).put("id", id)
                .put("result", result == null ? JSONObject.NULL : result).put("error", error).toString()); }
        catch (Exception ignored) { /* Документ мог уйти. */ }
    }
    void invalidate() { generation++; checked = null; ticket = null; }
    void resume() { resumed = true; }
    void pause() { resumed = false; }
    void destroy() { destroyed = true; invalidate(); worker.shutdownNow(); }
}
