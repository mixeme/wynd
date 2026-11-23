package mixeme.wynd;

import android.content.res.AssetManager;
import android.util.Log;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.HashMap;
import java.util.Locale;
import java.util.Map;

/**
 * Вшитый клиент (план 50, схема Настольного): страницы и статика выбранного
 * сервера отдаются из APK. Origin остаётся адресом сервера, поэтому сессии,
 * снимки и очередь в IndexedDB не переезжают. API, SSE, медиа, докачка и
 * скачивания идут обычной сетью.
 */
final class EmbeddedAssets {
    static final String PREFIX = "www/";
    private static final String TAG = "WyndAndroid";

    /** Файл вшитого клиента; null — такого файла нет. */
    interface Source {
        InputStream open(String path) throws IOException;
    }

    private final Source source;
    private final ServerOrigin origin;
    private boolean loggedIndex;

    EmbeddedAssets(Source source, ServerOrigin origin) {
        this.source = source;
        this.origin = origin;
    }

    static EmbeddedAssets fromAssets(final AssetManager assets, ServerOrigin origin) {
        return new EmbeddedAssets(path -> {
            try {
                return assets.open(PREFIX + path);
            } catch (IOException e) {
                return null;
            }
        }, origin);
    }

    WebResourceResponse intercept(WebResourceRequest request) {
        String method = request.getMethod();
        if (!"GET".equals(method) && !"HEAD".equals(method)) return null;
        if (!origin.contains(request.getUrl().toString())) return null;
        String asset = assetPath(request.getUrl().getPath());
        if (asset == null) return null;
        try {
            InputStream body = source.open(storagePath(asset));
            if (body != null) return response(asset, body, "HEAD".equals(method));
            if (servesIndex(asset, request.isForMainFrame())) {
                body = source.open("index.html");
                if (body == null) return null;
                if (!loggedIndex) {
                    loggedIndex = true;
                    Log.i(TAG, "embedded ui: index from APK");
                }
                return response("index.html", body, "HEAD".equals(method));
            }
        } catch (IOException ignored) {
            // Битый или недоступный asset не подменяет сеть.
        }
        return null;
    }

    /** Путь файла в APK или null, если запрос серверный либо небезопасный. */
    static String assetPath(String rawPath) {
        if (rawPath == null || !rawPath.startsWith("/")) return null;
        String path = rawPath.substring(1);
        while (path.endsWith("/")) path = path.substring(0, path.length() - 1);
        if (path.isEmpty()) return "index.html";
        if (path.indexOf('\\') >= 0 || path.indexOf('\0') >= 0) return null;
        for (String part : path.split("/", -1)) {
            if (part.isEmpty() || part.equals(".") || part.equals("..")) return null;
        }
        if (isServerPath(path)) return null;
        return path;
    }

    /**
     * Путь файла в APK: aapt не пакует каталоги с «_», поэтому `_app/…`
     * лежит как `app/…`. Наружу URL остаётся прежним.
     */
    static String storagePath(String asset) {
        return asset.startsWith("_app/") ? "app/" + asset.substring(5) : asset;
    }

    /** Маршрут клиента без расширения открывает index.html в главном фрейме. */
    static boolean servesIndex(String asset, boolean mainFrame) {
        if (!mainFrame) return false;
        String base = asset.substring(asset.lastIndexOf('/') + 1);
        return !base.contains(".");
    }

    /** Пути сервера, которые никогда не подменяются вшитым клиентом. */
    static boolean isServerPath(String path) {
        return path.equals("api") || path.startsWith("api/")
                || path.equals("health") || path.equals("ready");
    }

    static String mime(String name) {
        String base = name.substring(name.lastIndexOf('/') + 1);
        int dot = base.lastIndexOf('.');
        String ext = dot < 0 ? "" : base.substring(dot + 1).toLowerCase(Locale.ROOT);
        switch (ext) {
            case "html": return "text/html";
            case "js": case "mjs": return "application/javascript";
            case "css": return "text/css";
            case "json": return "application/json";
            case "webmanifest": return "application/manifest+json";
            case "svg": return "image/svg+xml";
            case "png": return "image/png";
            case "jpg": case "jpeg": return "image/jpeg";
            case "webp": return "image/webp";
            case "gif": return "image/gif";
            case "ico": return "image/x-icon";
            case "woff2": return "font/woff2";
            case "woff": return "font/woff";
            case "txt": return "text/plain";
            case "map": return "application/json";
            case "wasm": return "application/wasm";
            default: return "application/octet-stream";
        }
    }

    /** Те же правила кеша, что у серверного SPA (web/serve.go). */
    static String cacheControl(String name) {
        String base = name.substring(name.lastIndexOf('/') + 1);
        if (base.equals("sw.js") || base.equals("push-sw.js")
                || base.startsWith("workbox-") || base.equals("index.html")
                || name.endsWith(".webmanifest")) return "no-cache";
        if (name.startsWith("_app/immutable/")) return "public, max-age=31536000, immutable";
        if (name.startsWith("_app/")) return "no-cache";
        return "public, max-age=3600";
    }

    private static WebResourceResponse response(String name, InputStream body, boolean head) {
        Map<String, String> headers = new HashMap<>();
        headers.put("X-Content-Type-Options", "nosniff");
        headers.put("X-Frame-Options", "DENY");
        headers.put("Referrer-Policy", "strict-origin-when-cross-origin");
        headers.put("Content-Security-Policy", "frame-ancestors 'none'; object-src 'none'; base-uri 'self'");
        headers.put("Cache-Control", cacheControl(name));
        if (head) {
            try {
                body.close();
            } catch (IOException ignored) {
                // Поток HEAD уже не нужен.
            }
            body = new ByteArrayInputStream(new byte[0]);
        }
        return new WebResourceResponse(mime(name), "UTF-8", 200, "OK", headers, body);
    }
}
