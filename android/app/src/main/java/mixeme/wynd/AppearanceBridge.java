package mixeme.wynd;

import android.app.Activity;
import android.webkit.WebView;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONException;
import org.json.JSONObject;
import java.util.Collections;

/** Отдельная возможность цвета окна: только главный документ выбранного origin. */
final class AppearanceBridge {
    static void attach(Activity activity, WebView view, ServerOrigin origin, SystemBarsFrame bars) {
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        String rule = origin.url().substring(0, origin.url().length() - 1);
        WebViewCompat.addWebMessageListener(view, "__wyndAppearance", Collections.singleton(rule),
                (web, message, source, main, reply) -> {
                    if (!main || !origin.contains(source.toString()) || !origin.contains(view.getUrl())
                            || activity.isFinishing() || activity.isDestroyed() || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 256) return;
                    try {
                        JSONObject request = new JSONObject(raw);
                        String status = request.getString("status"), navigation = request.getString("navigation");
                        if (!BarColors.valid(status) || !BarColors.valid(navigation)) return;
                        bars.colors(status, navigation);
                        reply.postMessage(bars.state().toString());
                    } catch (JSONException | RuntimeException ignored) { /* Некорректный цвет или ушедшая страница. */ }
                });
    }
}
