package mixeme.wynd;

import java.net.URI;
import java.net.URISyntaxException;
import java.util.LinkedHashSet;
import java.util.Locale;
import java.util.Set;

/** Границы WebView-разрешений и результатов системного выбора, без Android-зависимостей. */
final class MediaPolicy {
    static final String VIDEO = "android.webkit.resource.VIDEO_CAPTURE";
    static final String AUDIO = "android.webkit.resource.AUDIO_CAPTURE";
    static final String CAMERA = "android.permission.CAMERA";
    static final String MICROPHONE = "android.permission.RECORD_AUDIO";

    static String[] permissions(String[] resources) {
        Set<String> result = new LinkedHashSet<>();
        for (String resource : resources) {
            if (VIDEO.equals(resource)) result.add(CAMERA);
            else if (AUDIO.equals(resource)) result.add(MICROPHONE);
            else return new String[0]; // Не выдавать будущие/неизвестные права WebView.
        }
        return result.toArray(new String[0]);
    }

    static String[] mimeTypes(String[] values) {
        Set<String> result = new LinkedHashSet<>();
        for (String value : values) for (String part : value.split(",")) {
            String mime = part.trim().toLowerCase(Locale.ROOT);
            if (mime.matches("[a-z0-9!#$&^_.+-]+/(\\*|[a-z0-9!#$&^_.+-]+)")) result.add(mime);
            else if (!mime.isEmpty()) return new String[] {"*/*"};
        }
        return result.isEmpty() ? new String[] {"*/*"} : result.toArray(new String[0]);
    }

    static boolean selectable(String raw, String ownAuthority) {
        try {
            URI uri = new URI(raw);
            return "content".equalsIgnoreCase(uri.getScheme()) && uri.getRawAuthority() != null
                    && !uri.getRawAuthority().isEmpty() && uri.getRawUserInfo() == null
                    && uri.getRawFragment() == null && !uri.getAuthority().equalsIgnoreCase(ownAuthority);
        } catch (URISyntaxException | NullPointerException e) { return false; }
    }
}
