package mixeme.wynd;

import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.ClipData;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.pm.ProviderInfo;
import android.content.res.AssetFileDescriptor;
import android.net.Uri;
import android.os.Build;
import android.os.Process;
import android.webkit.PermissionRequest;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebView;
import java.io.IOException;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Системные вопросы только по запросу страницы, без разрешений на всё хранилище. */
final class MediaChromeClient extends WebChromeClient {
    static final int FILE_REQUEST = 4101;
    static final int PERMISSION_REQUEST = 4102;
    static final int PHOTO_LOCATION_REQUEST = 4103;
    private final Activity activity;
    private final WebView view;
    private final ServerOrigin server;
    private final ExecutorService files = Executors.newSingleThreadExecutor();
    private PermissionRequest media;
    private ValueCallback<Uri[]> selection;
    private boolean selectionMultiple;
    private boolean resumed;
    private boolean askingPermissions;
    private boolean fileInFlight;
    private boolean destroyed;
    private OriginalPhotos originals;
    private boolean selectionOriginals, askingPhotoLocation;
    private List<Uri> photoSelection;
    private boolean photoLocationAsked;
    private String selectionPhotoNonce;
    private boolean photoLocationNeeded;

    MediaChromeClient(Activity activity, WebView view, ServerOrigin server) {
        this.activity = activity;
        this.view = view;
        this.server = server;
    }

    private boolean trusted() {
        return !destroyed && !activity.isFinishing() && server.contains(view.getUrl());
    }

    private boolean has(String permission) {
        return activity.checkPermission(permission, Process.myPid(), Process.myUid()) == PackageManager.PERMISSION_GRANTED;
    }

    void originalPhotos(OriginalPhotos originals) { this.originals = originals; }
    boolean busy() { return media != null || selection != null || fileInFlight || askingPermissions || askingPhotoLocation; }

    @Override public void onPermissionRequest(PermissionRequest request) {
        String[] required = MediaPolicy.permissions(request.getResources());
        if (!resumed || !trusted() || !server.contains(request.getOrigin().toString())
                || required.length == 0 || media != null || askingPermissions || selection != null || askingPhotoLocation) {
            request.deny(); return;
        }
        media = request;
        requestPermissions(required);
    }

    private void requestPermissions(String[] required) {
        List<String> missing = new ArrayList<>();
        for (String permission : required) if (!has(permission)) missing.add(permission);
        if (!missing.isEmpty() && Build.VERSION.SDK_INT >= 23) {
            askingPermissions = true;
            try { activity.requestPermissions(missing.toArray(new String[0]), PERMISSION_REQUEST); }
            catch (RuntimeException e) { askingPermissions = false; cancel(); }
        } else completePermission();
    }

    void permissionsReturned() {
        askingPermissions = false;
        completePermission();
    }

    private void completePermission() {
        // Android может прислать результат до onResume после своего диалога.
        if (!resumed || askingPermissions) return;
        if (media != null) {
            PermissionRequest request = media;
            media = null;
            String[] required = MediaPolicy.permissions(request.getResources());
            boolean granted = trusted() && server.contains(request.getOrigin().toString()) && required.length > 0;
            for (String permission : required) granted &= has(permission);
            if (granted) {
                // Список ограничен CAMERA/RECORD_AUDIO, а не произвольными ресурсами страницы.
                List<String> resources = new ArrayList<>();
                for (String permission : required) resources.add(permission.equals(MediaPolicy.CAMERA) ? MediaPolicy.VIDEO : MediaPolicy.AUDIO);
                request.grant(resources.toArray(new String[0]));
            } else request.deny();
        }
    }

    @Override public void onPermissionRequestCanceled(PermissionRequest request) {
        if (media == request) media = null;
    }

