package mixeme.wynd;

import java.nio.charset.StandardCharsets;

/** Ограничения SEND: отправитель не выбирает локальный путь, сервер или круг. */
final class SharePolicy {
    static final int MAX_FILES = 32;
    static final long MAX_BYTES = 512L * 1024 * 1024;
    static boolean text(String value) { return value != null && value.getBytes(StandardCharsets.UTF_8).length <= 32768; }
    static boolean media(String mime) { return PhotoPolicy.mediaMime(mime) && !mime.endsWith("/*"); }
    static boolean uri(String value) { return value != null && value.startsWith("content://") && !value.contains("@") && !value.contains("#"); }
    static boolean choicePage(String value, ServerOrigin origin) {
        if (!origin.contains(value)) return false;
        try { return "/share".equals(new java.net.URI(value).getPath()); }
        catch (java.net.URISyntaxException e) { return false; }
    }
}
