package mixeme.wynd;

import android.webkit.WebView;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONObject;
import java.io.IOException;
import java.util.Collections;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Доступен только встроенному главному документу; серверная страница его не получает. */
final class StartupBridge {
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private volatile boolean destroyed;
    private volatile int revision;
    private StartupTarget checked;
    private JSONObject preview;
    private long checkedAt;

    StartupBridge(MainActivity activity, WebView view, InterfaceCatalog catalog, String failedUrl) {
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        ServerOrigin local = ServerOrigin.parse(StartupAssets.ORIGIN, false);
        WebViewCompat.addWebMessageListener(view, "__wyndStartup", Collections.singleton(StartupAssets.ORIGIN),
                (web, message, source, mainFrame, reply) -> {
                    if (destroyed || !mainFrame || !local.contains(source.toString()) || !local.contains(web.getUrl())
                            || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 8192) return;
                    try {
                        JSONObject request = new JSONObject(raw);
                        int id = request.getInt("id");
                        if (id < 1) return;
                        String op = request.getString("op");
                        // Любое новое действие отменяет право предыдущей проверки изменить выбор.
                        int current = ++revision;
                        worker.execute(() -> {
                            if (destroyed || current != revision) return;
                            JSONObject response = new JSONObject();
                            StartupTarget open = null;
                            try {
                                response.put("id", id).put("protocol", 1);
                                switch (op) {
                                    case "state":
                                        JSONObject snapshot = catalog.snapshot();
                                        response.put("offline", failedUrl != null);
                                        response.put("selected", JSONObject.NULL);
                                        for (int i = 0; i < snapshot.getJSONArray("servers").length(); i++) {
                                            JSONObject item = snapshot.getJSONArray("servers").getJSONObject(i);
                                            if (item.getString("origin").equals(snapshot.getString("selected"))) response.put("selected", item);
                                        }
                                        break;
                                    case "cancel": checked = null; preview = null; break;
                                    case "check":
                                        checked = null; preview = null;
                                        StartupTarget target = StartupTarget.parse(request.getString("address"), BuildConfig.DEBUG);
                                        if (local.contains(target.url())) throw new IOException("Local address");
                                        JSONObject info = InterfaceServer.check(target.origin().url(), BuildConfig.DEBUG);
                                        if (destroyed || current != revision) return;
                                        checked = target; preview = info; checkedAt = android.os.SystemClock.elapsedRealtime();
                                        response.put("preview", info);
                                        break;
                                    case "open":
                                        StartupTarget requested = StartupTarget.parse(request.getString("address"), BuildConfig.DEBUG);
                                        if (checked == null || preview == null || !checked.url().equals(requested.url())
                                                || android.os.SystemClock.elapsedRealtime() - checkedAt > 60000) throw new IOException("Unchecked target");
                                        open = checked;
                                        break;
                                    case "retry":
                                        if (failedUrl == null) throw new IOException("No unavailable interface");
                                        StartupTarget retry = StartupTarget.parse(failedUrl, BuildConfig.DEBUG);
                                        // Ошибка произвольной серверной страницы возвращает к корню того же источника.
                                        InterfaceServer.check(retry.origin().url(), BuildConfig.DEBUG);
                                        open = retry;
                                        break;
                                    default: throw new IOException("Unknown startup operation");
                                }
                            } catch (Exception error) {
                                open = null;
                                try { response.put("error", "failed"); } catch (Exception ignored) { }
                            }
                            StartupTarget destination = open;
                            activity.runOnUiThread(() -> {
                                if (destroyed || current != revision || activity.isFinishing() || activity.isDestroyed() || !local.contains(web.getUrl())) return;
                                try {
                                    if (destination != null) {
                                        String nextOrigin = destination.origin().url();
                                        nextOrigin = nextOrigin.substring(0, nextOrigin.length() - 1);
                                        if (!nextOrigin.equals(catalog.selected()) && activity.interfaceSwitchBlocked()) throw new IOException("Busy");
                                        if (op.equals("open")) catalog.choose(preview);
                                    }
                                    reply.postMessage(response.toString());
                                    if (destination != null) activity.openStartupTarget(destination.url());
                                } catch (Exception error) {
                                    try { reply.postMessage(new JSONObject().put("id", id).put("protocol", 1).put("error", "failed").toString()); }
                                    catch (Exception ignored) { }
                                }
                            });
                        });
                    } catch (Exception ignored) { }
                });
    }
    void destroy() { destroyed = true; ++revision; worker.shutdownNow(); }
}
