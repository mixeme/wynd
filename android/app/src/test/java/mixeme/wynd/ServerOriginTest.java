package mixeme.wynd;

import org.junit.Test;

import static org.junit.Assert.*;

public final class ServerOriginTest {
    @Test public void normalizesLikeDesktopServerAddress() {
        assertEquals("https://home.example.org/", ServerOrigin.parse(" HOME.example.org/ ", false).url());
        assertEquals("https://home.example.org/", ServerOrigin.parse("https://home.example.org:443", false).url());
        assertEquals("https://home.example.org:8443/", ServerOrigin.parse("https://home.example.org:8443", false).url());
    }

    @Test public void allowsOnlySameSchemeHostAndEffectivePort() {
        ServerOrigin server = ServerOrigin.parse("https://home.example.org", false);
        assertTrue(server.contains("https://HOME.example.org:443/circles/1?tab=days#post"));
        assertFalse(server.contains("http://home.example.org/circles"));
        assertFalse(server.contains("https://home.example.org:8443/circles"));
        assertFalse(server.contains("https://home.example.org.evil.org/"));
        assertFalse(server.contains("https://home.example.org@evil.org/"));
        assertFalse(server.contains("//home.example.org/"));
    }

    @Test public void rejectsCredentialsPathsAndMalformedOrigins() {
        for (String address : new String[] { "", "https://", "https://user@home.example.org",
                "https://home.example.org/path", "https://home.example.org?token=secret",
                "https://home.example.org#fragment", "https://home.example.org:",
                "https://home.example.org:0", "https://home.example.org:65536",
                "file:///data/local/tmp/test.html", "javascript:alert(1)",
                "https://home.example.org\\@evil.org", "https://ho me.example.org" }) {
            assertThrows(address, IllegalArgumentException.class, () -> ServerOrigin.parse(address, true));
        }
    }

    @Test public void permitsCleartextOnlyForExplicitDebugLoopback() {
        for (String host : new String[] { "localhost", "127.0.0.1", "[::1]" }) {
            assertEquals("http://" + host + ":7676/", ServerOrigin.parse("http://" + host + ":7676", true).url());
            assertThrows(IllegalArgumentException.class, () -> ServerOrigin.parse("http://" + host, false));
        }
        for (String host : new String[] { "home.example.org", "192.168.1.2", "localhost.evil.org", "127.0.0.2" }) {
            assertThrows(IllegalArgumentException.class, () -> ServerOrigin.parse("http://" + host, true));
        }
    }

    @Test public void neverTreatsDeviceOrIntentUrlsAsExternalWebLinks() {
        assertTrue(ServerOrigin.isExternalWebLink("https://other.example.org/path"));
        for (String url : new String[] { "intent://scan/#Intent;end", "file:///sdcard/photo.jpg",
                "content://photos/1", "javascript:alert(1)", "data:text/html,hello",
                "https://user:password@other.example.org/", "https://other.example.org:99999" }) {
            assertFalse(url, ServerOrigin.isExternalWebLink(url));
        }
    }

    @Test public void handsOnlySystemSchemesToOtherApps() {
        for (String url : new String[] { "tel:+79990000000", "mailto:me@example.org?subject=Wynd",
                "sms:+79990000000", "mms:+79990000000", "geo:55.75,37.61?z=12" }) {
            assertTrue(url, ServerOrigin.isExternalAppLink(url));
        }
        for (String url : new String[] { "https://home.example.org/", "http://home.example.org/",
                "intent://scan/#Intent;end", "file:///sdcard/photo.jpg", "javascript:alert(1)",
                "data:text/html,hello", "wynd-push:x", null }) {
            assertFalse(String.valueOf(url), ServerOrigin.isExternalAppLink(url));
        }
    }
}
