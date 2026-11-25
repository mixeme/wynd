package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;
import org.junit.Test;

import java.io.IOException;

import static org.junit.Assert.*;

/** C127: общее хранилище входов оболочки Android. */
public final class SecretsStoreTest {
    private static final class Memory implements SecretsStore.Storage {
        String value;
        boolean fail;

        public String read() { return value; }

        public void write(String next) throws IOException {
            if (fail) throw new IOException("disk");
            value = next;
        }
    }

    private static JSONObject session(String origin, String account) throws Exception {
        return new JSONObject()
                .put("origin", origin)
                .put("name", "Круг")
                .put("email", account + "@example.org")
                .put("token", "token-" + account)
                .put("account_id", account)
                .put("signed_in_at", "2026-11-25T10:00:00.000Z");
    }

    private static SecretsStore store(Memory memory) throws Exception {
        return new SecretsStore(memory, false);
    }

    @Test public void storesSessionsOfEveryServerAcrossRestarts() throws Exception {
        Memory memory = new Memory();
        SecretsStore first = store(memory);
        first.put(session("https://a.example.org", "anya"));
        first.put(session("https://HOME.example.org:443", "bob"));
        JSONArray stored = first.list();
        assertEquals(2, stored.length());
        boolean canonical = false;
        for (int i = 0; i < stored.length(); i++) {
            if (stored.getJSONObject(i).getString("origin").equals("https://home.example.org")) canonical = true;
        }
        assertTrue("origin приводится к каноническому виду", canonical);

        SecretsStore second = store(memory);
        JSONArray sessions = second.list();
        assertEquals("записи переживают перезапуск", 2, sessions.length());
        second.remove("https://a.example.org");
        assertEquals(1, second.list().length());
        assertFalse(memory.value.contains("a.example.org"));
    }

    @Test public void rejectsSessionsItCannotTrust() throws Exception {
        Memory memory = new Memory();
        SecretsStore store = store(memory);
        assertThrows(Exception.class, () -> store.put(session("", "anya")));
        assertThrows(Exception.class, () -> store.put(session("http://a.example.org", "anya")));
        JSONObject noToken = session("https://a.example.org", "anya");
        noToken.remove("token");
        assertThrows(Exception.class, () -> store.put(noToken));
        JSONObject badAccount = session("https://a.example.org", "anya").put("account_id", "a/../../b");
        assertThrows(Exception.class, () -> store.put(badAccount));
        JSONObject longName = session("https://a.example.org", "anya").put("name", "x".repeat(513));
        assertThrows(Exception.class, () -> store.put(longName));
        assertEquals("отказы ничего не записали", 0, store.list().length());
        assertNull(memory.value);
    }

    @Test public void capsTheNumberOfSessions() throws Exception {
        Memory memory = new Memory();
        SecretsStore store = store(memory);
        for (int i = 0; i < 32; i++) store.put(session("https://s" + i + ".example.org", "account" + i));
        assertThrows(Exception.class,
                () -> store.put(session("https://overflow.example.org", "overflow")));
        assertEquals(32, store.list().length());
    }

    @Test public void failedWriteKeepsStateAndDiskInStep() throws Exception {
        Memory memory = new Memory();
        SecretsStore store = store(memory);
        memory.fail = true;
        assertThrows(IOException.class, () -> store.put(session("https://a.example.org", "anya")));
        assertEquals("сбой диска не оставляет запись в памяти", 0, store.list().length());
        memory.fail = false;
        store.put(session("https://a.example.org", "anya"));
        assertEquals(1, store.list().length());
    }

    @Test public void corruptFileIsAnErrorNotSilentLoss() {
        Memory memory = new Memory();
        memory.value = "{not json";
        assertThrows(JSONException.class, () -> store(memory));
    }
}
