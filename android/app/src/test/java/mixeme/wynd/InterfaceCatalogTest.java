package mixeme.wynd;

import org.json.JSONObject;
import org.junit.Test;
import java.io.IOException;
import static org.junit.Assert.*;

public final class InterfaceCatalogTest {
    private static final class Memory implements InterfaceCatalog.Storage {
        String value;
        boolean fail;
        public String read() { return value; }
        public void write(String next) throws IOException {
            if (fail) throw new IOException("disk");
            value = next;
        }
    }
    private static JSONObject server(String origin, String name) throws Exception {
        return new JSONObject().put("origin", origin).put("name", name);
    }
    @Test public void addingAndRemovingNeverSelectOrStoreAccounts() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog store = new InterfaceCatalog(disk, false, "home.example.org");
        store.add(server("https://files.example.org", "Файлы"));
        assertEquals("https://home.example.org", store.selected());
        assertEquals(2, store.snapshot().getJSONArray("servers").length());
        store.remove("files.example.org");
        assertEquals("https://home.example.org", store.selected());
        assertThrows(IOException.class, () -> store.remove(store.selected()));
        assertEquals(1, store.snapshot().getJSONArray("servers").length());
        assertFalse(disk.value.contains("token"));
    }
    @Test public void persistedSelectionOverridesBuildDefaultAfterRestart() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog store = new InterfaceCatalog(disk, false, "home.example.org");
        store.add(server("https://files.example.org", "Файлы"));
        store.select("files.example.org");
        InterfaceCatalog restored = new InterfaceCatalog(disk, false, "changed.example.org");
        assertEquals("https://files.example.org", restored.selected());
        assertEquals(2, restored.snapshot().getJSONArray("servers").length());
        assertThrows(IOException.class, () -> restored.select("unknown.example.org"));
    }
    @Test public void failedWriteDoesNotChangeSelectedOrList() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog store = new InterfaceCatalog(disk, false, "home.example.org");
        store.add(server("https://files.example.org", "Файлы"));
        String before = store.snapshot().toString();
        disk.fail = true;
        assertThrows(IOException.class, () -> store.select("files.example.org"));
        assertThrows(IOException.class, () -> store.remove("files.example.org"));
        assertThrows(IOException.class, () -> store.add(server("https://another.example.org", "Другой")));
        assertEquals(before, store.snapshot().toString());
    }
    @Test public void duplicatesUpdateNameAndSnapshotsCannotMutateState() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog store = new InterfaceCatalog(disk, false, "home.example.org");
        store.add(server("https://HOME.example.org:443", "Дом"));
        assertEquals(1, store.snapshot().getJSONArray("servers").length());
        JSONObject snapshot = store.snapshot();
        snapshot.put("selected", "https://evil.example.org");
        assertEquals("https://home.example.org", store.selected());
        assertEquals("Дом", store.snapshot().getJSONArray("servers").getJSONObject(0).getString("name"));
    }
    @Test public void corruptCatalogIsNotSilentlyReplacedOrWritten() throws Exception {
        Memory disk = new Memory();
        disk.value = "{\"selected\":\"https://missing.example.org\",\"servers\":[]}";
        String before = disk.value;
        assertThrows(IOException.class, () -> new InterfaceCatalog(disk, false, "home.example.org"));
        assertEquals(before, disk.value);
    }
    @Test public void rejectsForeignSiteResponsesAndKeepsRegistrationIndependent() throws Exception {
        ServerOrigin origin = ServerOrigin.parse("home.example.org", false);
        for (String body : new String[]{"<html>Login</html>", "{}", "{\"name\":\"Other site\"}"}) {
            assertThrows(Exception.class, () -> InterfaceServer.parse(origin, body));
        }
        JSONObject result = InterfaceServer.parse(origin,
                "{\"name\":\"Дом\",\"version\":\"0.53.0\",\"registration_mode\":\"closed\"}");
        assertEquals("Дом", result.getString("name"));
        assertEquals("https://home.example.org", result.getString("origin"));
    }
}
