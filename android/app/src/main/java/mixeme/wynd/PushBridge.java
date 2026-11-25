package mixeme.wynd;

import android.Manifest;
import android.app.Activity;
import android.os.Build;
import android.webkit.WebView;

import androidx.webkit.JavaScriptReplyProxy;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;

import org.json.JSONException;
import org.json.JSONObject;
import org.unifiedpush.android.connector.UnifiedPush;

import java.lang.ref.WeakReference;
import java.util.Collections;

/** Exposes a registration only to the selected, top-level Wynd interface. */
final class PushBridge {
    static final int PERMISSION_REQUEST = 4106;
    private static volatile WeakReference<PushBridge> current = new WeakReference<>(null);

    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final PushRegistry registry;
    private Runnable afterPermission;
    private boolean enabling;
    private boolean destroyed;

    PushBridge(Activity activity, WebView web, ServerOrigin origin) {
        this.activity = activity;
        this.web = web;
        this.origin = origin;
        registry = new PushRegistry(activity);
        registry.selectInterface(origin.url());
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        current = new WeakReference<>(this);
        WebViewCompat.addWebMessageListener(web, "__wyndPush",
                Collections.singleton(origin.url().substring(0, origin.url().length() - 1)),
                (view, message, source, main, reply) -> {
                    if (!main || !trusted() || !origin.contains(source.toString())
                            || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 2048) return;
                    int id = -1;
                    try {
                        JSONObject request = new JSONObject(raw);
                        id = request.getInt("id");
                        if (id < 1) return;
                        String operation = request.getString("op");
                        if (operation.equals("enable")) {
                            enable(id, reply);
                            return;
                        }
                        JSONObject result = switch (operation) {
                            case "status" -> status("");
                            case "disable" -> {
                                registry.setEnabled(false);
                                yield status("");
                            }
                            case "ensure", "published", "remove" -> registration(operation, request);
                            default -> throw new IllegalArgumentException("Unknown push operation");
                        };
                        respond(id, reply, result);
                    } catch (JSONException | RuntimeException error) {
                        if (id > 0) respond(id, reply, null);
                    }
                });
    }

    private JSONObject registration(String op, JSONObject request) throws JSONException {
        String account = request.getString("account");
        if (!account.matches("[A-Za-z0-9_-]{1,128}")) throw new IllegalArgumentException("Invalid account");
        String accountOrigin = ServerOrigin.parse(request.getString("origin"), BuildConfig.DEBUG).url();
        String instance = registry.instance(accountOrigin, account);
        JSONObject result;
        if (op.equals("ensure")) {
            String key = request.getString("vapid");
            if (!key.matches("[A-Za-z0-9_-]{87}=?")) throw new IllegalArgumentException("Invalid VAPID key");
            result = registry.ensure(instance, accountOrigin, account, key);
        } else {
            result = registry.get(instance);
            if (op.equals("remove")) registry.remove(instance);
            else {
                String endpoint = request.getString("endpoint");
                if (!endpoint.startsWith("https://") || endpoint.length() > 1000) {
                    throw new IllegalArgumentException("Invalid push endpoint");
                }
                registry.published(instance, endpoint);
            }
        }
        JSONObject reply = new JSONObject();
        if (result != null) {
            reply.put("subscription", result.optJSONObject("subscription"));
            reply.put("published", result.optString("published", ""));
        }
        return reply;
    }

    private JSONObject status(String reason) throws JSONException {
        return new JSONObject().put("enabled", registry.enabled() && registry.hasDistributor())
                .put("allowed", registry.allowed()).put("reason", reason);
    }

    private void enable(int id, JavaScriptReplyProxy reply) {
        if (enabling || !activity.hasWindowFocus()) { respond(id, reply, null); return; }
        enabling = true;
        Runnable chooseDistributor = () -> {
            if (!trusted()) return;
            if (!registry.allowed()) {
                completeEnable(id, reply, "permission");
                return;
            }
            try {
                UnifiedPush.tryUseCurrentOrDefaultDistributor(activity, success -> {
                    if (trusted() && success) registry.setEnabled(true);
                    completeEnable(id, reply, success ? "" : "distributor");
                    return kotlin.Unit.INSTANCE;
                });
            } catch (RuntimeException error) { completeEnable(id, reply, "distributor"); }
        };
        if (!registry.allowed() && Build.VERSION.SDK_INT >= 33) {
            afterPermission = chooseDistributor;
            try { activity.requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, PERMISSION_REQUEST); }
            catch (RuntimeException error) {
                afterPermission = null;
                completeEnable(id, reply, "permission");
            }
        } else chooseDistributor.run();
    }

    void permissionResult() {
        Runnable resume = afterPermission;
        afterPermission = null;
        if (resume != null) resume.run();
    }

    private void completeEnable(int id, JavaScriptReplyProxy reply, String reason) {
        enabling = false;
        try { respond(id, reply, status(reason)); }
        catch (JSONException error) { respond(id, reply, null); }
    }

    private void respond(int id, JavaScriptReplyProxy reply, JSONObject result) {
        if (!trusted() || !WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        try {
            JSONObject response = new JSONObject().put("id", id);
            if (result == null) response.put("error", "push_unavailable");
            else response.put("result", result);
            reply.postMessage(response.toString());
        } catch (JSONException | RuntimeException ignored) { /* The document may have been replaced. */ }
    }

    private boolean trusted() {
        return !destroyed && !activity.isFinishing() && !activity.isDestroyed()
                && origin.contains(web.getUrl()) && origin.url().equals(registry.source());
    }

    void destroy() {
        destroyed = true;
        afterPermission = null;
        if (current.get() == this) current.clear();
    }

    static void changed() {
        PushBridge bridge = current.get();
        if (bridge == null) return;
        bridge.activity.runOnUiThread(() -> {
            if (bridge.trusted()) bridge.web.evaluateJavascript(
                    "window.dispatchEvent(new Event('wynd:android-push'))", null);
        });
    }
}
