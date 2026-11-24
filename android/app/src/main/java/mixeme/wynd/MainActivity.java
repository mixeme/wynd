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

/** Выбранный источник открывается напрямую, без прокси; данные WebView остаются у своих origin. */
public final class MainActivity extends Activity {
    private static final String TAG = "WyndAndroid";
    private static final long COLD_RETRY_DELAY_MS = 250;

    private enum ColdLoad { FIRST, WAITING, ISSUED, STARTED, FINISHED }

    private WebView webView;
    private ServerOrigin server;
    private EmbeddedAssets embedded;
    private NativeQueueBridge queueBridge;
    private ArchiveBridge archiveBridge;
    private MediaChromeClient media;
    private FileExports exports;
    private OriginalPhotos originals;
    private IncomingShares shares;
    private PushBridge push;
    private InterfaceBridge interfaces;
    private StartupBridge startup;
    private InterfaceCatalog catalog;
    private String failedUrl;
    private boolean localStartup, resumed;
    private boolean backPending;
    private boolean embeddedReset;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        try {
            catalog = InterfaceBridge.open(this);
        } catch (Exception e) {
            Log.e(TAG, "Cannot read interface catalog", e);
            finish();
            return;
        }
        if (Build.VERSION.SDK_INT >= 33) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                    OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::goBack);
        }
        try {
            String selected = catalog.selected();
            if (selected.isEmpty() || state != null && state.getBoolean("local_startup")) {
                showStartup(!selected.isEmpty() && state != null && state.getBoolean("startup_unavailable")
                        ? ServerOrigin.parse(selected, BuildConfig.DEBUG).url() : null);
            }
            else showInterface(ServerOrigin.parse(selected, BuildConfig.DEBUG).url(), state, true);
        } catch (Exception e) { Log.e(TAG, "Cannot open interface", e); finish(); }
    }

    private void createPage(ServerOrigin origin, boolean local, String coldUrl) {
        destroyPage();
        server = origin;
        localStartup = local;
        backPending = false;
        // Вшитый клиент (план 50, схема Настольного): по умолчанию интерфейс
        // с сервера; сборка с -PwyndEmbeddedUi=true отдаёт страницы из APK.
        embedded = !local && BuildConfig.EMBEDDED_UI
                ? EmbeddedAssets.fromAssets(getAssets(), server)
                : null;
        embeddedReset = embedded != null && embeddedNeedsReset();
        if (embedded != null) Log.i(TAG, "embedded ui: interface from APK");
        webView = new WebView(this);
        media = new MediaChromeClient(this, webView, server);
        webView.setWebChromeClient(media);
        configureWebView(coldUrl);
        if (!local) {
            originals = new OriginalPhotos(this, webView, server, media);
            media.originalPhotos(originals);
            exports = new FileExports(this, webView, server);
            shares = new IncomingShares(this, webView, server);
            push = new PushBridge(this, webView, server);
            interfaces = new InterfaceBridge(this, webView, server, catalog);
            // Фоновая очередь (план 50, этап 5.1): записи без вложений уходят
            // и после закрытия окна. Прогон — WorkManager, страница не нужна.
            queueBridge = NativeQueueBridge.attach(this, webView, server);
            archiveBridge = ArchiveBridge.attach(this, webView, server);
            NativeQueueWorker.schedule(this);
        } else startup = new StartupBridge(this, webView, catalog, failedUrl);
        AndroidBridge.attach(this, webView, server);
        SystemBarsFrame bars = new SystemBarsFrame(this, webView);
        setContentView(bars);
        AppearanceBridge.attach(this, webView, server, bars);
        if (resumed) resumePage();
    }

    private void showInterface(String url, Bundle state, boolean cold) {
        ServerOrigin origin = ServerOrigin.parse(java.net.URI.create(url).resolve("/").toString(), BuildConfig.DEBUG);
        String pushPath = WyndPushService.route(this, getIntent(), origin.url());
        if (pushPath != null) {
            url = origin.url() + pushPath;
            state = null;
        }
        failedUrl = null;
        createPage(origin, false, cold && state == null && url.startsWith("https://") ? url : null);
        if (state == null) receiveShare(getIntent());
        if (state == null || !server.url().equals(state.getString("interface_origin"))
                || webView.restoreState(state) == null || !server.contains(webView.getUrl())) {
            webView.loadUrl(url);
        }
    }

    void openStartupTarget(String url) { showInterface(url, null, false); }

    void showStartup(String unavailable) {
        failedUrl = unavailable;
        createPage(ServerOrigin.parse(StartupAssets.ORIGIN, false), true, null);
        webView.loadUrl(StartupAssets.URL);
    }

    private void unavailable(WebView failed, String url) {
        if (failed != webView || localStartup || isFinishing() || isDestroyed()) return;
        String retry;
        try { retry = StartupTarget.parse(url, BuildConfig.DEBUG).url(); }
        catch (IllegalArgumentException error) { retry = server.url(); }
        showStartup(retry);
    }

    boolean interfaceSwitchBlocked() {
        return media != null && media.busy() || exports != null && exports.busy()
                || IncomingShares.pendingForInterface(this);
    }

    void restartInterface() {
        // Новый WebView получает все origin-bound мосты заново; прежний теряет доступ.
        if (webView != null) webView.stopLoading();
        recreate();
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        if (WyndPushService.isOpen(intent)) {
            if (!localStartup && webView != null) {
                String path = WyndPushService.route(this, intent, server.url());
                if (path != null) webView.loadUrl(server.url() + path);
            }
            return;
        }
        receiveShare(intent);
    }

    private void receiveShare(Intent intent) {
        if (shares == null) {
            // Входящее «Поделиться» на локальном старте ждёт выбора сервера.
            // Проверки содержимого и URI выполнит IncomingShares после перехода.
            setIntent(intent);
            return;
        }
        shares.accept(intent);
        // Не держать входящий payload как Intent текущего окна после обработки.
        setIntent(new Intent(Intent.ACTION_MAIN).setClass(this, MainActivity.class));
    }

    @SuppressLint("SetJavaScriptEnabled") // Wynd — Svelte; Java-мост странице не выдаётся.
    private void configureWebView(String coldUrl) {
        final StartupAssets assets = localStartup ? new StartupAssets(this) : null;
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
            private ColdLoad coldLoad = ColdLoad.FIRST;

            @Override public void onPageStarted(WebView view, String url, android.graphics.Bitmap favicon) {
                if (view != webView) return;
                if (coldUrl != null && coldLoad != ColdLoad.FINISHED) {
                    if (!coldUrl.equals(url)) finishColdLoad(view);
                    else if (coldLoad == ColdLoad.ISSUED) coldLoad = ColdLoad.STARTED;
                }
                media.cancel();
                if (exports != null) exports.cancel();
                if (originals != null) originals.clear();
            }

            @Override public void onPageCommitVisible(WebView view, String url) {
                if (view == webView && coldUrl != null
                        && (coldLoad == ColdLoad.FIRST || coldLoad == ColdLoad.STARTED)) finishColdLoad(view);
            }

            private void finishColdLoad(WebView view) {
                coldLoad = ColdLoad.FINISHED;
                view.setAlpha(1f);
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
                if (assets != null) return assets.intercept(request.getUrl());
                if (shares != null) {
                    WebResourceResponse shared = shares.intercept(request);
                    if (shared != null) return shared;
                }
                WebResourceResponse archive = ArchiveStore.intercept(MainActivity.this, server, request);
                if (archive != null) return archive;
                if (embedded != null) {
                    WebResourceResponse page = embedded.intercept(request);
                    if (page != null) return page;
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
                if (!server.contains(url)) return;
                Log.i(TAG, "Selected interface loaded");
                if (embeddedReset) {
                    embeddedReset = false;
                    markEmbeddedReset();
                    // Старый сервис-воркер от прежней сборки отдаёт свой index
                    // и чанки, которых в этом APK уже нет (как чистка Service
                    // Worker в Настольном). Снимаем регистрации и кэши один раз
                    // на обновление APK; IndexedDB, сессии и очередь не трогаем.
                    view.evaluateJavascript("(async()=>{try{const rs=await navigator.serviceWorker"
                            + ".getRegistrations();await Promise.all(rs.map(r=>r.unregister()));"
                            + "const ks=await caches.keys();await Promise.all(ks.map(k=>caches.delete(k)));}"
                            + "catch(e){}location.reload();})()", null);
                }
            }

            @Override public void onReceivedError(WebView view, WebResourceRequest request, android.webkit.WebResourceError error) {
                // Платформа вызывает этот overload только с API 23; guard и для lint при minSdk 21.
                if (Build.VERSION.SDK_INT < 23) return;
                if (request.isForMainFrame() && assets == null && server.contains(request.getUrl().toString())) {
                    if (error.getErrorCode() == ERROR_CONNECT && retryColdLoad(view, request)) return;
                    view.post(() -> unavailable(view, request.getUrl().toString()));
                }
            }

            @Override public void onReceivedHttpError(WebView view, WebResourceRequest request, WebResourceResponse response) {
                if (request.isForMainFrame() && assets == null && response.getStatusCode() >= 500
                        && server.contains(request.getUrl().toString())) {
                    if (response.getStatusCode() == 503 && retryColdLoad(view, request)) return;
                    view.post(() -> unavailable(view, request.getUrl().toString()));
                }
            }

            private boolean retryColdLoad(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                if (view != webView || !url.equals(coldUrl) || !"GET".equals(request.getMethod())) return false;
                if (coldLoad == ColdLoad.WAITING) return true;
                if (coldLoad != ColdLoad.FIRST) return false;

                // Не показываем документ ошибки между двумя загрузками.
                coldLoad = ColdLoad.WAITING;
                view.setAlpha(0f);
                view.postDelayed(() -> {
                    if (view != webView || coldLoad != ColdLoad.WAITING || isFinishing() || isDestroyed()) return;
                    coldLoad = ColdLoad.ISSUED;
                    view.loadUrl(url);
                }, COLD_RETRY_DELAY_MS);
                return true;
            }

            @Override @SuppressWarnings("deprecation")
            public void onReceivedError(WebView view, int code, String description, String failingUrl) {
                if (Build.VERSION.SDK_INT < 23 && assets == null && server.contains(failingUrl)) view.post(() -> unavailable(view, failingUrl));
            }
        });
    }

    /** Сброс вшитого воркера нужен один раз на установленное обновление APK. */
    private boolean embeddedNeedsReset() {
        try {
            long updated = getPackageManager().getPackageInfo(getPackageName(), 0).lastUpdateTime;
            return getSharedPreferences("wynd", MODE_PRIVATE)
                    .getLong("embedded_sw_reset", 0) != updated;
        } catch (Exception ignored) {
            return false;
        }
    }

    private void markEmbeddedReset() {
        try {
            long updated = getPackageManager().getPackageInfo(getPackageName(), 0).lastUpdateTime;
            getSharedPreferences("wynd", MODE_PRIVATE).edit()
                    .putLong("embedded_sw_reset", updated).apply();
        } catch (Exception ignored) {
            // Маркер не записался — следующий запуск повторит сброс.
        }
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
        state.putBoolean("local_startup", localStartup);
        state.putBoolean("startup_unavailable", failedUrl != null);
        if (server != null) state.putString("interface_origin", server.url());
        if (webView != null) webView.saveState(state);
        super.onSaveInstanceState(state);
    }

    @Override
    protected void onResume() {
        super.onResume();
        resumed = true;
        resumePage();
        UpdateController.get(this).resume(this);
    }

    private void resumePage() {
        if (webView != null) webView.onResume();
        if (media != null) media.resume();
        if (exports != null) exports.resume();
        if (originals != null) originals.resume();
        if (shares != null) shares.resume();
    }

    @Override
    protected void onPause() {
        resumed = false;
        if (media != null) media.pause();
        if (exports != null) exports.pause();
        if (originals != null) originals.pause();
        if (shares != null) shares.pause();
        UpdateController.get(this).pause(this);
        if (webView != null) {
            webView.onPause();
            CookieManager.getInstance().flush();
        }
        super.onPause();
    }

    @Override
    protected void onDestroy() {
        destroyPage();
        super.onDestroy();
    }

    private void destroyPage() {
        if (push != null) {
            push.destroy();
            push = null;
        }
        if (startup != null) startup.destroy();
        if (interfaces != null) interfaces.destroy();
        if (media != null) media.destroy();
        if (exports != null) exports.destroy();
        if (originals != null) originals.clear();
        if (shares != null) shares.destroy();
        if (webView != null) {
            webView.stopLoading();
            if (webView.getParent() instanceof ViewGroup parent) parent.removeView(webView);
            webView.destroy();
            webView = null;
        }
        embedded = null;
        startup = null;
        if (queueBridge != null) {
            queueBridge.destroy();
            queueBridge = null;
        }
        if (archiveBridge != null) {
            archiveBridge.destroy();
            archiveBridge = null;
        }
        interfaces = null;
        media = null;
        exports = null;
        originals = null;
        shares = null;
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
        if (requestCode == PushBridge.PERMISSION_REQUEST && push != null) push.permissionResult();
        if (requestCode == MediaChromeClient.PHOTO_LOCATION_REQUEST && media != null) media.photoPermissionsReturned();
    }
}
