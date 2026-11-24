package mixeme.wynd;

import org.junit.Test;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

public class ArchiveStoreTest {
    @Test
    public void recognizesWyndArchivesByExtensionOrMime() {
        assertTrue(ArchiveStore.isWynd("family.wynd", "application/octet-stream"));
        assertTrue(ArchiveStore.isWynd("FAMILY.WYND", null));
        assertTrue(ArchiveStore.isWynd("archive.bin", "application/x-wynd"));
        assertFalse(ArchiveStore.isWynd("family.zip", "application/zip"));
        assertFalse(ArchiveStore.isWynd("photo.jpg", "image/jpeg"));
        assertFalse(ArchiveStore.isWynd(null, null));
    }
}
