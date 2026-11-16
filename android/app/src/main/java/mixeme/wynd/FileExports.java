package mixeme.wynd;

import android.app.Activity;
import android.Manifest;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.content.pm.ProviderInfo;
import android.net.Uri;
import android.os.Process;
import android.content.pm.PackageManager;
import android.database.Cursor;
import android.os.Build;
import android.provider.OpenableColumns;
import android.util.Base64;
import android.webkit.WebView;
import android.widget.Toast;
import androidx.webkit.JavaScriptReplyProxy;
import androidx.webkit.WebMessageCompat;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.json.JSONObject;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Collections;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Только выбранный пользователем документ; без FileProvider и доступа ко всему хранилищу. */
final class FileExports {
    static final int SAVE_REQUEST = 7103;
    static final int NOTIFICATION_REQUEST = 7104;
    private final Activity activity;
    private final WebView web;
    private final ServerOrigin origin;
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private Job current;
    private Job pickerJob;
    private boolean destroyed, resumed;
    private Job toOpen;

    private static final class Job {
        final String token = UUID.randomUUID().toString();
        final JavaScriptReplyProxy reply;
        final int beginId;
        final String name, mime, url;
        String savedName;
        final long length;
        volatile boolean cancelled;
        boolean writing;
        Uri uri;
        OutputStream stream; // Только worker.
        long written; // Только worker.
        int sequence;
        Job(JSONObject request, JavaScriptReplyProxy reply) throws Exception {
            this.reply = reply;
            beginId = request.getInt("id");
            name = ExportPolicy.filename(request.optString("name"));
            savedName = name;
            mime = ExportPolicy.mime(request.optString("mime"));
            url = request.optString("url", "");
            length = url.isEmpty() ? request.getLong("size") : -1;
            if (beginId <= 0 || length < -1 || url.isEmpty() && length < 0) throw new IllegalArgumentException();
        }
    }

    FileExports(Activity activity, WebView web, ServerOrigin origin) {
        this.activity = activity;
        this.web = web;
        this.origin = origin;
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        WebViewCompat.addWebMessageListener(web, "__wyndFiles",
                Collections.singleton(origin.url().substring(0, origin.url().length() - 1)),
                (view, message, source, main, reply) -> {
                    if (!main || !origin.contains(source.toString()) || !trusted()
                            || message.getType() != WebMessageCompat.TYPE_STRING) return;
                    String raw = message.getData();
                    if (raw == null || raw.length() > 46000) return;
                    int id = 0;
                    try {
                        JSONObject request = new JSONObject(raw);
                        id = request.getInt("id");
                        if (id <= 0) return;
                        String operation = request.getString("op");
                        if (operation.equals("begin")) begin(request, reply);
                        else {
                            Job job = current;
                            if (job == null || !job.token.equals(request.optString("token"))) throw new IllegalArgumentException();
                            if (operation.equals("cancel")) { cancel(); respond(reply, id, "cancelled", ""); }
                            else write(job, request, id);
                        }
                    } catch (Exception e) { if (id > 0) respond(reply, id, "error", "invalid_request"); }
                });
    }

    private boolean trusted() {
        return !destroyed && !activity.isFinishing() && !activity.isDestroyed() && origin.contains(web.getUrl());
    }

