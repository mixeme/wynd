package mixeme.wynd;

import org.junit.Test;
import static org.junit.Assert.*;

public final class ExportPolicyTest {
    @Test public void namesCannotBecomePathsOrControlCharacters() {
        assertEquals("a_b_c_.txt", ExportPolicy.filename("a/b\\c\n.txt"));
        assertEquals("wynd-file", ExportPolicy.filename(".."));
        assertEquals("Фото.jpg", ExportPolicy.filename("Фото.jpg"));
        assertEquals("application/octet-stream", ExportPolicy.mime("text/*"));
        assertEquals("text/plain", ExportPolicy.mime("text/plain;charset=utf-8"));
    }

    @Test public void onlyHttpsOrSelectedUsbDevelopmentOriginCanBeDownloaded() {
        ServerOrigin selected = ServerOrigin.parse("http://127.0.0.1:7696", true);
        assertTrue(ExportPolicy.remote("https://circle.example/archive?token=x", selected, false));
        assertTrue(ExportPolicy.remote("http://127.0.0.1:7696/archive", selected, true));
        for (String raw : new String[] {"file:///private", "content://provider/file", "https://u:p@example/a", "https://example/a#b", "http://127.0.0.1:7697/a", "http://circle.example/a"}) {
            assertFalse(raw, ExportPolicy.remote(raw, selected, true));
        }
        assertFalse(ExportPolicy.remote("http://127.0.0.1:7696/a", selected, false));
    }
}
