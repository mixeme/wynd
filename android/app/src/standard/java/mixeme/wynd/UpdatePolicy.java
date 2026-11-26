package mixeme.wynd;

import java.net.URI;
import java.net.URISyntaxException;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Общие правила выбора выпуска; обход выдержки не меняет границы доверия. */
final class UpdatePolicy {
    // До 1.0.0 отлёжка снята (решение владельца 2026-11-26): выпуск предлагается
    // сразу после публикации. Вернуть 72L * 60 * 60 * 1000 при выходе 1.0.0.
    static final long DELAY_MILLIS = 0;
    static final String REPOSITORY = "mixeme/wynd";
    static final String ASSET = "wynd-android.apk";
    static final String RELEASES_API = "https://api.github.com/repos/" + REPOSITORY + "/releases";
    private static final Pattern VERSION = Pattern.compile("(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)");

    static int versionCode(String version) {
        Matcher match = VERSION.matcher(version);
        if (!match.matches()) throw new IllegalArgumentException("Invalid version");
        try {
            long major = Long.parseLong(match.group(1));
            long minor = Long.parseLong(match.group(2));
            long patch = Long.parseLong(match.group(3));
            if (major > 2100 || minor > 999 || patch > 999) throw new IllegalArgumentException("Version overflow");
            long code = major * 1_000_000 + minor * 1000 + patch;
            if (code <= 0 || code > 2_100_000_000) throw new IllegalArgumentException("Version overflow");
            return (int) code;
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("Version overflow", e);
        }
    }

    static boolean eligible(long publishedAt, long now, boolean draft, boolean prerelease,
                            boolean developmentMode, boolean immediately) {
        return !draft && !prerelease && publishedAt > 0 && publishedAt <= now
                && (developmentMode && immediately || now - publishedAt >= DELAY_MILLIS);
    }

    static String assetUrl(String version) {
        versionCode(version);
        return "https://github.com/" + REPOSITORY + "/releases/download/android-v" + version + "/" + ASSET;
    }

    static boolean transportUrlAllowed(String url, boolean debugBuild, String fixtureOrigin) {
        try {
            URI uri = new URI(url);
            if (uri.getRawUserInfo() != null || uri.getRawFragment() != null || uri.getHost() == null) return false;
            if (debugBuild && fixtureOrigin != null && !fixtureOrigin.isEmpty()
                    && ServerOrigin.parse(fixtureOrigin, true).contains(url)) return true;
            if (!"https".equalsIgnoreCase(uri.getScheme()) || uri.getPort() != -1 && uri.getPort() != 443) return false;
            String host = uri.getHost();
            return host.equalsIgnoreCase("api.github.com") || host.equalsIgnoreCase("github.com")
                    || host.equalsIgnoreCase("release-assets.githubusercontent.com")
                    || host.equalsIgnoreCase("objects.githubusercontent.com");
        } catch (URISyntaxException | IllegalArgumentException e) {
            return false;
        }
    }
}