    @SuppressWarnings("deprecation")
    private void begin(JSONObject request, JavaScriptReplyProxy reply) throws Exception {
        if (current != null || pickerJob != null || !resumed) { respond(reply, request.getInt("id"), "error", "busy"); return; }
        Job job = new Job(request, reply);
        if (!job.url.isEmpty() && !ExportPolicy.remote(job.url, origin, BuildConfig.DEBUG)) throw new IllegalArgumentException();
        current = job;
        pickerJob = job;
        try {
            activity.startActivityForResult(new Intent(Intent.ACTION_CREATE_DOCUMENT)
                    .addCategory(Intent.CATEGORY_OPENABLE).setType(job.mime)
                    .putExtra(Intent.EXTRA_TITLE, job.name)
                    .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION
                            | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION), SAVE_REQUEST);
        } catch (ActivityNotFoundException | SecurityException e) {
            current = null;
            pickerJob = null;
            respond(reply, job.beginId, "error", "no_file_picker");
        }
    }

    void returned(int resultCode, Intent data) {
        Job job = pickerJob;
        pickerJob = null;
        if (job == null) return;
        if (job.cancelled || current != job) {
            Uri abandoned = data == null ? null : data.getData();
            if (resultCode == Activity.RESULT_OK && safeDestination(abandoned)) {
                job.uri = abandoned; worker.execute(() -> discard(job));
            }
            return;
        }
        if (resultCode != Activity.RESULT_OK) {
            current = null; respond(job.reply, job.beginId, "cancelled", ""); return;
        }
        Uri uri = data == null ? null : data.getData();
        if (!trusted() || !safeDestination(uri)) {
            current = null;
            if (safeDestination(uri)) { job.uri = uri; worker.execute(() -> discard(job)); }
            respond(job.reply, job.beginId, "error", "invalid_destination"); return;
        }
        job.uri = uri;
        worker.execute(() -> {
            try {
                if (job.cancelled) return;
                job.stream = activity.getContentResolver().openOutputStream(uri, "wt");
                if (job.stream == null) throw new IOException();
                if (job.url.isEmpty()) activity.runOnUiThread(() -> {
                    if (current == job && trusted()) respond(job.reply, job.beginId, "ready", job.token);
                });
                else { download(job); complete(job, job.beginId); }
            } catch (Exception e) { fail(job, job.beginId); }
        });
    }

    private boolean safeDestination(Uri uri) {
        if (uri == null || !MediaPolicy.selectable(uri.toString(), activity.getPackageName() + ".files")) return false;
        try {
            ProviderInfo provider = activity.getPackageManager().resolveContentProvider(uri.getAuthority(), 0);
            return provider != null && provider.applicationInfo.uid != Process.myUid();
        } catch (RuntimeException e) { return false; }
    }

    private void write(Job job, JSONObject request, int id) throws Exception {
        if (!job.url.isEmpty() || job.uri == null || job.writing || request.getInt("sequence") != job.sequence)
            throw new IllegalArgumentException();
        String operation = request.getString("op");
        if (!operation.equals("chunk") && !operation.equals("end")) throw new IllegalArgumentException();
        byte[] bytes = operation.equals("chunk") ? Base64.decode(request.getString("data"), Base64.NO_WRAP) : null;
        if (bytes != null && (bytes.length == 0 || bytes.length > ExportPolicy.CHUNK_BYTES)) throw new IllegalArgumentException();
        job.writing = true;
        job.sequence++;
        worker.execute(() -> {
            try {
                if (job.cancelled || job.stream == null) throw new IOException();
                if (bytes == null) {
                    if (job.written != job.length) throw new IOException();
                    complete(job, id);
                } else {
                    if (job.written + bytes.length > job.length) throw new IOException();
                    job.stream.write(bytes); job.written += bytes.length;
                    activity.runOnUiThread(() -> {
                        job.writing = false;
                        if (current == job && trusted()) respond(job.reply, id, "ready", job.token);
                    });
                }
            } catch (Exception e) { fail(job, id); }
        });
    }

    private void download(Job job) throws Exception {
        String address = job.url;
        for (int redirects = 0; redirects <= 5; redirects++) {
            if (job.cancelled || !ExportPolicy.remote(address, origin, BuildConfig.DEBUG)) throw new IOException();
            HttpURLConnection connection = (HttpURLConnection) new URL(address).openConnection();
            connection.setInstanceFollowRedirects(false);
            connection.setConnectTimeout(30000); connection.setReadTimeout(30000);
            connection.setRequestProperty("Accept-Encoding", "identity");
            try {
                int status = connection.getResponseCode();
                if (status >= 300 && status <= 399) {
                    String next = connection.getHeaderField("Location");
                    if (next == null) throw new IOException();
                    String resolved = new URL(new URL(address), next).toExternalForm();
                    if (connection.getURL().getProtocol().equalsIgnoreCase("https")
                            && !new URL(resolved).getProtocol().equalsIgnoreCase("https")) throw new IOException();
                    address = resolved; continue;
                }
                if (status != 200) throw new IOException();
                long expected = -1;
                String header = connection.getHeaderField("Content-Length");
                if (header != null) { expected = Long.parseLong(header); if (expected < 0) throw new IOException(); }
                try (InputStream input = connection.getInputStream()) {
                    byte[] buffer = new byte[ExportPolicy.CHUNK_BYTES];
                    int read;
                    while ((read = input.read(buffer)) != -1) {
                        if (job.cancelled) throw new IOException();
                        job.stream.write(buffer, 0, read); job.written += read;
                    }
                }
                if (expected >= 0 && expected != job.written) throw new IOException();
                return;
            } finally { connection.disconnect(); }
        }
        throw new IOException();
    }

    private void complete(Job job, int id) throws IOException {
        job.stream.flush(); job.stream.close(); job.stream = null;
        if (job.cancelled) { discard(job); return; }
        try (Cursor cursor = activity.getContentResolver().query(job.uri,
                new String[] {OpenableColumns.DISPLAY_NAME}, null, null, null)) {
            if (cursor != null && cursor.moveToFirst() && !cursor.isNull(0)) job.savedName = ExportPolicy.filename(cursor.getString(0));
        } catch (RuntimeException ignored) { /* Имя из запроса остаётся запасным. */ }
        activity.runOnUiThread(() -> {
            if (current != job || !trusted()) return;
            current = null;
            respond(job.reply, id, "saved", job.token);
            toOpen = job;
            notifySaved();
        });
    }

    private void notifySaved() {
        if (!resumed || toOpen == null || !trusted()) return;
        if (Build.VERSION.SDK_INT >= 33 && activity.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            android.content.SharedPreferences settings = activity.getSharedPreferences("saved-files", Activity.MODE_PRIVATE);
            if (!settings.getBoolean("asked", false)) {
                settings.edit().putBoolean("asked", true).apply();
                activity.requestPermissions(new String[] {Manifest.permission.POST_NOTIFICATIONS}, NOTIFICATION_REQUEST);
                return;
            }
        }
        Job job = toOpen; toOpen = null;
        if (!SavedFiles.notify(activity, job.uri, job.savedName, job.mime)) Toast.makeText(activity, "Файл сохранён", Toast.LENGTH_SHORT).show();
    }

    private void fail(Job job, int id) {
        job.cancelled = true; discard(job);
        activity.runOnUiThread(() -> {
            if (current != job) return;
            current = null;
            respond(job.reply, id, "error", "write_failed");
            if (trusted()) Toast.makeText(activity, "Не удалось сохранить файл полностью", Toast.LENGTH_LONG).show();
        });
    }

    private void discard(Job job) {
        try { if (job.stream != null) job.stream.close(); } catch (IOException ignored) { }
        job.stream = null;
        // Не удалять URI: provider вправе вернуть уже существующий документ.
        // При обрыве в выбранном месте может остаться неполный файл.
    }

    private void respond(JavaScriptReplyProxy reply, int id, String state, String detail) {
        if (destroyed || !WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)) return;
        try { reply.postMessage(new JSONObject().put("id", id).put("state", state).put("detail", detail).toString()); }
        catch (Exception ignored) { /* Исходный документ мог уйти. */ }
    }

    void cancel() {
        Job job = current; current = null;
        if (job != null) {
            job.cancelled = true;
            respond(job.reply, job.beginId, "cancelled", "");
            worker.execute(() -> discard(job));
        }
        toOpen = null;
    }
    boolean busy() { return current != null || pickerJob != null; }
    void resume() { resumed = true; notifySaved(); }
    void notificationsReturned() { notifySaved(); }
    void pause() { resumed = false; }
    void destroy() { cancel(); destroyed = true; worker.shutdown(); }
}
