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
    @Test public void firstRunCanRemainEmptyAcrossRestartWithoutBuildDefault() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog first = new InterfaceCatalog(disk, false, "");
        assertEquals("", first.selected());
        assertEquals(0, first.snapshot().getJSONArray("servers").length());
        InterfaceCatalog restored = new InterfaceCatalog(disk, false, "home.example.org");
        assertEquals("", restored.selected());
        restored.choose(server("https://home.example.org", "Дом"));
        assertEquals("https://home.example.org", restored.selected());
        assertEquals(1, restored.snapshot().getJSONArray("servers").length());
        assertFalse(disk.value.contains("invite"));
    }
    @Test public void choosingIsAtomicForFirstRunAndExistingSelection() throws Exception {
        for (String initial : new String[]{"", "home.example.org"}) {
            Memory disk = new Memory();
            InterfaceCatalog store = new InterfaceCatalog(disk, false, initial);
            String before = disk.value;
            disk.fail = true;
            assertThrows(IOException.class, () -> store.choose(server("https://files.example.org", "Файлы")));
            assertEquals(before, disk.value);
            assertEquals(before, store.snapshot().toString());
        }
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

    private static final String HOME_SERVER_FILE =
            "{\"selected\":\"https://home.example.org\",\"servers\":[{\"origin\":\"https://home.example.org\",\"name\":\"Дом\"}]}";

    @Test public void sourceDefaultsToEmbeddedForFilesWithoutTheField() throws Exception {
        Memory disk = new Memory();
        disk.value = HOME_SERVER_FILE;
        InterfaceCatalog restored = new InterfaceCatalog(disk, false, "other.example.org");
        // Решение владельца 28.11: прежние файлы читаются как вшитый интерфейс;
        // файл при чтении не переписывается.
        assertEquals("embedded", restored.source());
        assertFalse(disk.value.contains("source"));
    }

    @Test public void freshCatalogStoresEmbeddedSource() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog first = new InterfaceCatalog(disk, false, "");
        assertEquals("embedded", first.source());
        assertTrue(disk.value.contains("\"source\":\"embedded\""));
    }

    @Test public void foreignSourceValueAndServerWithoutSelectionReadAsEmbedded() throws Exception {
        Memory disk = new Memory();
        disk.value = "{\"selected\":\"https://home.example.org\",\"source\":\"local\",\"servers\":[{\"origin\":\"https://home.example.org\",\"name\":\"Дом\"}]}";
        assertEquals("embedded", new InterfaceCatalog(disk, false, "").source());
        Memory empty = new Memory();
        empty.value = "{\"selected\":\"\",\"source\":\"server\",\"servers\":[]}";
        assertEquals("embedded", new InterfaceCatalog(empty, false, "").source());
    }

    @Test public void serverChoiceOpensServerUiAndEmbeddedKeepsDataServer() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog store = new InterfaceCatalog(disk, false, "home.example.org");
        store.add(server("https://files.example.org", "Файлы"));
        assertEquals("embedded", store.source());
        store.select("files.example.org");
        assertEquals("server", store.source());
        assertEquals("https://files.example.org", store.selected());
        store.selectEmbedded();
        assertEquals("embedded", store.source());
        // Вшитый возвращает страницы из APK с тем же сервером данных.
        assertEquals("https://files.example.org", store.selected());
        InterfaceCatalog restored = new InterfaceCatalog(disk, false, "");
        assertEquals("embedded", restored.source());
        assertEquals("https://files.example.org", restored.selected());
    }

    @Test public void failedWriteDoesNotChangeSource() throws Exception {
        Memory disk = new Memory();
        InterfaceCatalog store = new InterfaceCatalog(disk, false, "home.example.org");
        String before = store.snapshot().toString();
        disk.fail = true;
        assertThrows(IOException.class, () -> store.selectEmbedded());
        assertEquals(before, store.snapshot().toString());
        assertEquals("embedded", store.source());
    }
}
