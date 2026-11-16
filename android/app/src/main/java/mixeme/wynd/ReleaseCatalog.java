package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.text.ParsePosition;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;
import java.util.TimeZone;

final class ReleaseCatalog {
    static final class Release {
        final String version;
        final int code;
        final String apkUrl;
        final String checksumUrl;
        final boolean requiresBypass;

        Release(String version, String apkUrl, String checksumUrl, boolean requiresBypass) {
            this.version = version;
            code = UpdatePolicy.versionCode(version);
            this.apkUrl = apkUrl;
            this.checksumUrl = checksumUrl;
            this.requiresBypass = requiresBypass;
        }
    }

    static Release select(String json, int installedCode, long now, boolean debug, boolean developmentMode,
                          boolean immediately, String fixtureOrigin) throws JSONException {
        JSONArray rows = new JSONArray(json);
        Release best = null;
        for (int i = 0; i < rows.length(); i++) {
            JSONObject row = rows.optJSONObject(i);
            if (row == null || !row.has("draft") || !row.has("prerelease")) continue;
            if (!UpdatePolicy.eligible(publishedAt(row.optString("published_at")), now,
                    row.optBoolean("draft", true), row.optBoolean("prerelease", true), developmentMode, immediately)) continue;
            String tag = row.optString("tag_name");
            if (!tag.startsWith("android-v")) continue;
            String version = tag.substring("android-v".length());
            int code;
            try { code = UpdatePolicy.versionCode(version); }
            catch (IllegalArgumentException e) { continue; }
            if (code <= installedCode || best != null && code <= best.code) continue;
            JSONArray assets = row.optJSONArray("assets");
            if (assets == null) continue;
            String apk = null;
            String checksum = null;
            boolean duplicate = false;
            for (int j = 0; j < assets.length(); j++) {
                JSONObject asset = assets.optJSONObject(j);
                if (asset == null) continue;
                String name = asset.optString("name");
                if (name.equals(UpdatePolicy.ASSET)) {
                    duplicate |= apk != null;
                    apk = asset.optString("browser_download_url");
                } else if (name.equals(UpdatePolicy.ASSET + ".sha256")) {
                    duplicate |= checksum != null;
                    checksum = asset.optString("browser_download_url");
                }
            }
            if (duplicate || apk == null || checksum == null) continue;
            boolean fixture = debug && fixtureOrigin != null && !fixtureOrigin.isEmpty();
            String expected = fixture ? fixtureOrigin + "/releases/download/" + tag + "/" + UpdatePolicy.ASSET
                    : UpdatePolicy.assetUrl(version);
            if (!apk.equals(expected) || !checksum.equals(expected + ".sha256")) continue;
            best = new Release(version, apk, checksum,
                    now - publishedAt(row.optString("published_at")) < UpdatePolicy.DELAY_MILLIS);
        }
        return best;
    }

    private static long publishedAt(String value) {
        if (value.length() != 20) return 0;
        SimpleDateFormat format = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", Locale.ROOT);
        format.setTimeZone(TimeZone.getTimeZone("UTC"));
        format.setLenient(false);
        ParsePosition position = new ParsePosition(0);
        Date date = format.parse(value, position);
        return date != null && position.getIndex() == value.length() ? date.getTime() : 0;
    }
}
