package mixeme.wynd;

import android.Manifest;
import android.app.Activity;
import android.content.pm.PackageManager;
import android.content.Intent;
import android.content.ContentUris;
import android.content.SharedPreferences;
import android.content.res.AssetFileDescriptor;
import android.database.Cursor;
import android.net.Uri;
import android.os.Build;
import android.os.SystemClock;
import android.provider.MediaStore;
import android.provider.DocumentsContract;
import android.provider.OpenableColumns;
import android.webkit.WebView;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONObject;
import java.io.IOException;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.UUID;

/** Одноразовое намерение главного документа; input остаётся обычным File API WebView. */
final class OriginalPhotos {
    private static final String PICKER_MODE = "picker-mode";
    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private volatile String owner = UUID.randomUUID().toString();
    private final PhotoRequestNonce intent = new PhotoRequestNonce();
    private final LinkedHashMap<String, Boolean> decisions = new LinkedHashMap<>();
    private boolean resumed;

    OriginalPhotos(Activity activity, WebView web, ServerOrigin origin, MediaChromeClient media) {
        this.activity = activity; this.web = web; this.origin = origin;
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        SharedPreferences preferences = activity.getSharedPreferences("photo-location", Activity.MODE_PRIVATE);
        WebViewCompat.addWebMessageListener(web, "__wyndPhotoLocation",
                Collections.singleton(origin.url().substring(0, origin.url().length() - 1)),
                (view, message, source, main, reply) -> {
                    if (!main || !resumed || !origin.contains(source.toString())
                            || !origin.contains(web.getUrl()) || activity.isFinishing() || activity.isDestroyed()
                            || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 128) return;
                    try {
                        JSONObject request = new JSONObject(raw);
                        int id = request.getInt("id"); if (id < 1) return;
                        String operation = request.optString("op", "arm");
                        if (operation.equals("result")) {
                            Boolean allowed = decisions.remove(request.optString("token"));
                            JSONObject response = new JSONObject().put("id", id).put("protocol", 1);
                            if (allowed == null) response.put("error", "expired"); else response.put("allowed", allowed);
                            reply.postMessage(response.toString()); return;
                        }
                        if (operation.equals("picker")) {
                            boolean offerChoice = media.offersPhotoPickerChoice();
                            JSONObject response = new JSONObject().put("id", id).put("protocol", 1)
                                    .put("offerChoice", offerChoice);
                            String mode = request.optString("mode");
                            if (offerChoice && (mode.equals("documents") || mode.equals("gallery"))) {
                                preferences.edit().putString(PICKER_MODE, mode).apply();
                            }
                            if (offerChoice) response.put("mode", preferences.getString(PICKER_MODE, ""));
                            reply.postMessage(response.toString()); return;
                        }
                        if (!operation.equals("arm") || media.busy()) {
                            reply.postMessage(new JSONObject().put("id", id).put("protocol", 1).put("error", "busy").toString()); return;
                        }
                        String token = intent.issue(SystemClock.elapsedRealtime());
                        reply.postMessage(new JSONObject().put("id", id).put("protocol", 1).put("token", token).toString());
                    } catch (Exception ignored) { }
                });
    }

    boolean consume(String token) {
        return resumed && intent.consume(token, SystemClock.elapsedRealtime());
    }

    void record(String token, boolean allowed) {
        if (!PhotoPolicy.token(token)) return;
        decisions.put(token, allowed);
        while (decisions.size() > 16) decisions.remove(decisions.keySet().iterator().next());
    }

    boolean needsLocation(Uri uri) {
        return Build.VERSION.SDK_INT >= 29 && PhotoPolicy.localMedia(uri.getAuthority())
                && ExportPolicy.mime(activity.getContentResolver().getType(uri)).startsWith("image/");
    }
    boolean hasLocation() {
        return Build.VERSION.SDK_INT >= 29 && activity.checkSelfPermission(Manifest.permission.ACCESS_MEDIA_LOCATION) == PackageManager.PERMISSION_GRANTED;
    }
    boolean hasOriginalAccess() {
        if (Build.VERSION.SDK_INT < 29) return false;
        return hasLocation() && (Build.VERSION.SDK_INT > 30
                || activity.checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED);
    }
    @androidx.annotation.RequiresApi(29)
    String[] permissions() {
        return Build.VERSION.SDK_INT <= 30
                ? new String[] {Manifest.permission.ACCESS_MEDIA_LOCATION, Manifest.permission.READ_EXTERNAL_STORAGE}
                : new String[] {Manifest.permission.ACCESS_MEDIA_LOCATION};
    }

