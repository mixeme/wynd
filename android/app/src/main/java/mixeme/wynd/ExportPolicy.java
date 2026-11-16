package mixeme.wynd;

import java.net.URI;
import java.util.Locale;

/** Имя — предложение системному диалогу, никогда не путь файловой системы. */
final class ExportPolicy {
    static final int CHUNK_BYTES = 32768;

    static String filename(String raw) {
        if (raw == null) return "wynd-file";
        String clean = raw.replaceAll("[\\p{Cntrl}/\\\\:*?\"<>|]", "_").trim();
        if (clean.isEmpty() || clean.equals(".") || clean.equals("..")) return "wynd-file";
        return clean.length() <= 180 ? clean : clean.substring(0, 180);
    }

    static String mime(String raw) {
        if (raw == null) return "application/octet-stream";
        String type = raw.split(";", 2)[0].trim().toLowerCase(Locale.ROOT);
        return type.matches("[a-z0-9!#$&^_.+-]+/[a-z0-9!#$&^_.+-]+") ? type : "application/octet-stream";
    }

    static boolean remote(String raw, ServerOrigin selected, boolean debug) {
        if (raw == null || raw.length() > 8192 || !ServerOrigin.isExternalWebLink(raw)) return false;
        try {
            URI uri = new URI(raw);
            return uri.getRawFragment() == null && ("https".equalsIgnoreCase(uri.getScheme())
                    || debug && selected.contains(raw));
        } catch (Exception e) { return false; }
    }
}
