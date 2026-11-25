package mixeme.wynd;

import android.app.Activity;
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
 * Мост общего хранилища входов (C127): отдаёт записи только выбранному
 * источнику интерфейса, как и остальные мосты оболочки.
 */
final class SecretsBridge {
    private static final String TAG = "WyndSecrets";
    private static final int MAX_REQUEST = 8192;

    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final SecretsStore store;
    private volatile boolean destroyed;

    static SecretsBridge attach(Activity activity, WebView web, ServerOrigin origin) {
        SecretsStore store;
        try {
            store = SecretsStore.open(activity, BuildConfig.DEBUG);
        } catch (Exception error) {
            // Повреждённый файл не должен мешать открыть интерфейс: входы
            // восстановятся при следующем входе.
            Log.w(TAG, "secrets store unavailable", error);
            return null;
        }
        return new SecretsBridge(activity, web, origin, store);
    }

    private SecretsBridge(Activity activity, WebView web, ServerOrigin origin, SecretsStore store) {
        this.activity = activity;
        this.web = web;
        this.origin = origin;
        this.store = store;
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        String rule = origin.url().substring(0, origin.url().length() - 1);
        WebViewCompat.addWebMessageListener(web, "__wyndSecrets",
                Collections.singleton(rule),
                (view, message, source, mainFrame, reply) -> {
                    if (destroyed || !mainFrame || !trusted() || !origin.contains(source.toString())
                            || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > MAX_REQUEST) return;
                    int id = -1;
                    try {
                        JSONObject request = new JSONObject(raw);
                        id = request.getInt("id");
                        if (id < 1) return;
                        respond(id, reply, run(request));
                    } catch (JSONException | java.io.IOException | RuntimeException error) {
                        if (id > 0) respond(id, reply, null);
                    }
                });
    }

    private JSONObject run(JSONObject request) throws JSONException, java.io.IOException {
        String operation = request.getString("op");
        JSONObject result = new JSONObject();
        switch (operation) {
            case "list" -> result.put("sessions", store.list());
            case "put" -> store.put(request.getJSONObject("session"));
            case "delete" -> store.remove(request.getString("origin"));
            default -> throw new JSONException("Unknown secrets operation");
        }
        return result;
    }

    private void respond(int id, JavaScriptReplyProxy reply, JSONObject result) {
        if (!trusted() || !WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        try {
            JSONObject response = new JSONObject().put("id", id);
            if (result == null) response.put("error", "secrets_unavailable");
            else response.put("result", result);
            reply.postMessage(response.toString());
        } catch (JSONException | RuntimeException ignored) {
            /* The document may have been replaced. */
        }
    }

    private boolean trusted() {
        return !destroyed && !activity.isFinishing() && !activity.isDestroyed()
                && origin.contains(web.getUrl());
    }

    void destroy() {
        destroyed = true;
    }
}