    String scope() { return owner; }
    Uri readableOriginal(Uri selected, String scope) {
        String stage = "map";
        try {
            if (!scope.equals(owner) || Build.VERSION.SDK_INT < 29 || !hasOriginalAccess()
                    || !ExportPolicy.mime(activity.getContentResolver().getType(selected)).startsWith("image/")) return selected;
            Uri media = mediaUri(selected);
            if (media == null || !"media".equals(media.getAuthority())) {
                if (BuildConfig.DEBUG) android.util.Log.i("WyndAndroid", "Selected photo has no MediaStore equivalent");
                return selected;
            }
            Uri requested = MediaStore.setRequireOriginal(media);
            stage = "open";
            long size;
            try (AssetFileDescriptor descriptor = activity.getContentResolver().openAssetFileDescriptor(requested, "r")) {
                if (descriptor == null) return selected;
                size = descriptor.getLength();
            }
            String name = "photo";
            try (Cursor cursor = activity.getContentResolver().query(selected, new String[] {OpenableColumns.DISPLAY_NAME}, null, null, null)) {
                if (cursor != null && cursor.moveToFirst() && !cursor.isNull(0)) name = ExportPolicy.filename(cursor.getString(0));
            }
            if (!scope.equals(owner)) return selected;
            String token = SelectedMediaProvider.REGISTRY.register(new SelectedMediaRegistry.Entry(scope, requested.toString(), name,
                    ExportPolicy.mime(activity.getContentResolver().getType(selected)), size));
            if (!scope.equals(owner)) { SelectedMediaProvider.REGISTRY.clear(scope); return selected; }
            if (BuildConfig.DEBUG) android.util.Log.i("WyndAndroid", "Selected original photo source prepared");
            return new Uri.Builder().scheme("content").authority(activity.getPackageName() + ".selected-media").appendPath(token).build();
        } catch (IOException | RuntimeException e) {
            if (BuildConfig.DEBUG) android.util.Log.i("WyndAndroid", "Original photo source unavailable at " + stage + ": " + e.getClass().getSimpleName());
            return selected;
        }
    }

    @androidx.annotation.RequiresApi(29)
    private Uri mediaUri(Uri selected) {
        if ("media".equals(selected.getAuthority())) return selected;
        // В 10–11 getMediaUri ещё не поддерживает MediaDocumentsProvider.
        // У этого системного provider image:<id> обозначает тот же Images URI.
        // Чтение разрешается только уже выданным legacy read + media-location.
        if (Build.VERSION.SDK_INT <= 30 && "com.android.providers.media.documents".equals(selected.getAuthority())) {
            String document = DocumentsContract.getDocumentId(selected);
            if (document.startsWith("image:")) {
                long id = Long.parseLong(document.substring(6));
                if (id > 0) return ContentUris.withAppendedId(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, id);
            }
        }
        boolean acquired = false;
        try {
            boolean held = false;
            for (android.content.UriPermission permission : activity.getContentResolver().getPersistedUriPermissions())
                held |= permission.getUri().equals(selected) && permission.isReadPermission();
            if (!held) {
                // Android 10 передаёт getMediaUri только persisted grants. Не удерживать
                // новое право после преобразования и не освобождать чужое прежнее право.
                activity.getContentResolver().takePersistableUriPermission(selected, Intent.FLAG_GRANT_READ_URI_PERMISSION);
                acquired = true;
            }
            return MediaStore.getMediaUri(activity, selected);
        } finally {
            if (acquired) activity.getContentResolver().releasePersistableUriPermission(selected, Intent.FLAG_GRANT_READ_URI_PERMISSION);
        }
    }

    void clear() { intent.clear(); decisions.clear(); String previous = owner; owner = UUID.randomUUID().toString(); SelectedMediaProvider.REGISTRY.clear(previous); }
    void resume() { resumed = true; }
    void pause() { resumed = false; }
}
