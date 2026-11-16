package mixeme.wynd;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.graphics.Color;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.util.Log;
import android.view.ViewGroup;
import android.view.WindowInsets;
import android.webkit.CookieManager;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.window.OnBackInvokedDispatcher;

import java.io.ByteArrayInputStream;
import java.util.Collections;

/** Первый каркас: выбранный при dev-сборке origin открывается напрямую, без прокси. */
public final class MainActivity extends Activity {
    private static final String TAG = "WyndAndroid";
    private WebView webView;
    private ServerOrigin server;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        if (BuildConfig.START_URL.isEmpty()) {
            Log.e(TAG, "No dev server configured. Build with -PwyndDevServerUrl=https://host");
            finish();
            return;
        }
        try {
            server = ServerOrigin.parse(BuildConfig.START_URL, BuildConfig.DEBUG);
        } catch (IllegalArgumentException e) {
            Log.e(TAG, "Invalid dev server configuration", e);
            finish();
            return;
        }
        webView = new WebView(this);
        configureWebView();
        AndroidBridge.attach(this, webView, server);
        setContentView(webView);
        // target 36: системные полосы не должны закрывать существующий веб-интерфейс.
        if (Build.VERSION.SDK_INT >= 30) {
            webView.setOnApplyWindowInsetsListener((view, insets) -> {
                android.graphics.Insets bars = insets.getInsets(
                        WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout()
                                | WindowInsets.Type.ime());
                view.setPadding(bars.left, bars.top, bars.right, bars.bottom);
                return WindowInsets.CONSUMED;
            });
        }
        if (Build.VERSION.SDK_INT >= 33) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                    OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::goBack);
        }
        if (state == null || webView.restoreState(state) == null || !server.contains(webView.getUrl())) {
            webView.loadUrl(server.url());
        }
    }

    @SuppressLint("SetJavaScriptEnabled") // Wynd — Svelte; Java-мост странице не выдаётся.
    private void configureWebView() {
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG);
        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setAllowFileAccess(false);
        settings.setAllowContentAccess(false);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
        settings.setMediaPlaybackRequiresUserGesture(true);
        CookieManager.getInstance().setAcceptThirdPartyCookies(webView, false);
        webView.setBackgroundColor(Color.rgb(244, 240, 233));
        webView.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                if (!request.isForMainFrame()) return false;
                return navigate(request.getUrl().toString(), request.hasGesture());
            }

            @Override
            @SuppressWarnings("deprecation")
            public boolean shouldOverrideUrlLoading(WebView view, String url) {
                // Android 5–6 не передаёт признак жеста: чужой переход отменяем.
                return navigate(url, false);
            }

            @Override
            public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                // Включая POST-переходы: shouldOverrideUrlLoading их не перехватывает.
                String url = request.getUrl().toString();
                // На API 21–23 networkSecurityConfig ещё не действует.
                boolean foreignCleartext = "http".equalsIgnoreCase(request.getUrl().getScheme())
                        && !server.contains(url);
                if (foreignCleartext || request.isForMainFrame() && !server.contains(url)) {
                    return new WebResourceResponse("text/plain", "UTF-8", 403, "Blocked",
                            Collections.emptyMap(), new ByteArrayInputStream(new byte[0]));
                }
                // API, медиа и SSE идут обычной сетью WebView. Локальный прокси
                // настольного Wails здесь не нужен и не должен буферизовать SSE.
                return null;
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                if (server.contains(url)) Log.i(TAG, "Selected interface loaded");
            }
        });
    }

    private boolean navigate(String url, boolean userGesture) {
        if (server.contains(url)) return false;
        if (userGesture && ServerOrigin.isExternalWebLink(url)) {
            try {
                startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(url))
                        .addCategory(Intent.CATEGORY_BROWSABLE));
            } catch (ActivityNotFoundException e) {
                Log.w(TAG, "No browser available for an external link");
            }
        }
        return true;
    }

    private void goBack() {
        if (webView != null && webView.canGoBack()) webView.goBack();
        else finish();
    }

    @Override
    @SuppressLint("GestureBackNavigation") // Только API 21–32; с 33 зарегистрирован OnBackInvokedCallback.
    @SuppressWarnings("deprecation")
    public void onBackPressed() {
        goBack();
    }

    @Override
    protected void onSaveInstanceState(Bundle state) {
        if (webView != null) webView.saveState(state);
        super.onSaveInstanceState(state);
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (webView != null) webView.onResume();
        UpdateController.get(this).resume(this);
    }

    @Override
    protected void onPause() {
        UpdateController.get(this).pause(this);
        if (webView != null) {
            webView.onPause();
            CookieManager.getInstance().flush();
        }
        super.onPause();
    }

    @Override
    protected void onDestroy() {
        if (webView != null) {
            webView.stopLoading();
            if (webView.getParent() instanceof ViewGroup parent) parent.removeView(webView);
            webView.destroy();
            webView = null;
        }
        super.onDestroy();
    }
}
