package mixeme.wynd;

import org.junit.Test;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertTrue;

public class EmbeddedAssetsTest {
    @Test public void rootAndClientRoutes() {
        assertEquals("index.html", EmbeddedAssets.assetPath("/"));
        assertEquals("index.html", EmbeddedAssets.assetPath("//"));
        assertEquals("circles/abc", EmbeddedAssets.assetPath("/circles/abc"));
        assertEquals("circles/abc", EmbeddedAssets.assetPath("/circles/abc/"));
        assertEquals("icons.svg", EmbeddedAssets.assetPath("/icons.svg"));
        assertEquals("_app/immutable/entry/start.js", EmbeddedAssets.assetPath("/_app/immutable/entry/start.js"));
    }

    @Test public void serverPathsStayOnServer() {
        assertNull(EmbeddedAssets.assetPath("/api"));
        assertNull(EmbeddedAssets.assetPath("/api/v1/instance"));
        assertNull(EmbeddedAssets.assetPath("/api/v1/sync"));
        assertNull(EmbeddedAssets.assetPath("/health"));
        assertNull(EmbeddedAssets.assetPath("/ready"));
    }

    @Test public void unsafeOrAmbiguousPathsPassToNetwork() {
        assertNull(EmbeddedAssets.assetPath(null));
        assertNull(EmbeddedAssets.assetPath("circles/abc"));
        assertNull(EmbeddedAssets.assetPath("/../secret"));
        assertNull(EmbeddedAssets.assetPath("/circles/../secret"));
        assertNull(EmbeddedAssets.assetPath("/circles//abc"));
        assertNull(EmbeddedAssets.assetPath("/circles\\abc"));
        assertNull(EmbeddedAssets.assetPath("/circles/./abc"));
    }

    @Test public void underscoreAppIsStoredWithoutUnderscore() {
        assertEquals("app/immutable/entry/start.js",
                EmbeddedAssets.storagePath("_app/immutable/entry/start.js"));
        assertEquals("icons.svg", EmbeddedAssets.storagePath("icons.svg"));
        assertEquals("_application.js", EmbeddedAssets.storagePath("_application.js"));
    }

    @Test public void onlyMainFrameExtensionlessRoutesFallBackToIndex() {
        assertTrue(EmbeddedAssets.servesIndex("circles/abc", true));
        assertTrue(EmbeddedAssets.servesIndex("settings/app", true));
        assertFalse(EmbeddedAssets.servesIndex("circles/abc", false));
        assertFalse(EmbeddedAssets.servesIndex("icons.svg", true));
        assertFalse(EmbeddedAssets.servesIndex("_app/immutable/entry/start.js", true));
        assertFalse(EmbeddedAssets.servesIndex("manifest.webmanifest", true));
    }

    @Test public void mimeTypesMatchTheClientBuild() {
        assertEquals("text/html", EmbeddedAssets.mime("index.html"));
        assertEquals("application/javascript", EmbeddedAssets.mime("sw.js"));
        assertEquals("application/javascript", EmbeddedAssets.mime("_app/immutable/chunk.mjs"));
        assertEquals("text/css", EmbeddedAssets.mime("_app/immutable/chunk.css"));
        assertEquals("application/manifest+json", EmbeddedAssets.mime("manifest.webmanifest"));
        assertEquals("image/svg+xml", EmbeddedAssets.mime("icons.svg"));
        assertEquals("font/woff2", EmbeddedAssets.mime("fonts/GolosText-Variable-latin.woff2"));
        assertEquals("application/octet-stream", EmbeddedAssets.mime("unknown"));
    }

    @Test public void cacheRulesFollowTheServerSpa() {
        assertEquals("no-cache", EmbeddedAssets.cacheControl("index.html"));
        assertEquals("no-cache", EmbeddedAssets.cacheControl("sw.js"));
        assertEquals("no-cache", EmbeddedAssets.cacheControl("workbox-2fbc6a65.js"));
        assertEquals("no-cache", EmbeddedAssets.cacheControl("manifest.webmanifest"));
        assertEquals("public, max-age=31536000, immutable",
                EmbeddedAssets.cacheControl("_app/immutable/entry/start.js"));
        assertEquals("no-cache", EmbeddedAssets.cacheControl("_app/entry.js"));
        assertEquals("public, max-age=3600", EmbeddedAssets.cacheControl("icons.svg"));
    }
}
