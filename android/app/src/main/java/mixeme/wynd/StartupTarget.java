package mixeme.wynd;

import java.net.URI;
import java.net.URISyntaxException;

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
                if (uri.getRawQuery() != null) throw new IllegalArgumentException("Invalid server address");
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

    ServerOrigin origin() { return origin; }
    String path() { return path; }
    String url() { return origin.url() + path.substring(1); }
}
