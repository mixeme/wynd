package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONObject;
import org.junit.Test;
import static org.junit.Assert.*;

public final class UpdatePolicyTest {
    private static final long PUBLISHED = 1_700_000_000_000L;

    @Test public void ordinaryBuildsGetReleaseImmediately() {
        // До 1.0.0 отлёжка снята: опубликованный выпуск доступен сразу.
        assertEquals(0, UpdatePolicy.DELAY_MILLIS);
        assertTrue(UpdatePolicy.eligible(PUBLISHED, PUBLISHED, false, false, false, false));
        assertTrue(UpdatePolicy.eligible(PUBLISHED, PUBLISHED + 1000, false, false, false, true));
        assertFalse(UpdatePolicy.eligible(PUBLISHED, PUBLISHED - 1, false, false, false, false));
    }

    @Test public void developmentSwitchDoesNotChangeTrustBoundaries() {
        // Обходить нечего, пока выдержка нулевая; границы доверия прежние.
        assertTrue(UpdatePolicy.eligible(PUBLISHED, PUBLISHED, false, false, true, true));
        assertTrue(UpdatePolicy.eligible(PUBLISHED, PUBLISHED, false, false, true, false));
        assertFalse(UpdatePolicy.eligible(PUBLISHED, PUBLISHED - 1, false, false, true, true));
        assertFalse(UpdatePolicy.eligible(PUBLISHED, PUBLISHED, true, false, true, true));
        assertFalse(UpdatePolicy.eligible(PUBLISHED, PUBLISHED, false, true, true, true));
    }

    @Test public void versionMappingIsStrictAndMonotonic() {
        assertEquals(1000, UpdatePolicy.versionCode("0.1.0"));
        assertEquals(1001, UpdatePolicy.versionCode("0.1.1"));
        assertEquals(1_000_000, UpdatePolicy.versionCode("1.0.0"));
        for (String version : new String[] {"0.0.0", "01.2.3", "1.1000.0", "1.0.1000", "2101.0.0", "1.2.3-beta", "1.2", "999999999999999999999999.0.0"}) {
            assertThrows(version, IllegalArgumentException.class, () -> UpdatePolicy.versionCode(version));
        }
    }

    @Test public void rejectsRedirectEscapeEvenInDevelopment() {
        assertTrue(UpdatePolicy.transportUrlAllowed("https://release-assets.githubusercontent.com/path?signature=x", false, ""));
        assertTrue(UpdatePolicy.transportUrlAllowed("http://127.0.0.1:7697/releases", true, "http://127.0.0.1:7697"));
        for (String url : new String[] {"http://github.com/", "https://github.com.evil.org/", "https://user@github.com/", "https://github.com:444/", "https://evil.org/", "file:///tmp/apk", "http://127.0.0.1:7698/"}) {
            assertFalse(url, UpdatePolicy.transportUrlAllowed(url, true, "http://127.0.0.1:7697"));
        }
        assertFalse(UpdatePolicy.transportUrlAllowed("http://127.0.0.1:7697/", false, "http://127.0.0.1:7697"));
    }

    @Test public void selectsNewestVersionWhenAllPublishingTimesAreEligible() throws Exception {
        JSONArray rows = new JSONArray().put(release("0.2.0", "2026-11-16T00:00:00Z"))
                .put(release("0.1.1", "2026-11-01T00:00:00Z"));
        long now = 1_794_787_200_000L; // 2026-11-16 UTC
        // Отлёжка нулевая: свежий выпуск доступен сразу, обход не нужен.
        assertEquals("0.2.0", ReleaseCatalog.select(rows.toString(), 1000, now, false, false, false, "").version);
        assertFalse(ReleaseCatalog.select(rows.toString(), 1000, now, false, false, false, "").requiresBypass);
        assertEquals("0.2.0", ReleaseCatalog.select(rows.toString(), 1000, now, false, true, true, "").version);
        assertFalse(ReleaseCatalog.select(rows.toString(), 1000, now, false, true, true, "").requiresBypass);
        assertNull(ReleaseCatalog.select(rows.toString(), 2000, now, true, true, true, ""));
    }

    @Test public void rejectsForeignAssetsMalformedDatesAndDuplicateAssets() throws Exception {
        JSONObject foreign = release("0.1.1", "2026-11-01T00:00:00Z");
        foreign.getJSONArray("assets").getJSONObject(0).put("browser_download_url", "https://evil.org/wynd-android.apk");
        JSONObject duplicate = release("0.1.2", "2026-11-01T00:00:00Z");
        duplicate.getJSONArray("assets").put(duplicate.getJSONArray("assets").getJSONObject(0));
        JSONObject invalidDate = release("0.1.3", "2026-99-01T00:00:00Z");
        JSONObject draft = release("0.1.4", "2026-11-01T00:00:00Z").put("draft", true);
        JSONObject prerelease = release("0.1.5", "2026-11-01T00:00:00Z").put("prerelease", true);
        assertNull(ReleaseCatalog.select(new JSONArray().put(foreign).put(duplicate).put(invalidDate).put(draft).put(prerelease).toString(), 1000, Long.MAX_VALUE, true, true, true, ""));
    }

    private JSONObject release(String version, String publishedAt) throws Exception {
        String url = UpdatePolicy.assetUrl(version);
        return new JSONObject().put("tag_name", "android-v" + version).put("published_at", publishedAt)
                .put("draft", false).put("prerelease", false).put("assets", new JSONArray()
                        .put(new JSONObject().put("name", UpdatePolicy.ASSET).put("browser_download_url", url))
                        .put(new JSONObject().put("name", UpdatePolicy.ASSET + ".sha256").put("browser_download_url", url + ".sha256")));
    }
}
