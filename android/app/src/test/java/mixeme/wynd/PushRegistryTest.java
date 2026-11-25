package mixeme.wynd;

import org.json.JSONObject;
import org.junit.Test;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.junit.Assert.*;

/**
 * C126: подписка принадлежит паре «сервер + учётная запись». Смена источника
 * интерфейса и выход с одного сервера не трогают уведомления остальных.
 */
public final class PushRegistryTest {
    private static final String VAPID = "A".repeat(87);

    private static final class Memory implements PushRegistry.Store {
        final Map<String, Object> values = new HashMap<>();

        public String string(String key, String fallback) {
            Object value = values.get(key);
            return value instanceof String ? (String) value : fallback;
        }

        public boolean bool(String key, boolean fallback) {
            Object value = values.get(key);
            return value instanceof Boolean ? (Boolean) value : fallback;
        }

        public int integer(String key, int fallback) {
            Object value = values.get(key);
            return value instanceof Integer ? (Integer) value : fallback;
        }

        public boolean contains(String key) { return values.containsKey(key); }
        public void putString(String key, String value) {
            if (value == null) values.remove(key);
            else values.put(key, value);
        }
        public void putBoolean(String key, boolean value) { values.put(key, value); }
        public void putInt(String key, int value) { values.put(key, value); }
        public void remove(String key) { values.remove(key); }
        public Set<String> keys() { return new HashSet<>(values.keySet()); }
    }

    private static final class FakeConnector implements PushRegistry.Connector {
        boolean allowed = true;
        final Set<String> installed = new HashSet<>(Collections.singleton("ntfy"));
        String saved = "ntfy";
        final List<String> registered = new ArrayList<>();
        final List<String> unregistered = new ArrayList<>();
        final List<String> cancelled = new ArrayList<>();

        public boolean allowed() { return allowed; }
        public boolean installed(String distributor) { return installed.contains(distributor); }
        public String savedDistributor() { return saved; }
        public void saveDistributor(String distributor) { saved = distributor; }
        public void register(String instance, String vapid) { registered.add(instance); }
        public void unregister(String instance) { unregistered.add(instance); }
        public void cancel(String instance) { cancelled.add(instance); }
    }

    private static void subscribe(PushRegistry registry, String instance, String origin,
            String account, String endpoint) throws Exception {
        JSONObject registration = registry.ensure(instance, origin, account, VAPID);
        assertNotNull("после ensure должна быть строка регистрации", registration);
        assertTrue("новый адрес доставки принимается", registry.update(instance, endpoint, "p256dh", "auth"));
    }

    @Test public void changingInterfaceKeepsNotificationsAndOtherAccounts() throws Exception {
        Memory store = new Memory();
        FakeConnector connector = new FakeConnector();
        PushRegistry registry = new PushRegistry(store, connector);
        registry.selectInterface("https://a.example/");
        registry.setEnabled(true);

        String a = registry.instance("https://a.example/", "account-a");
        String b = registry.instance("https://b.example/", "account-b");
        assertNotEquals("ключи разных серверов различаются", a, b);
        subscribe(registry, a, "https://a.example/", "account-a", "https://push.example/a");
        subscribe(registry, b, "https://b.example/", "account-b", "https://push.example/b");
        assertEquals("подписки создаются на обоих серверах", 2, connector.registered.size());

        registry.selectInterface("https://b.example/");

        assertTrue("смена источника не выключает уведомления", registry.enabled());
        assertTrue(registry.hasDistributor());
        assertNotNull("подписка A сохраняется", registry.get(a));
        assertNotNull("подписка Б сохраняется", registry.get(b));
        assertTrue("при смене источника ничего не снимается", connector.unregistered.isEmpty());
        assertTrue(connector.cancelled.isEmpty());
        assertEquals("https://b.example/", registry.source());

        // Коннектор мог забыть доставщика после выхода последнего входа:
        // следующая синхронизация возвращает его, тогл не гаснет.
        connector.saved = null;
        assertNotNull(registry.ensure(b, "https://b.example/", "account-b", VAPID));
        assertEquals("ntfy", connector.savedDistributor());

        registry.setEnabled(false);
        registry.selectInterface("https://a.example/");
        assertFalse("выключенное остаётся выключенным", registry.enabled());
    }

    @Test public void logoutRemovesOnlyItsRegistration() throws Exception {
        Memory store = new Memory();
        FakeConnector connector = new FakeConnector();
        PushRegistry registry = new PushRegistry(store, connector);
        registry.selectInterface("https://a2.example/");
        registry.setEnabled(true);

        String a = registry.instance("https://a2.example/", "account-a");
        String b = registry.instance("https://b2.example/", "account-b");
        subscribe(registry, a, "https://a2.example/", "account-a", "https://push.example/a2");
        subscribe(registry, b, "https://b2.example/", "account-b", "https://push.example/b2");

        registry.remove(a);

        assertNull("выход с A снимает только A", registry.get(a));
        assertNotNull("подписка Б остаётся", registry.get(b));
        assertTrue("уведомления на телефоне остаются включёнными", registry.enabled());
        assertTrue(registry.hasDistributor());
        assertEquals(Collections.singletonList(a), connector.unregistered);
        assertEquals(Collections.singletonList(a), connector.cancelled);
    }

    @Test public void legacyRegistrationsAreReplacedOnceAndDeviceStaysOn() {
        Memory store = new Memory();
        FakeConnector connector = new FakeConnector();
        store.putString("interface", "https://a3.example/");
        store.putBoolean("enabled", true);
        store.putString("distributor", "ntfy");
        store.putString("registration:legacy-one", "{\"vapid\":\"x\",\"published\":\"https://push.example/old\"}");
        store.putString("registration:legacy-two", "{}");
        connector.saved = null;

        PushRegistry registry = new PushRegistry(store, connector);

        assertFalse(store.contains("registration:legacy-one"));
        assertFalse(store.contains("registration:legacy-two"));
        assertEquals(new HashSet<>(Arrays.asList("legacy-one", "legacy-two")),
                new HashSet<>(connector.unregistered));
        assertEquals("доставщик возвращается коннектору", "ntfy", connector.savedDistributor());
        assertTrue("устройство остаётся включённым", registry.enabled());
        assertTrue(registry.hasDistributor());
        assertEquals(PushRegistry.VERSION_CURRENT, store.integer(PushRegistry.VERSION, 0));

        connector.unregistered.clear();
        new PushRegistry(store, connector);
        assertTrue("миграция выполняется один раз", connector.unregistered.isEmpty());
    }
}
