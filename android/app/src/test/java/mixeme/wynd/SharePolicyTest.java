package mixeme.wynd;

import org.junit.Test;
import static org.junit.Assert.*;

public final class SharePolicyTest {
    @Test public void cancelsOnlyTheSelectedOriginsSharePage() {
        ServerOrigin origin = ServerOrigin.parse("https://home.example", false);
        assertTrue(SharePolicy.choicePage("https://home.example/share", origin));
        assertFalse(SharePolicy.choicePage("https://foreign.example/share", origin));
        assertFalse(SharePolicy.choicePage("https://home.example/circles/c1/compose", origin));
        assertFalse(SharePolicy.choicePage(null, origin));
    }
    @Test public void rejectsLocalPathsAndCrossProfileAuthorities() {
        assertTrue(SharePolicy.uri("content://media/external/images/media/12"));
        assertFalse(SharePolicy.uri("file:///data/data/mixeme.wynd/files/private"));
        assertFalse(SharePolicy.uri("https://example.org/photo.jpg"));
        assertFalse(SharePolicy.uri("content://10@media/external/12"));
        assertFalse(SharePolicy.uri(null));
    }
    @Test public void limitsActualMediaAndUtf8Text() {
        assertTrue(SharePolicy.media("image/jpeg")); assertTrue(SharePolicy.media("video/mp4"));
        assertFalse(SharePolicy.media("text/html")); assertFalse(SharePolicy.media("image/*"));
        assertTrue(SharePolicy.text("я".repeat(16384)));
        assertFalse(SharePolicy.text("я".repeat(16385)));
    }
}
