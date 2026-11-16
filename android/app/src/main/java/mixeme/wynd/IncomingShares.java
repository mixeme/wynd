package mixeme.wynd;

import android.app.Activity;
import android.content.ClipData;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.pm.ProviderInfo;
import android.database.Cursor;
import android.net.Uri;
import android.os.Process;
import android.provider.OpenableColumns;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebView;
import android.widget.Toast;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.LinkedHashSet;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Один входящий набор; private-копии сохраняют его после утраты временного grant. */
final class IncomingShares {
    static final String PATH = "/.wynd-android-share/";
    private static final Object LOCK = new Object();
    private static final ExecutorService WORKER = Executors.newSingleThreadExecutor();
    private static volatile JSONObject pending;
    private static volatile boolean preparing;
    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final File directory;
    private boolean resumed, destroyed;
    private boolean closingFromBack;

    IncomingShares(Activity activity, WebView web, ServerOrigin origin) {
        this.activity = activity; this.web = web; this.origin = origin;
        directory = new File(activity.getFilesDir(), "incoming-share");
        WORKER.execute(this::restore);
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        WebViewCompat.addWebMessageListener(web, "__wyndIncomingShare",
                Collections.singleton(origin.url().substring(0, origin.url().length() - 1)),
                (view, message, source, main, reply) -> {
                    if (!main || !resumed || destroyed || activity.isFinishing() || activity.isDestroyed()
                            || !origin.contains(source.toString()) || !origin.contains(web.getUrl())
                            || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData(); if (raw == null || raw.length() > 192) return;
                    try {
                        JSONObject request = new JSONObject(raw);
                        int id = request.getInt("id"); if (id < 1) return;
                        String op = request.getString("op");
                        if (op.equals("peek")) {
                            reply.postMessage(new JSONObject().put("id", id).put("protocol", 1)
                                    .put("preparing", preparing).put("pending", pending == null ? JSONObject.NULL : pending).toString());
                        } else if (op.equals("finish") && PhotoPolicy.token(request.optString("token"))) {
                            String token = request.getString("token");
                            WORKER.execute(() -> {
                                boolean removed;
                                synchronized (LOCK) {
                                    removed = pending != null && token.equals(pending.optString("token"));
                                    if (removed) { pending = null; erase(); }
                                }
                                activity.runOnUiThread(() -> {
                                    if (destroyed || !origin.contains(web.getUrl())) return;
                                    try { reply.postMessage(new JSONObject().put("id", id).put("protocol", 1).put("finished", removed).toString()); }
                                    catch (Exception ignored) { }
                                });
                            });
                        }
                    } catch (Exception ignored) { }
                });
    }

    @SuppressWarnings("deprecation")
    void accept(Intent intent) {
        if (intent == null || !Intent.ACTION_SEND.equals(intent.getAction()) && !Intent.ACTION_SEND_MULTIPLE.equals(intent.getAction())) return;
        if (preparing) { note("Дождитесь получения предыдущих файлов"); return; }
        preparing = true;
        WORKER.execute(() -> {
            try {
                if (pending != null) { note("Сначала выберите круг для полученного или отмените его"); return; }
                CharSequence extraText = intent.getCharSequenceExtra(Intent.EXTRA_TEXT);
                String text = extraText == null ? "" : extraText.toString();
                if (!SharePolicy.text(text)) throw new IOException("text");
                LinkedHashSet<Uri> sources = new LinkedHashSet<>();
                if (Intent.ACTION_SEND_MULTIPLE.equals(intent.getAction())) {
                    java.util.ArrayList<?> list = intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);
                    if (list != null) {
                        if (list.size() > SharePolicy.MAX_FILES) throw new IOException("count");
                        for (Object item : list) { if (!(item instanceof Uri)) throw new IOException("uri"); sources.add((Uri) item); }
                    }
                } else {
                    Object item = intent.getParcelableExtra(Intent.EXTRA_STREAM);
                    if (item != null) { if (!(item instanceof Uri)) throw new IOException("uri"); sources.add((Uri) item); }
                }
                if (sources.isEmpty() && intent.getClipData() != null) {
                    ClipData clip = intent.getClipData();
                    if (clip.getItemCount() > SharePolicy.MAX_FILES) throw new IOException("count");
                    for (int i = 0; i < clip.getItemCount(); i++) {
                        ClipData.Item item = clip.getItemAt(i);
                        if (item.getIntent() != null) throw new IOException("intent");
                        if (item.getUri() != null) sources.add(item.getUri());
                    }
                }
                if (sources.size() > SharePolicy.MAX_FILES || sources.isEmpty() && text.isEmpty()) throw new IOException("empty");
                erase();
                if (!directory.isDirectory() && !directory.mkdirs()) throw new IOException("directory");
                String token = UUID.randomUUID().toString();
                JSONArray files = new JSONArray(); long total = 0;
                for (Uri source : sources) {
                    if (!SharePolicy.uri(source.toString()) || source.getAuthority() == null
                            || activity.checkUriPermission(source, Process.myPid(), Process.myUid(), Intent.FLAG_GRANT_READ_URI_PERMISSION) != PackageManager.PERMISSION_GRANTED)
                        throw new IOException("grant");
                    ProviderInfo provider = activity.getPackageManager().resolveContentProvider(source.getAuthority(), 0);
                    if (provider == null || provider.applicationInfo.uid == Process.myUid()) throw new IOException("provider");
                    String mime = ExportPolicy.mime(activity.getContentResolver().getType(source));
                    if (!SharePolicy.media(mime)) throw new IOException("type");
                    String name = "shared-" + files.length();
                    try (Cursor cursor = activity.getContentResolver().query(source, new String[] {OpenableColumns.DISPLAY_NAME}, null, null, null)) {
                        if (cursor != null && cursor.moveToFirst() && !cursor.isNull(0)) name = ExportPolicy.filename(cursor.getString(0));
                    }
                    long size = 0;
                    try (InputStream input = activity.getContentResolver().openInputStream(source);
                         FileOutputStream out = new FileOutputStream(new File(directory, Integer.toString(files.length())))) {
                        if (input == null) throw new IOException("read");
                        byte[] buffer = new byte[32768]; int n;
                        while ((n = input.read(buffer)) != -1) {
                            size += n; total += n;
                            if (total > SharePolicy.MAX_BYTES) throw new IOException("size");
                            out.write(buffer, 0, n);
                        }
                    }
                    files.put(new JSONObject().put("name", name).put("type", mime).put("size", size)
                            .put("path", PATH + token + "/" + files.length()));
                }
                JSONObject record = new JSONObject().put("token", token).put("text", text).put("files", files).put("origin", origin.url());
                try (FileOutputStream out = new FileOutputStream(new File(directory, "pending.tmp"))) {
                    out.write(record.toString().getBytes(StandardCharsets.UTF_8)); out.getFD().sync();
                }
                if (!new File(directory, "pending.tmp").renameTo(new File(directory, "pending.json"))) throw new IOException("commit");
                pending = record;
                note("Получено для новой записи. Выберите круг после входа; открытую запись можно сначала завершить");
            } catch (Exception e) {
                erase();
                note("Не удалось получить материал. Можно передать до 32 фото и видео, всего до 512 МиБ, и текст до 32 КиБ");
            } finally { preparing = false; }
        });
    }

