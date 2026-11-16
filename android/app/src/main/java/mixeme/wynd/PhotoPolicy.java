package mixeme.wynd;

/** Ограничения выбора фото и временных локальных ссылок; без Android API. */
final class PhotoPolicy {
    static final String ACCEPT_PREFIX = "application/x-wynd-original-";
    static boolean token(String value) { return value != null && value.matches("[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}"); }
    static boolean mediaMime(String value) { return value != null && value.matches("(image|video)/(\\*|[a-z0-9!#$&^_.+-]+)"); }
    static boolean localMedia(String authority) {
        return "media".equals(authority) || "com.android.externalstorage.documents".equals(authority)
                || "com.android.providers.media.documents".equals(authority);
    }
}
