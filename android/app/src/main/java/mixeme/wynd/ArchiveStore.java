package mixeme.wynd;

import android.content.ContentResolver;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.res.AssetFileDescriptor;
import android.database.Cursor;
import android.net.Uri;
import android.provider.OpenableColumns;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;

import org.json.JSONObject;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.UUID;

/**
 * Помнит выбранный файл `.wynd` (план 50, 5.4): постоянное право чтения URI
 * и токен для отдачи файла странице через тот же origin. Файл не копируется;
 * после перезапуска процесса страница просит его снова.
 */
final class ArchiveStore {
    private static final String PREFS = "wynd-archive";
    private static final String KEY_ORIGIN = "origin";
    private static final String KEY_URI = "uri";
    private static final String KEY_NAME = "name";
    private static final String KEY_SIZE = "size";
    private static final String KEY_TOKEN = "token";
    private static final String PREFIX = "/__wynd_archive/";

    private ArchiveStore() {
    }

    /** Запоминает первый выбранный .wynd; остальные файлы выбора не трогает. */
    static synchronized boolean remember(Context context, ServerOrigin origin, List<Uri> uris) {
        ContentResolver resolver = context.getContentResolver();
        for (Uri uri : uris) {
            String name = displayName(context, uri);
            String mime = resolver.getType(uri);
            if (!isWynd(name, mime)) continue;
            try {
                resolver.takePersistableUriPermission(
                        uri, Intent.FLAG_GRANT_READ_URI_PERMISSION);
            } catch (SecurityException ignored) {
                // Провайдер не дал постоянного права; текущее чтение может работать.
            }
            if (!openable(context, uri)) continue;
            long size = size(context, uri);
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
                    .putString(KEY_ORIGIN, origin.url())
                    .putString(KEY_URI, uri.toString())
                    .putString(KEY_NAME, name)
                    .putLong(KEY_SIZE, size)
                    .putString(KEY_TOKEN, UUID.randomUUID().toString())
                    .apply();
            return true;
        }
        return false;
    }

    static synchronized JSONObject status(Context context, ServerOrigin origin) {
        Stored stored = load(context, origin);
        if (stored == null) return null;
        try {
            return new JSONObject().put("name", stored.name).put("size", stored.size);
        } catch (Exception error) {
            return null;
        }
    }

    static synchronized JSONObject open(Context context, ServerOrigin origin) {
        Stored stored = load(context, origin);
        if (stored == null) return null;
        try {
            return new JSONObject()
                    .put("url", PREFIX + stored.token)
                    .put("name", stored.name)
                    .put("size", stored.size);
        } catch (Exception error) {
            return null;
        }
    }

    static synchronized void forget(Context context) {
        SharedPreferences prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
        String raw = prefs.getString(KEY_URI, "");
        if (!raw.isEmpty()) {
            try {
                context.getContentResolver().releasePersistableUriPermission(
                        Uri.parse(raw), Intent.FLAG_GRANT_READ_URI_PERMISSION);
            } catch (SecurityException ignored) {
                // Права уже нет.
            }
        }
        prefs.edit().clear().apply();
    }

    /** Отдаёт файл странице по одноразовому пути в origin выбранного сервера. */
    static WebResourceResponse intercept(Context context, ServerOrigin origin,
                                         WebResourceRequest request) {
        String method = request.getMethod();
        if (!"GET".equals(method) && !"HEAD".equals(method)) return null;
        String url = request.getUrl().toString();
        if (!origin.contains(url)) return null;
        String path = request.getUrl().getPath();
        if (path == null || !path.startsWith(PREFIX)) return null;
        String token = path.substring(PREFIX.length());
        Stored stored = load(context, origin);
        if (stored == null || !stored.token.equals(token)) return null;
        try {
            long length;
            try (AssetFileDescriptor descriptor =
                         context.getContentResolver().openAssetFileDescriptor(stored.uri, "r")) {
                if (descriptor == null) return gone();
                length = descriptor.getLength();
            }
            InputStream stream = "HEAD".equals(method)
                    ? new ByteArrayInputStream(new byte[0])
                    : context.getContentResolver().openInputStream(stored.uri);
            if (stream == null) return gone();
            Map<String, String> headers = new HashMap<>();
            headers.put("Content-Type", "application/octet-stream");
            headers.put("Cache-Control", "no-store");
            headers.put("X-Content-Type-Options", "nosniff");
            if (length >= 0) headers.put("Content-Length", Long.toString(length));
            return new WebResourceResponse("application/octet-stream", null, 200, "OK",
                    headers, stream);
        } catch (IOException | SecurityException error) {
            return gone();
        }
    }

    private static WebResourceResponse gone() {
        return new WebResourceResponse("text/plain", "UTF-8", 410, "Gone",
                Collections.emptyMap(),
                new ByteArrayInputStream("archive unavailable".getBytes(StandardCharsets.UTF_8)));
    }

    private static Stored load(Context context, ServerOrigin origin) {
        SharedPreferences prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
        String storedOrigin = prefs.getString(KEY_ORIGIN, "");
        String raw = prefs.getString(KEY_URI, "");
        String token = prefs.getString(KEY_TOKEN, "");
        if (!origin.url().equals(storedOrigin) || raw.isEmpty() || token.isEmpty()) return null;
        Uri uri = Uri.parse(raw);
        if (!openable(context, uri)) return null;
        return new Stored(uri, prefs.getString(KEY_NAME, "archive.wynd"),
                prefs.getLong(KEY_SIZE, 0), token);
    }

    private static boolean openable(Context context, Uri uri) {
        try (AssetFileDescriptor descriptor =
                     context.getContentResolver().openAssetFileDescriptor(uri, "r")) {
            return descriptor != null;
        } catch (IOException | SecurityException error) {
            return false;
        }
    }

    private static String displayName(Context context, Uri uri) {
        try (Cursor cursor = context.getContentResolver().query(uri,
                new String[]{OpenableColumns.DISPLAY_NAME}, null, null, null)) {
            if (cursor != null && cursor.moveToFirst()) {
                String value = cursor.getString(0);
                if (value != null && !value.isEmpty()) return value;
            }
        } catch (RuntimeException ignored) {
            // Имя не обязательно; ниже — запасное.
        }
        String last = uri.getLastPathSegment();
        return last == null ? "archive.wynd" : last;
    }

    private static long size(Context context, Uri uri) {
        try (Cursor cursor = context.getContentResolver().query(uri,
                new String[]{OpenableColumns.SIZE}, null, null, null)) {
            if (cursor != null && cursor.moveToFirst() && !cursor.isNull(0)) {
                return cursor.getLong(0);
            }
        } catch (RuntimeException ignored) {
            // Размер узнает AssetFileDescriptor при отдаче.
        }
        return 0;
    }

    /** .wynd по имени или MIME; другие документы не трогаем. */
    static boolean isWynd(String name, String mime) {
        if (name != null && name.toLowerCase(Locale.ROOT).endsWith(".wynd")) return true;
        return mime != null && mime.toLowerCase(Locale.ROOT).contains("wynd");
    }

    private static final class Stored {
        final Uri uri;
        final String name;
        final long size;
        final String token;

        Stored(Uri uri, String name, long size, String token) {
            this.uri = uri;
            this.name = name;
            this.size = size;
            this.token = token;
        }
    }
}
