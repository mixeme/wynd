package mixeme.wynd;

import android.app.Activity;
import android.webkit.WebView;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONObject;
import java.util.Collections;

/** Только главный фрейм выбранного источника; никакого addJavascriptInterface. */
final class AndroidBridge {
    static void attach(Activity activity, WebView view, ServerOrigin origin) {
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        String rule = origin.url().substring(0, origin.url().length() - 1);
        UpdateController updates = UpdateController.get(activity);
        WebViewCompat.addWebMessageListener(view, "__wyndAndroid", Collections.singleton(rule),
                (webView, message, sourceOrigin, isMainFrame, reply) -> {
                    if (!isMainFrame || !origin.contains(sourceOrigin.toString())) return;
                    if (message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 1024) return;
                    try {
                        JSONObject request = new JSONObject(raw);
                        int id = request.getInt("id");
                        if (id < 1) return;
                        String method = request.getString("method");
                        if (method.equals("install")) {
                            updates.install(activity);
                            reply.postMessage(new JSONObject().put("id", id).put("result", updates.status()).toString());
                            return;
                        }
                        updates.worker.execute(() -> {
                            try {
                                switch (method) {
                                    case "status": break;
                                    case "check": updates.check(); break;
                                    case "prepare": updates.prepare(); break;
                                    case "setImmediately": updates.setImmediately(request.getBoolean("value")); updates.check(); break;
                                    default: throw new IllegalArgumentException("Unknown capability");
                                }
                            } catch (Exception e) {
                                updates.failure(e);
                            }
                            activity.runOnUiThread(() -> {
                                if (activity.isFinishing() || activity.isDestroyed()) return;
                                try { reply.postMessage(new JSONObject().put("id", id).put("result", updates.status()).toString()); }
                                catch (RuntimeException | org.json.JSONException ignored) { /* Страница могла уйти. */ }
                            });
                        });
                    } catch (org.json.JSONException | RuntimeException ignored) { /* Чужие или повреждённые сообщения не исполняем. */ }
                });
    }
}
