package mixeme.wynd;

import android.app.NotificationManager;
import android.content.Context;
import android.content.SharedPreferences;
import android.os.Build;

import org.json.JSONException;
import org.json.JSONObject;
import org.unifiedpush.android.connector.UnifiedPush;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Registration state only. Sessions and requests to account servers belong to the web client.
 *
 * Подписка принадлежит паре «сервер + учётная запись», а не выбранному источнику
 * интерфейса: смена источника не гасит уведомления остальных входов (C126).
 */
final class PushRegistry {
    private static final String SOURCE = "interface";
    private static final String ENABLED = "enabled";
    private static final String DISTRIBUTOR = "distributor";
    private static final String REGISTRATION = "registration:";
    /** Версия ключей: 2 — ключ из сервера и учётной записи, без источника интерфейса. */
    static final String VERSION = "registrations-version";
    static final int VERSION_CURRENT = 2;
    private static final Map<String, Long> requestedAt = new ConcurrentHashMap<>();

    /** Настройки устройства: без Android, чтобы поведение проверялось обычным тестом. */
    interface Store {
        String string(String key, String fallback);
        boolean bool(String key, boolean fallback);
        int integer(String key, int fallback);
        boolean contains(String key);
        void putString(String key, String value);
        void putBoolean(String key, boolean value);
        void putInt(String key, int value);
        void remove(String key);
        /** Снимок ключей: по нему идёт удаление. */
        Set<String> keys();
    }

    /** Доставщик и системный показ: единственные обращения к UnifiedPush и Android. */
    interface Connector {
        boolean allowed();
        boolean installed(String distributor);
        String savedDistributor();
        void saveDistributor(String distributor);
        void register(String instance, String vapid);
        void unregister(String instance);
        void cancel(String instance);
    }

    private final Store store;
    private final Connector connector;

    PushRegistry(Context context) {
        this(new AndroidStore(context), new AndroidConnector(context));
    }

    PushRegistry(Store store, Connector connector) {
        this.store = store;
        this.connector = connector;
        migrate();
    }

    /**
     * Ключ регистрации раньше включал источник интерфейса, поэтому смена источника
     * теряла подписки. Старые строки снимаются один раз и создаются заново с ключом
     * «сервер + учётная запись»; устройство остаётся включённым.
     */
    private void migrate() {
        if (store.integer(VERSION, 0) >= VERSION_CURRENT) return;
        for (String key : store.keys()) {
            if (!key.startsWith(REGISTRATION)) continue;
            String instance = key.substring(REGISTRATION.length());
            store.remove(key);
            requestedAt.remove(instance);
            connector.cancel(instance);
            try {
                connector.unregister(instance);
            } catch (RuntimeException ignored) {
                /* The distributor may have been removed. */
            }
        }
        store.putInt(VERSION, VERSION_CURRENT);
        String distributor = store.string(DISTRIBUTOR, null);
        // Снятие последней регистрации заставляет коннектор забыть доставщика;
        // уведомления на телефоне остаются включёнными.
        if (distributor != null && enabled()) connector.saveDistributor(distributor);
    }

    /** Запоминает источник интерфейса. Регистрации и тогл принадлежат устройству. */
    void selectInterface(String origin) {
        if (source().equals(origin)) return;
        store.putString(SOURCE, origin);
    }

    String source() { return store.string(SOURCE, ""); }
    boolean enabled() { return store.bool(ENABLED, false); }

    void setEnabled(boolean enabled) {
        store.putBoolean(ENABLED, enabled);
        if (enabled) store.putString(DISTRIBUTOR, connector.savedDistributor());
        else {
            store.remove(DISTRIBUTOR);
            removeAll();
        }
    }

    boolean hasDistributor() {
        String distributor = store.string(DISTRIBUTOR, null);
        return distributor != null && connector.installed(distributor);
    }

    boolean allowed() {
        return connector.allowed();
    }

    private void removeAll() {
        for (String key : store.keys()) {
            if (key.startsWith(REGISTRATION)) remove(key.substring(REGISTRATION.length()));
        }
    }

    /** Ключ подписки — только сервер и учётная запись: источник интерфейса не участвует. */
    String instance(String origin, String account) {
        try {
            byte[] bytes = (origin + "\n" + account).getBytes(StandardCharsets.UTF_8);
            byte[] digest = MessageDigest.getInstance("SHA-256").digest(bytes);
            StringBuilder hex = new StringBuilder(digest.length * 2);
            for (byte value : digest) {
                hex.append(Character.forDigit((value >> 4) & 0xf, 16));
                hex.append(Character.forDigit(value & 0xf, 16));
            }
            return hex.toString();
        } catch (NoSuchAlgorithmException impossible) {
            throw new IllegalStateException(impossible);
        }
    }