    boolean busyForInterface() {
        return preparing || pending != null || new File(directory, "pending.json").exists();
    }

    private void restore() {
        synchronized (LOCK) {
            try {
                File metadata = new File(directory, "pending.json");
                if (!metadata.isFile() || metadata.length() > 262144) { pending = null; erase(); return; }
                byte[] bytes = new byte[(int) metadata.length()];
                try (FileInputStream input = new FileInputStream(metadata)) {
                    int offset = 0, n; while (offset < bytes.length && (n = input.read(bytes, offset, bytes.length - offset)) > 0) offset += n;
                    if (offset != bytes.length) throw new IOException("metadata");
                }
                JSONObject record = new JSONObject(new String(bytes, StandardCharsets.UTF_8));
                if (!PhotoPolicy.token(record.getString("token")) || !SharePolicy.text(record.getString("text")) || !origin.url().equals(record.getString("origin"))) throw new IOException("metadata");
                JSONArray files = record.getJSONArray("files"); long total = 0;
                if (files.length() > SharePolicy.MAX_FILES) throw new IOException("count");
                for (int i = 0; i < files.length(); i++) {
                    JSONObject item = files.getJSONObject(i); File file = new File(directory, Integer.toString(i));
                    if (!SharePolicy.media(item.getString("type")) || !file.isFile() || file.length() != item.getLong("size")
                            || !item.getString("path").equals(PATH + record.getString("token") + "/" + i)) throw new IOException("metadata");
                    total += file.length();
                }
                if (total > SharePolicy.MAX_BYTES) throw new IOException("size");
                pending = record;
            } catch (Exception ignored) { pending = null; erase(); }
        }
    }

    WebResourceResponse intercept(WebResourceRequest request) {
        Uri uri = request.getUrl();
        if (!origin.contains(uri.toString()) || uri.getPath() == null || !uri.getPath().startsWith(PATH)) return null;
        synchronized (LOCK) {
            try {
                JSONObject record = pending;
                if (request.isForMainFrame() || !"GET".equals(request.getMethod()) || uri.getQuery() != null || uri.getFragment() != null || record == null) throw new IOException("request");
                JSONArray files = record.getJSONArray("files");
                for (int i = 0; i < files.length(); i++) {
                    JSONObject item = files.getJSONObject(i);
                    if (item.getString("path").equals(uri.getPath())) {
                        java.util.Map<String, String> headers = new java.util.HashMap<>();
                        headers.put("Cache-Control", "no-store"); headers.put("X-Content-Type-Options", "nosniff");
                        return new WebResourceResponse(item.getString("type"), null, 200, "OK", headers, new FileInputStream(new File(directory, Integer.toString(i))));
                    }
                }
            } catch (Exception ignored) { }
        }
        return new WebResourceResponse("text/plain", "UTF-8", 404, "Not Found", Collections.singletonMap("Cache-Control", "no-store"), new ByteArrayInputStream(new byte[0]));
    }

    private void erase() {
        // Только фиксированные собственные имена, не пути или имена отправителя.
        for (int i = 0; i < SharePolicy.MAX_FILES; i++) new File(directory, Integer.toString(i)).delete();
        new File(directory, "pending.tmp").delete(); new File(directory, "pending.json").delete();
    }
    boolean cancelFromBack(Runnable navigate) {
        if (closingFromBack) return true;
        JSONObject record = pending;
        if (record == null || !SharePolicy.choicePage(web.getUrl(), origin)) return false;
        String token = record.optString("token");
        closingFromBack = true;
        WORKER.execute(() -> {
            synchronized (LOCK) {
                if (pending != null && token.equals(pending.optString("token"))) { pending = null; erase(); }
            }
            activity.runOnUiThread(() -> {
                closingFromBack = false;
                if (!destroyed) navigate.run();
            });
        });
        return true;
    }
    private void note(String message) { activity.runOnUiThread(() -> { if (!destroyed) Toast.makeText(activity, message, Toast.LENGTH_LONG).show(); }); }
    void resume() { resumed = true; }
    void pause() { resumed = false; }
    void destroy() { destroyed = true; }
}
