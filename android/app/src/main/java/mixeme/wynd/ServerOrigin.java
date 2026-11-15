package mixeme.wynd;

import java.net.URI;
import java.net.URISyntaxException;
import java.util.Locale;

/** Источник интерфейса, а не список учётных записей (настольный план 62). */
final class ServerOrigin {
    private final String scheme;
    private final String host;
    private final int port;
    private final String origin;

    private ServerOrigin(URI uri) {
        scheme = uri.getScheme().toLowerCase(Locale.ROOT);
        host = uri.getHost().toLowerCase(Locale.ROOT);
        port = effectivePort(uri);
        int explicitPort = uri.getPort();
        boolean defaultPort = explicitPort == -1 || explicitPort == (scheme.equals("https") ? 443 : 80);
        origin = scheme + "://" + host + (defaultPort ? "" : ":" + explicitPort);
    }

    static ServerOrigin parse(String raw, boolean debugLoopback) {
        if (raw == null) throw new IllegalArgumentException("Invalid server address");
        try {
            String address = raw.trim();
            if (!address.contains("://")) address = "https://" + address;
            URI uri = new URI(address);
            if (!isWebAddress(uri) || (uri.getRawPath() != null
                    && !uri.getRawPath().isEmpty() && !uri.getRawPath().equals("/"))
                    || uri.getRawQuery() != null || uri.getRawFragment() != null) {
                throw new IllegalArgumentException("Invalid server address");
            }
            if (!uri.getScheme().equalsIgnoreCase("https")
                    && !(debugLoopback && isLoopback(uri.getHost()))) {
                throw new IllegalArgumentException("HTTPS is required outside the USB debug loopback");
            }
            return new ServerOrigin(uri);
        } catch (URISyntaxException e) {
            throw new IllegalArgumentException("Invalid server address", e);
        }
    }

    String url() {
        return origin + "/";
    }

    boolean contains(String url) {
        if (url == null) return false;
        try {
            URI uri = new URI(url);
            return isWebAddress(uri) && scheme.equalsIgnoreCase(uri.getScheme())
                    && host.equalsIgnoreCase(uri.getHost()) && port == effectivePort(uri);
        } catch (URISyntaxException e) {
            return false;
        }
    }

    static boolean isExternalWebLink(String url) {
        if (url == null) return false;
        try {
            return isWebAddress(new URI(url));
        } catch (URISyntaxException e) {
            return false;
        }
    }

    private static boolean isWebAddress(URI uri) {
        String scheme = uri.getScheme();
        return ("https".equalsIgnoreCase(scheme) || "http".equalsIgnoreCase(scheme))
                && uri.getHost() != null && !uri.getHost().isEmpty()
                && uri.getRawUserInfo() == null
                && (uri.getPort() == -1 || uri.getPort() > 0 && uri.getPort() <= 65535)
                && !uri.getRawAuthority().endsWith(":");
    }

    private static int effectivePort(URI uri) {
        return uri.getPort() == -1 ? ("https".equalsIgnoreCase(uri.getScheme()) ? 443 : 80) : uri.getPort();
    }

    private static boolean isLoopback(String host) {
        return "localhost".equalsIgnoreCase(host) || "127.0.0.1".equals(host)
                || "[::1]".equals(host);
    }
}
