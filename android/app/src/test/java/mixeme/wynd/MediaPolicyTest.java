package mixeme.wynd;

import org.junit.Test;
import static org.junit.Assert.*;

public final class MediaPolicyTest {
    @Test public void cameraNeverImplicitlyRequestsMicrophone() {
        assertArrayEquals(new String[] {MediaPolicy.CAMERA}, MediaPolicy.permissions(new String[] {MediaPolicy.VIDEO}));
        assertArrayEquals(new String[] {MediaPolicy.MICROPHONE}, MediaPolicy.permissions(new String[] {MediaPolicy.AUDIO}));
        assertArrayEquals(new String[] {MediaPolicy.CAMERA, MediaPolicy.MICROPHONE},
                MediaPolicy.permissions(new String[] {MediaPolicy.VIDEO, MediaPolicy.AUDIO, MediaPolicy.VIDEO}));
    }

    @Test public void rejectsUnknownOrFutureWebPermissions() {
        for (String unknown : new String[] {"android.webkit.resource.MIDI_SYSEX", "future-resource", "", "android.permission.CAMERA"}) {
            assertEquals(0, MediaPolicy.permissions(new String[] {MediaPolicy.VIDEO, unknown}).length);
        }
        assertEquals(0, MediaPolicy.permissions(new String[0]).length);
    }

    @Test public void acceptsOnlyContentUrisOutsideOwnProvider() {
        assertTrue(MediaPolicy.selectable("content://com.android.providers.media.documents/document/image%3A123", "mixeme.wynd.files"));
        for (String raw : new String[] {"file:///data/data/mixeme.wynd/shared_prefs/android-updates.xml",
                "content://mixeme.wynd.files/capture/private", "content://MIXEME.WYND.FILES/anything", "content:///path",
                "content://10@media/file", "content://media/file#fragment", "https://server/photo", "data:text/plain,secret", "invalid", null}) {
            assertFalse(String.valueOf(raw), MediaPolicy.selectable(raw, "mixeme.wynd.files"));
        }
    }

    @Test public void handlesRealAcceptListsAndFallsBackForExtensions() {
        assertArrayEquals(new String[] {"image/*", "video/*"}, MediaPolicy.mimeTypes(new String[] {"image/*, VIDEO/*", "image/*"}));
        assertArrayEquals(new String[] {"application/pdf"}, MediaPolicy.mimeTypes(new String[] {"application/pdf"}));
        for (String[] list : new String[][] {new String[0], {""}, {".wynd"}, {"*/*"}, {"image/*\r\nX-Header: value"}}) {
            assertArrayEquals(new String[] {"*/*"}, MediaPolicy.mimeTypes(list));
        }
    }

    @Test public void photoPickerKeepsVisualFiltersAndLeavesOtherFilesToDocuments() {
        assertEquals("image/*", MediaPolicy.visualPickerMime(new String[] {"image/*"}));
        assertEquals("video/mp4", MediaPolicy.visualPickerMime(new String[] {"video/mp4"}));
        assertEquals("*/*", MediaPolicy.visualPickerMime(new String[] {"video/*", "image/*"}));
        assertNull(MediaPolicy.visualPickerMime(new String[] {"*/*"}));
        assertNull(MediaPolicy.visualPickerMime(new String[] {"audio/*"}));
        assertNull(MediaPolicy.visualPickerMime(new String[] {"image/png", "application/pdf"}));
        assertNull(MediaPolicy.visualPickerMime(new String[] {"image/jpeg", "video/mp4"}));
    }
}
