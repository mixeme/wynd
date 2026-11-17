package mixeme.wynd;

import org.junit.Test;
import static org.junit.Assert.*;

public final class StartupTargetTest {
    @Test public void acceptsServerAddressWithDesktopNormalization() {
        StartupTarget target = StartupTarget.parse(" HOME.example.org:443/ ", false);
        assertEquals("https://home.example.org/", target.origin().url());
        assertEquals("/", target.path());
        assertEquals(target.origin().url(), target.url());
    }
    @Test public void separatesInvitationTokenFromTheOriginUsedForChecking() {
        StartupTarget target = StartupTarget.parse("https://home.example.org/invite/OwnToken_123?members=1", false);
        assertEquals("https://home.example.org/", target.origin().url());
        assertEquals("/invite/OwnToken_123?members=1", target.path());
        assertFalse(target.origin().url().contains("OwnToken"));
        assertEquals("https://home.example.org/invite/OwnToken_123?members=1", target.url());
    }
    @Test public void keepsForeignServerInvitationOnItsOwnOrigin() {
        StartupTarget target = StartupTarget.parse("https://other.example.org:8443/join/aBc_123-/", false);
        assertEquals("https://other.example.org:8443/", target.origin().url());
        assertEquals("/join/aBc_123-", target.path());
    }
    @Test public void rejectsCredentialsDeviceUrlsPathsAndTraversal() {
        for (String raw : new String[] { "", "/invite/token", "https://user:secret@home.example.org/invite/a",
                "file:///data/secret", "content://photos/1", "javascript:alert(1)",
                "https://home.example.org/settings", "https://home.example.org?token=secret",
                "https://home.example.org/invite/", "https://home.example.org/invite/a/next",
                "https://home.example.org/invite/../settings", "https://home.example.org/invite/%2e%2e",
                "https://home.example.org/invite/a%2fsettings", "https://home.example.org/invite/a#token",
                "https://home.example.org\\@other.example.org/invite/a" }) {
            assertThrows(raw, IllegalArgumentException.class, () -> StartupTarget.parse(raw, true));
        }
    }
    @Test public void permitsHttpOnlyForDebugLoopbackAndBoundsInput() {
        assertEquals("http://127.0.0.1:7696/invite/a", StartupTarget.parse("http://127.0.0.1:7696/invite/a", true).url());
        assertThrows(IllegalArgumentException.class, () -> StartupTarget.parse("http://127.0.0.1/invite/a", false));
        assertThrows(IllegalArgumentException.class, () -> StartupTarget.parse("http://home.example.org/invite/a", true));
        assertThrows(IllegalArgumentException.class, () -> StartupTarget.parse("https://home.example.org/invite/" + "a".repeat(513), false));
        assertThrows(IllegalArgumentException.class, () -> StartupTarget.parse("a".repeat(4097), true));
    }
}