    @Override public boolean onShowFileChooser(WebView webView, ValueCallback<Uri[]> callback, FileChooserParams parameters) {
        if (!resumed || !trusted() || selection != null || fileInFlight || askingPermissions || askingPhotoLocation || media != null
                || parameters.getMode() != FileChooserParams.MODE_OPEN && parameters.getMode() != FileChooserParams.MODE_OPEN_MULTIPLE) {
            callback.onReceiveValue(null); return true;
        }
        selection = callback;
        photoLocationAsked = false;
        photoLocationNeeded = false;
        selectionMultiple = parameters.getMode() == FileChooserParams.MODE_OPEN_MULTIPLE;
        String[] accepted = parameters.getAcceptTypes();
        List<String> regular = new ArrayList<>();
        String nonce = null;
        for (String value : accepted) for (String part : value.split(",")) {
            String type = part.trim();
            if (type.startsWith(PhotoPolicy.ACCEPT_PREFIX)) {
                if (nonce != null) { finishSelection(null); return true; }
                nonce = type.substring(PhotoPolicy.ACCEPT_PREFIX.length());
            } else regular.add(type);
        }
        selectionOriginals = nonce != null && originals != null && originals.consume(nonce);
        selectionPhotoNonce = selectionOriginals ? nonce : null;
        if (nonce != null && !selectionOriginals) { finishSelection(null); return true; }
        String[] types = MediaPolicy.mimeTypes(regular.toArray(new String[0]));
        if (selectionOriginals) for (String type : types) if (!PhotoPolicy.mediaMime(type) && !"*/*".equals(type)) { finishSelection(null); return true; }
        // Живые камера/микрофон — getUserMedia. Здесь базовый выбор файлов:
        // внешняя камера OPPO выгружала процесс с незавершённым input callback.
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT).addCategory(Intent.CATEGORY_OPENABLE)
                .setType(types.length == 1 ? types[0] : "*/*")
                .putExtra(Intent.EXTRA_ALLOW_MULTIPLE, selectionMultiple);
        if (types.length > 1) intent.putExtra(Intent.EXTRA_MIME_TYPES, types);
        launch(intent);
        return true;
    }

    @SuppressWarnings("deprecation")
    private void launch(Intent intent) {
        try {
            fileInFlight = true;
            activity.startActivityForResult(intent, FILE_REQUEST);
        } catch (ActivityNotFoundException | SecurityException e) {
            fileInFlight = false;
            finishSelection(null);
        }
    }

    void filesReturned(int resultCode, Intent data) {
        fileInFlight = false;
        if (selection == null || !trusted() || resultCode != Activity.RESULT_OK) { finishSelection(null); return; }
        LinkedHashSet<Uri> candidates = new LinkedHashSet<>();
        if (data != null && data.getClipData() != null) {
            ClipData clip = data.getClipData();
            if (clip.getItemCount() > 256) { finishSelection(null); return; }
            for (int i = 0; i < clip.getItemCount(); i++) candidates.add(clip.getItemAt(i).getUri());
        } else if (data != null && data.getData() != null) candidates.add(data.getData());
        if (!selectionMultiple && candidates.size() > 1) { finishSelection(null); return; }
        ValueCallback<Uri[]> expected = selection;
        boolean wantsOriginals = selectionOriginals;
        files.execute(() -> {
            List<Uri> readable = new ArrayList<>();
            boolean needsLocation = false;
            try {
                for (Uri uri : candidates) {
                    if (!readable(uri)) { readable.clear(); break; }
                    readable.add(uri);
                    if (wantsOriginals && originals != null) needsLocation |= originals.needsLocation(uri);
                }
            } catch (RuntimeException ignored) { readable.clear(); }
            boolean localPhotoLocation = needsLocation;
            activity.runOnUiThread(() -> {
                if (destroyed || selection != expected) return;
                if (!trusted() || readable.isEmpty()) { finishSelection(null); return; }
                if (selectionOriginals) { photoLocationNeeded = localPhotoLocation; photoSelection = readable; completePhotoSelection(); }
                else finishSelection(readable.toArray(new Uri[0]));
            });
        });
    }

    private void completePhotoSelection() {
        if (!resumed || askingPhotoLocation || photoSelection == null || selection == null || !trusted()) return;
        List<Uri> selected = photoSelection;
        ValueCallback<Uri[]> expected = selection;
        if (!photoLocationAsked && photoLocationNeeded && originals != null && Build.VERSION.SDK_INT >= 29 && !originals.hasOriginalAccess()) {
            photoLocationAsked = true;
            askingPhotoLocation = true;
            try { activity.requestPermissions(originals.permissions(), PHOTO_LOCATION_REQUEST); }
            catch (RuntimeException e) { askingPhotoLocation = false; finishSelection(null); }
            return;
        }
        photoSelection = null;
        String scope = originals == null ? "" : originals.scope();
        files.execute(() -> {
            List<Uri> result = new ArrayList<>();
            for (Uri uri : selected) result.add(originals == null ? uri : originals.readableOriginal(uri, scope));
            activity.runOnUiThread(() -> {
                if (!destroyed && selection == expected) finishSelection(trusted() ? result.toArray(new Uri[0]) : null);
            });
        });
    }

    void photoPermissionsReturned() { askingPhotoLocation = false; completePhotoSelection(); }

    private boolean readable(Uri uri) {
        if (uri == null || !MediaPolicy.selectable(uri.toString(), activity.getPackageName() + ".files")) return false;
        try {
            ProviderInfo provider = activity.getPackageManager().resolveContentProvider(uri.getAuthority(), 0);
            if (provider == null || provider.applicationInfo.uid == Process.myUid()) return false;
            try (AssetFileDescriptor descriptor = activity.getContentResolver().openAssetFileDescriptor(uri, "r")) {
                return descriptor != null;
            }
        } catch (IOException | RuntimeException e) { return false; }
    }

    private void finishSelection(Uri[] result) {
        ValueCallback<Uri[]> callback = selection;
        if (originals != null && selectionPhotoNonce != null) originals.record(selectionPhotoNonce,
                result != null && (!photoLocationNeeded || originals.hasOriginalAccess()));
        selection = null;
        selectionPhotoNonce = null;
        photoSelection = null;
        selectionOriginals = false;
        if (callback != null) callback.onReceiveValue(result);
    }

    void cancel() {
        if (media != null) { PermissionRequest request = media; media = null; request.deny(); }
        finishSelection(null);
    }

    void resume() { resumed = true; completePermission(); completePhotoSelection(); }
    void pause() { resumed = false; }
    void destroy() { destroyed = true; cancel(); files.shutdownNow(); }
}
