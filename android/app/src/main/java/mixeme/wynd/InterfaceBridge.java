package mixeme.wynd;

import android.util.AtomicFile;
import android.webkit.WebView;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONObject;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Самостоятельный origin-bound протокол источников, без изменения протокола обновлений. */
final class InterfaceBridge {
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private volatile boolean destroyed, switching;

    static InterfaceCatalog open(MainActivity activity) throws Exception {
        AtomicFile file = new AtomicFile(new File(activity.getFilesDir(), "interface-servers.json"));
        return new InterfaceCatalog(new InterfaceCatalog.Storage() {
            public String read() throws IOException {
                if (!file.getBaseFile().exists() && !new File(file.getBaseFile() + ".bak").exists()) return null;
                byte[] bytes = file.readFully();
                if (bytes.length > 65536) throw new IOException("Interface catalog too large");
                return new String(bytes, StandardCharsets.UTF_8);
            }
            public void write(String value) throws IOException {
                FileOutputStream output = file.startWrite();
                try {
                    output.write(value.getBytes(StandardCharsets.UTF_8));
                    file.finishWrite(output);
                } catch (IOException | RuntimeException error) { file.failWrite(output); throw error; }
            }
        }, BuildConfig.DEBUG, BuildConfig.START_URL);
    }

    InterfaceBridge(MainActivity activity, WebView view, ServerOrigin origin, InterfaceCatalog catalog) {
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        String rule = origin.url().substring(0, origin.url().length() - 1);
        WebViewCompat.addWebMessageListener(view, "__wyndInterfaces", Collections.singleton(rule),
                (web, message, source, mainFrame, reply) -> {
                    if (destroyed || switching || !mainFrame || !origin.contains(source.toString())
                            || !origin.contains(web.getUrl()) || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 2048) return;
                    try {
                        JSONObject request = new JSONObject(raw);
                        int id = request.getInt("id");
                        if (id < 1) return;
                        String op = request.getString("op");
                        // Проверяем действия на UI-потоке до постановки в очередь.
                        boolean blocked = op.equals("select") && activity.interfaceSwitchBlocked();
                        worker.execute(() -> {
                            if (destroyed || switching) return;
                            JSONObject response = new JSONObject();
                            boolean selected = false;
                            boolean startup = false;
                            try {
                                response.put("id", id).put("protocol", 1);
                                if (blocked) throw new IOException("busy");
                                switch (op) {
                                    case "list": break;
                                    case "preview": response.put("preview", InterfaceServer.check(request.getString("address"), BuildConfig.DEBUG)); break;
                                    case "add": add(catalog, InterfaceServer.check(request.getString("address"), BuildConfig.DEBUG)); break;
                                    case "remove":
                                        synchronized (this) {
                                            if (destroyed || switching) return;
                                            catalog.remove(request.getString("origin"));
                                        }
                                        break;
                                    case "startup":
                                        // С экрана входа назад к вводу адреса (план 50):
                                        // стартовый экран оболочки, черновик вернёт он сам.
                                        startup = !activity.interfaceSwitchBlocked();
                                        if (!startup) throw new IOException("busy");
                                        break;
                                    case "select":
                                        String target = request.getString("origin");
                                        if (!catalog.contains(target)) throw new IOException("unknown");
                                        JSONObject checked = InterfaceServer.check(target, BuildConfig.DEBUG);
                                        add(catalog, checked);
                                        // Повторная проверка незавершённых действий перед записью выбора — на UI-потоке ниже.
                                        selected = true;
                                        break;
                                    default: throw new IOException("unknown");
                                }
                                response.put("catalog", catalog.snapshot());
                            } catch (Exception error) {
                                try { response.put("error", blocked ? "busy" : "failed"); }
                                catch (org.json.JSONException ignored) { }
                            }
                            final boolean choose = selected && !response.has("error");
                            final boolean openStartup = startup && !response.has("error");
                            activity.runOnUiThread(() -> {
                                if (destroyed || switching || activity.isFinishing() || activity.isDestroyed() || !origin.contains(web.getUrl())) return;
                                if (openStartup) {
                                    switching = true;
                                    activity.showStartup(null);
                                    return;
                                }
                                try {
                                    if (choose) {
                                        if (activity.interfaceSwitchBlocked()) response.put("error", "busy");
                                        else {
                                            synchronized (this) {
                                                if (destroyed || switching) return;
                                                catalog.select(request.getString("origin"));
                                                switching = true;
                                                response.put("catalog", catalog.snapshot());
                                            }
                                        }
                                    }
                                    reply.postMessage(response.toString());
                                    if (switching) web.postDelayed(() -> {
                                        if (!destroyed && !activity.isFinishing() && !activity.isDestroyed()) activity.restartInterface();
                                    }, 150);
                                } catch (Exception ignored) {
                                    try { reply.postMessage(new JSONObject().put("id", id).put("protocol", 1).put("error", "failed").toString()); }
                                    catch (Exception gone) { }
                                }
                            });
                        });
                    } catch (Exception ignored) { }
                });
    }
    private synchronized void add(InterfaceCatalog catalog, JSONObject checked) throws Exception {
        if (destroyed || switching) throw new IOException("Inactive interface");
        catalog.add(checked);
    }
    synchronized void destroy() { destroyed = true; worker.shutdownNow(); }
}