    JSONObject get(String instance) {
        String raw = store.string(REGISTRATION + instance, null);
        if (raw == null) return null;
        try { return new JSONObject(raw); }
        catch (JSONException malformed) { return null; }
    }

    JSONObject ensure(String instance, String origin, String account, String vapid) throws JSONException {
        JSONObject registration = get(instance);
        if (!enabled() || !allowed() || !hasDistributor()) return registration;
        String distributor = store.string(DISTRIBUTOR, "");
        // The connector forgets the last distributor when the last account signs out.
        if (!distributor.equals(connector.savedDistributor())) {
            connector.saveDistributor(distributor);
        }
        if (registration == null) registration = new JSONObject();
        boolean apply = !vapid.equals(registration.optString("vapid"));
        if (apply || !origin.equals(registration.optString("origin"))
                || !account.equals(registration.optString("account"))) {
            registration.put("origin", origin).put("account", account);
            if (apply) {
                registration.put("vapid", vapid).remove("subscription");
                requestedAt.remove(instance);
            }
            save(instance, registration);
        }
        long now = System.nanoTime() / 1_000_000;
        Long lastRequest = requestedAt.get(instance);
        if (!registration.has("subscription") && (lastRequest == null || now - lastRequest >= 30_000)) {
            requestedAt.put(instance, now);
            try { connector.register(instance, vapid); }
            catch (RuntimeException error) { requestedAt.remove(instance); throw error; }
        }
        return registration;
    }

    boolean update(String instance, String url, String p256dh, String auth) throws JSONException {
        requestedAt.remove(instance);
        JSONObject registration = get(instance);
        if (registration == null || !enabled()) return false;
        if (url == null || p256dh == null || auth == null
                || !url.startsWith("https://") || url.length() > 1000) return false;
        JSONObject sub = new JSONObject().put("endpoint", url).put("p256dh", p256dh).put("auth", auth);
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
        boolean existed = store.contains(REGISTRATION + instance);
        store.remove(REGISTRATION + instance);
        requestedAt.remove(instance);
        connector.cancel(instance);
        if (existed) {
            try { connector.unregister(instance); }
            catch (RuntimeException ignored) { /* The distributor may have been removed. */ }
        }
    }

    private void save(String instance, JSONObject entry) {
        store.putString(REGISTRATION + instance, entry.toString());
    }

    private static final class AndroidStore implements Store {
        private final SharedPreferences preferences;

        AndroidStore(Context context) {
            preferences = context.getApplicationContext().getSharedPreferences("push", Context.MODE_PRIVATE);
        }

        public String string(String key, String fallback) { return preferences.getString(key, fallback); }
        public boolean bool(String key, boolean fallback) { return preferences.getBoolean(key, fallback); }
        public int integer(String key, int fallback) { return preferences.getInt(key, fallback); }
        public boolean contains(String key) { return preferences.contains(key); }
        public void putString(String key, String value) { preferences.edit().putString(key, value).apply(); }
        public void putBoolean(String key, boolean value) { preferences.edit().putBoolean(key, value).apply(); }
        public void putInt(String key, int value) { preferences.edit().putInt(key, value).apply(); }
        public void remove(String key) { preferences.edit().remove(key).apply(); }
        public Set<String> keys() { return new HashSet<>(preferences.getAll().keySet()); }
    }

    private static final class AndroidConnector implements Connector {
        private final Context context;

        AndroidConnector(Context context) { this.context = context.getApplicationContext(); }

        public boolean allowed() {
            NotificationManager manager =
                    (NotificationManager) context.getSystemService(Context.NOTIFICATION_SERVICE);
            return manager != null && (Build.VERSION.SDK_INT < 24 || manager.areNotificationsEnabled());
        }

        public boolean installed(String distributor) {
            return UnifiedPush.getDistributors(context).contains(distributor);
        }

        public String savedDistributor() { return UnifiedPush.getSavedDistributor(context); }

        public void saveDistributor(String distributor) {
            UnifiedPush.saveDistributor(context, distributor);
        }

        public void register(String instance, String vapid) {
            UnifiedPush.register(context, instance, "Wynd", vapid);
        }

        public void unregister(String instance) { UnifiedPush.unregister(context, instance); }

        public void cancel(String instance) { WyndPushService.cancel(context, instance); }
    }
}
