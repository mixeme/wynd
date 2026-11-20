package mixeme.wynd;

import android.app.NotificationManager;
import android.content.Context;
import android.content.SharedPreferences;
import android.os.Build;
import android.os.SystemClock;

import org.json.JSONException;
import org.json.JSONObject;
import org.unifiedpush.android.connector.UnifiedPush;
import org.unifiedpush.android.connector.data.PushEndpoint;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/** Registration state only. Sessions and requests to account servers belong to the web client. */
final class PushRegistry {
    private static final String SOURCE = "interface";
    private static final String ENABLED = "enabled";
    private static final String DISTRIBUTOR = "distributor";
    private static final String REGISTRATION = "registration:";
    private static final Map<String, Long> requestedAt = new ConcurrentHashMap<>();
    private final Context context;
    private final SharedPreferences preferences;

    PushRegistry(Context context) {
        this.context = context.getApplicationContext();
        preferences = context.getSharedPreferences("push", Context.MODE_PRIVATE);
    }

    void selectInterface(String origin) {
        if (source().equals(origin)) return;
        removeAll();
        preferences.edit().putString(SOURCE, origin).putBoolean(ENABLED, false).remove(DISTRIBUTOR).apply();
    }

    String source() { return preferences.getString(SOURCE, ""); }
    boolean enabled() { return preferences.getBoolean(ENABLED, false); }
    void setEnabled(boolean enabled) {
        SharedPreferences.Editor edit = preferences.edit().putBoolean(ENABLED, enabled);
        if (enabled) edit.putString(DISTRIBUTOR, UnifiedPush.getSavedDistributor(context));
        else edit.remove(DISTRIBUTOR);
        edit.apply();
        if (!enabled) removeAll();
    }

    boolean hasDistributor() {
        String distributor = preferences.getString(DISTRIBUTOR, null);
        return distributor != null && UnifiedPush.getDistributors(context).contains(distributor);
    }

    boolean allowed() {
        NotificationManager manager = (NotificationManager) context.getSystemService(Context.NOTIFICATION_SERVICE);
        return manager != null && (Build.VERSION.SDK_INT < 24 || manager.areNotificationsEnabled());
    }

    private void removeAll() {
        for (String key : preferences.getAll().keySet()) {
            if (key.startsWith(REGISTRATION)) remove(key.substring(REGISTRATION.length()));
        }
    }

    String instance(String origin, String account) {
        try {
            byte[] bytes = (source() + "\n" + origin + "\n" + account).getBytes(StandardCharsets.UTF_8);
            byte[] digest = MessageDigest.getInstance("SHA-256").digest(bytes);
            return android.util.Base64.encodeToString(digest,
                    android.util.Base64.URL_SAFE | android.util.Base64.NO_WRAP | android.util.Base64.NO_PADDING);
        } catch (NoSuchAlgorithmException impossible) {
            throw new IllegalStateException(impossible);
        }
    }

    JSONObject get(String instance) {
        String raw = preferences.getString(REGISTRATION + instance, null);
        if (raw == null) return null;
        try { return new JSONObject(raw); }
        catch (JSONException malformed) { return null; }
    }

    JSONObject ensure(String instance, String vapid) throws JSONException {
        JSONObject registration = get(instance);
        if (!enabled() || !allowed() || !hasDistributor()) return registration;
        if (UnifiedPush.getSavedDistributor(context) == null) {
            // The connector forgets the last distributor when the last account signs out.
            UnifiedPush.saveDistributor(context, preferences.getString(DISTRIBUTOR, ""));
        }
        if (registration == null) registration = new JSONObject();
        boolean apply = !vapid.equals(registration.optString("vapid"));
        if (apply) {
            registration.put("vapid", vapid).remove("subscription");
            save(instance, registration);
            requestedAt.remove(instance);
        }
        long now = SystemClock.elapsedRealtime();
        Long lastRequest = requestedAt.get(instance);
        if (!registration.has("subscription") && (lastRequest == null || now - lastRequest >= 30_000)) {
            requestedAt.put(instance, now);
            try { UnifiedPush.register(context, instance, "Wynd", vapid); }
            catch (RuntimeException error) { requestedAt.remove(instance); throw error; }
        }
        return registration;
    }

    boolean update(String instance, PushEndpoint endpoint) throws JSONException {
        requestedAt.remove(instance);
        JSONObject registration = get(instance);
        if (registration == null || !enabled()) return false;
        org.unifiedpush.android.connector.data.PublicKeySet keys = endpoint.getPubKeySet();
        if (keys == null || !endpoint.getUrl().startsWith("https://") || endpoint.getUrl().length() > 1000) return false;
        JSONObject sub = new JSONObject().put("endpoint", endpoint.getUrl())
                .put("p256dh", keys.getPubKey()).put("auth", keys.getAuth());
        if (sub.toString().equals(registration.optString("subscription"))) return false;
        registration.put("subscription", sub);
        save(instance, registration);
        return true;
    }

    void failed(String instance, boolean unregistered) {
        JSONObject registration = get(instance);
        if (unregistered && registration != null) {
            registration.remove("subscription");
            save(instance, registration);
        }
    }

    void published(String instance, String endpoint) throws JSONException {
        JSONObject registration = get(instance);
        if (registration != null) save(instance, registration.put("published", endpoint));
    }

    void remove(String instance) {
        boolean existed = preferences.contains(REGISTRATION + instance);
        preferences.edit().remove(REGISTRATION + instance).apply();
        requestedAt.remove(instance);
        WyndPushService.cancel(context, instance);
        if (existed) {
            try { UnifiedPush.unregister(context, instance); }
            catch (RuntimeException ignored) { /* The distributor may have been removed. */ }
        }
    }

    private void save(String instance, JSONObject entry) {
        preferences.edit().putString(REGISTRATION + instance, entry.toString()).apply();
    }
}
