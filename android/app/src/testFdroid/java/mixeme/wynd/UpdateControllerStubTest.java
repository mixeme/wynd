package mixeme.wynd;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.json.JSONObject;
import org.junit.Test;

/** Охрана контракта заглушки: страница fdroid-редакции обязана видеть
 *  отсутствие обновлений и не может ничего скачать или установить. */
public class UpdateControllerStubTest {
    private static final String[] KEYS = {"version", "debug", "development", "protocol",
            "capabilities", "immediately", "state", "available", "nextVersion", "ready",
            "error", "blocked", "installing", "permission"};

    @Test
    public void statusReportsUpdatesUnavailable() throws Exception {
        JSONObject status = UpdateController.get(null).status();
        assertEquals(1, status.getInt("protocol"));
        assertFalse(status.getJSONObject("capabilities").getBoolean("updates"));
        assertFalse(status.getJSONObject("capabilities").getBoolean("developmentUpdateBypass"));
        assertFalse(status.getBoolean("available"));
        assertFalse(status.getBoolean("ready"));
        assertFalse(status.getBoolean("installing"));
        assertEquals("", status.getString("nextVersion"));
        assertEquals("idle", status.getString("state"));
    }

    @Test
    public void statusKeepsWebContractKeys() throws Exception {
        JSONObject status = UpdateController.get(null).status();
        assertEquals(KEYS.length, status.length());
        for (String key : KEYS) assertTrue("Missing key " + key, status.has(key));
    }

    @Test
    public void mutatingMethodsAreDeniedWithoutStateChange() throws Exception {
        UpdateController updates = UpdateController.get(null);
        try {
            updates.check();
            throw new AssertionError("check must be denied");
        } catch (IllegalStateException expected) { }
        try {
            updates.prepare();
            throw new AssertionError("prepare must be denied");
        } catch (IllegalStateException expected) { }
        try {
            updates.setImmediately(true);
            throw new AssertionError("setImmediately must be denied");
        } catch (IllegalStateException expected) { }
        updates.install(null);
        updates.installationResult(5, 1, null);
        updates.failure(new IllegalStateException("fake"));
        JSONObject status = updates.status();
        assertEquals("idle", status.getString("state"));
        assertEquals("", status.getString("error"));
    }
}
