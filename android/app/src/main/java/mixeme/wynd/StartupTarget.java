package mixeme.wynd;

import java.io.UnsupportedEncodingException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.URLDecoder;
import java.net.URLEncoder;

/** Разделяет адрес проверки и временную цель приглашения; ничего не сохраняет. */
final class StartupTarget {
    private final ServerOrigin origin;
    private final String path;

    private StartupTarget(ServerOrigin origin, String path) {
        this.origin = origin;
        this.path = path;
    }

    static StartupTarget parse(String input, boolean debugLoopback) {
        if (input == null || input.trim().isEmpty() || input.length() > 4096) {
            throw new IllegalArgumentException("Invalid startup target");
        }
        String address = input.trim();
        if (!address.contains("://")) address = "https://" + address;
        try {
            URI uri = new URI(address);
            if (uri.getRawAuthority() == null || uri.getRawFragment() != null) {
                throw new IllegalArgumentException("Invalid startup target");
            }
            ServerOrigin origin = ServerOrigin.parse(uri.getScheme() + "://" + uri.getRawAuthority(), debugLoopback);
            String path = uri.getRawPath();
            if (path == null || path.isEmpty() || path.equals("/")) {
                if (uri.getRawQuery() != null) {
                    // Ссылка подключения устройства (план 72): «/?email=почта» —
                    // вход сразу с подставленной почтой. Только один параметр.
                    return new StartupTarget(origin, loginPath(queryValue(uri.getRawQuery())));
                }
                return new StartupTarget(origin, "/");
            }
            // Генерируемые Wynd токены — hex; сохраняем совместимость и с URL-safe токенами.
            if (!path.matches("^/(invite|join)/[A-Za-z0-9_-]{1,512}/?$")) {
                throw new IllegalArgumentException("Invalid invitation path");
            }
            if (path.endsWith("/")) path = path.substring(0, path.length() - 1);
            if (uri.getRawQuery() != null) path += "?" + uri.getRawQuery();
            return new StartupTarget(origin, path);
        } catch (URISyntaxException error) {
            throw new IllegalArgumentException("Invalid startup target");
        }
    }

    /** Цель того же сервера — вход с подставленной почтой (план 72). */
    static StartupTarget withEmail(StartupTarget target, String email) {
        return new StartupTarget(target.origin, loginPath(email));
    }

    /** «/?email=…» только с одной валидной почтой; прочее — отказ. */
    private static String loginPath(String email) {
        if (email == null || !email.matches("[A-Za-z0-9._%+'-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}")) {
            throw new IllegalArgumentException("Invalid login email");
        }
        try {
            return "/?email=" + URLEncoder.encode(email, "UTF-8");
        } catch (UnsupportedEncodingException error) {
            throw new IllegalArgumentException("Invalid login email");
        }
    }

    /** Значение единственного параметра строки запроса, иначе null. */
    private static String queryValue(String rawQuery) {
        String[] pairs = rawQuery.split("&");
        if (pairs.length != 1) return null;
        String[] kv = pairs[0].split("=", 2);
        if (kv.length != 2 || !"email".equals(kv[0])) return null;
        try {
            return URLDecoder.decode(kv[1], "UTF-8");
        } catch (UnsupportedEncodingException error) {
            return null;
        }
    }

    ServerOrigin origin() { return origin; }
    String path() { return path; }
    String url() { return origin.url() + path.substring(1); }
}
