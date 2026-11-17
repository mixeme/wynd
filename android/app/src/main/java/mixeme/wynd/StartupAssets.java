package mixeme.wynd;

import android.content.Context;
import android.webkit.WebResourceResponse;
import androidx.webkit.WebViewAssetLoader;
import java.io.ByteArrayInputStream;
import java.util.HashMap;
import java.util.Map;

/** Отдельный HTTPS origin только для собранных стартовых экранов, без сети и service worker. */
final class StartupAssets {
    static final String ORIGIN = "https://appassets.androidplatform.net";
    static final String URL = ORIGIN + "/startup/index.html";
    private final WebViewAssetLoader loader;

    StartupAssets(Context context) {
        loader = new WebViewAssetLoader.Builder()
                .addPathHandler("/", new WebViewAssetLoader.AssetsPathHandler(context)).build();
    }
    WebResourceResponse intercept(android.net.Uri uri) {
        WebResourceResponse response = null;
        if (ServerOrigin.parse(ORIGIN, false).contains(uri.toString()) && uri.getPath() != null) {
            String path = uri.getPath();
            if (path.equals("/icons.svg") || path.startsWith("/fonts/")) uri = uri.buildUpon().path("/startup" + path).build();
            if (uri.getPath().startsWith("/startup/")) response = loader.shouldInterceptRequest(uri);
        }
        if (response == null) return new WebResourceResponse("text/plain", "UTF-8", 403, "Blocked",
                java.util.Collections.emptyMap(), new ByteArrayInputStream(new byte[0]));
        Map<String, String> headers = new HashMap<>();
        headers.put("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "
                + "img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'none'; object-src 'none'; frame-src 'none'; base-uri 'none'");
        headers.put("Cache-Control", "no-store");
        headers.put("X-Content-Type-Options", "nosniff");
        response.setResponseHeaders(headers);
        return response;
    }
}
