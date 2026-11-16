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
import android.webkit.CookieManager;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.window.OnBackInvokedDispatcher;

import java.io.ByteArrayInputStream;
import java.util.Collections;

/** Выбранный origin открывается напрямую, без прокси и переноса данных между origin. */
public final class MainActivity extends Activity {
    private static final String TAG = "WyndAndroid";
    private WebView webView;
    private ServerOrigin server;
    private MediaChromeClient media;
    private FileExports exports;
    private OriginalPhotos originals;
    private IncomingShares shares;
    private InterfaceBridge interfaces;
    private boolean backPending;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        String selected = getSharedPreferences("interface", 0).getString("origin", BuildConfig.START_URL);
        if (selected == null || selected.isEmpty()) {
            Log.e(TAG, "No interface selected; first-start local UI is not included yet");
            finish();
            return;
        }
        try {
            server = ServerOrigin.parse(selected, BuildConfig.DEBUG);
        } catch (IllegalArgumentException e) {
            Log.e(TAG, "Invalid dev server configuration", e);
            finish();
            return;
        }
        createWebView(state, null);
        if (Build.VERSION.SDK_INT >= 33) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                    OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::goBack);
        }
    }

    private void createWebView(Bundle state, String initialPath) {
        webView = new WebView(this);
        media = new MediaChromeClient(this, webView, server);
        webView.setWebChromeClient(media);
        configureWebView();
        originals = new OriginalPhotos(this, webView, server, media);
        media.originalPhotos(originals);
        exports = new FileExports(this, webView, server);
        shares = new IncomingShares(this, webView, server);
        if (state == null && initialPath == null) receiveShare(getIntent());
        AndroidBridge.attach(this, webView, server);
        interfaces = new InterfaceBridge(this, webView, server);
        SystemBarsFrame bars = new SystemBarsFrame(this, webView);
        setContentView(bars);
        AppearanceBridge.attach(this, webView, server, bars);
        if (initialPath != null && InterfaceBridge.validPath(initialPath)) {
            webView.loadUrl(server.url() + initialPath.substring(1));
        } else if (state == null || !server.url().equals(state.getString("interfaceOrigin"))
                || webView.restoreState(state) == null || !server.contains(webView.getUrl())) {
            webView.loadUrl(server.url());
        }
    }

    boolean interfaceBusy() {
        return media != null && media.busy() || exports != null && exports.busy()
                || shares != null && shares.busy();
    }

    void openInterface(ServerOrigin next, String path) {
        // Новый WebView без старой истории. Всё выполняется одним шагом UI-потока:
        // пришедший следом SEND уже получит новый origin, а не удалится при restore.
        destroyWebView();
        server = next;
        createWebView(null, path);
        webView.onResume();
        media.resume(); exports.resume(); originals.resume(); shares.resume(); interfaces.resume();
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        receiveShare(intent);
    }

    private void receiveShare(Intent intent) {
        if (shares != null) shares.accept(intent);
        // Не держать входящий payload как Intent текущего окна после обработки.
        setIntent(new Intent(Intent.ACTION_MAIN).setClass(this, MainActivity.class));
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
            @Override public void onPageStarted(WebView view, String url, android.graphics.Bitmap favicon) {
                media.cancel();
                if (exports != null) exports.cancel();
                if (originals != null) originals.clear();
                if (interfaces != null) interfaces.invalidate();
            }
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
                if (shares != null) {
                    WebResourceResponse shared = shares.intercept(request);
                    if (shared != null) return shared;
                }
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
        if (backPending) return;
        WebView active = webView;
        if (active != null && server.contains(active.getUrl())) {
            backPending = true;
            active.evaluateJavascript("!window.dispatchEvent(new Event('wynd:android-back',{cancelable:true}))", result -> {
                backPending = false;
                if (webView == active && !isFinishing() && !isDestroyed() && !"true".equals(result)) goBackWithoutOverlay();
            });
        } else goBackWithoutOverlay();
    }

    private void goBackWithoutOverlay() {
        // Между документами JS не получает popstate. Удалить входящий набор
        // до шага истории, чтобы прежний документ не открыл выбор снова.
        if (shares != null && shares.cancelFromBack(this::goBackHistory)) return;
        goBackHistory();
    }

    private void goBackHistory() {
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
        if (server != null) state.putString("interfaceOrigin", server.url());
        super.onSaveInstanceState(state);
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (webView != null) webView.onResume();
        if (media != null) media.resume();
        if (exports != null) exports.resume();
        if (originals != null) originals.resume();
        if (shares != null) shares.resume();
        if (interfaces != null) interfaces.resume();
        UpdateController.get(this).resume(this);
    }

    @Override
    protected void onPause() {
        if (media != null) media.pause();
        if (exports != null) exports.pause();
        if (originals != null) originals.pause();
        if (shares != null) shares.pause();
        if (interfaces != null) interfaces.pause();
        UpdateController.get(this).pause(this);
        if (webView != null) {
            webView.onPause();
            CookieManager.getInstance().flush();
        }
        super.onPause();
    }

    @Override
    protected void onDestroy() {
        destroyWebView();
        super.onDestroy();
    }

    private void destroyWebView() {
        if (media != null) media.destroy();
        if (exports != null) exports.destroy();
        if (originals != null) originals.clear();
        if (shares != null) shares.destroy();
        if (interfaces != null) interfaces.destroy();
        if (webView != null) {
            webView.stopLoading();
            if (webView.getParent() instanceof ViewGroup parent) parent.removeView(webView);
            webView.destroy();
            webView = null;
        }
    }

    @Override
    @SuppressWarnings("deprecation")
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == MediaChromeClient.FILE_REQUEST && media != null) media.filesReturned(resultCode, data);
        if (requestCode == FileExports.SAVE_REQUEST && exports != null) exports.returned(resultCode, data);
    }

    @Override public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode == MediaChromeClient.PERMISSION_REQUEST && media != null) media.permissionsReturned();
        if (requestCode == FileExports.NOTIFICATION_REQUEST && exports != null) exports.notificationsReturned();
        if (requestCode == MediaChromeClient.PHOTO_LOCATION_REQUEST && media != null) media.photoPermissionsReturned();
    }
}
