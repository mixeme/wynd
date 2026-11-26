package mixeme.wynd;

import android.app.Activity;
import android.app.Application;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageInstaller;
import android.net.Uri;
import android.os.Build;
import android.provider.Settings;

import org.json.JSONException;
import org.json.JSONObject;

import java.io.File;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.OutputStream;
import java.lang.ref.WeakReference;
import java.util.Locale;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Жизнь загрузки не привязана к Activity; веб-страница не задаёт URL или путь APK. */
final class UpdateController {
    private static UpdateController instance;
    final ExecutorService worker = Executors.newSingleThreadExecutor();
    private final Application context;
    private final SharedPreferences preferences;
    private final UpdateTransport transport;
    private final File apk;
    private volatile ReleaseCatalog.Release release;
    private volatile String error = "";
    private volatile String state = "idle";
    private volatile boolean ready;
    private volatile boolean installing;
    private boolean awaitingPermission;
    private volatile boolean signatureBlocked;
    private volatile int activeSessionId;
    private WeakReference<Activity> foreground = new WeakReference<>(null);
    private Intent confirmation;

    static synchronized UpdateController get(Context context) {
        if (instance == null) instance = new UpdateController((Application) context.getApplicationContext());
        return instance;
    }

    private UpdateController(Application context) {
        this.context = context;
        preferences = context.getSharedPreferences("android-updates", Context.MODE_PRIVATE);
        activeSessionId = preferences.getInt("activeSessionId", -1);
        transport = new UpdateTransport(BuildConfig.DEBUG, BuildConfig.UPDATE_FIXTURE_ORIGIN);
        apk = new File(context.getCacheDir(), "wynd-update.apk");
    }

    synchronized JSONObject status() throws JSONException {
        revokeUnavailableBypass();
        return new JSONObject().put("version", BuildConfig.APP_VERSION).put("debug", BuildConfig.DEBUG)
                .put("development", developmentEnabled())
                .put("protocol", 1).put("capabilities", new JSONObject().put("updates", true)
                        // Обходить нечего, пока выдержка снята: переключатель в
                        // «Настройках» не показывается (вернётся с 72 часами при 1.0.0).
                        .put("developmentUpdateBypass",
                                developmentEnabled() && UpdatePolicy.DELAY_MILLIS > 0))
                .put("immediately", developmentEnabled() && preferences.getBoolean("immediately", false))
                .put("state", state).put("available", release != null)
                .put("nextVersion", release == null ? "" : release.version)
                .put("ready", ready).put("error", error).put("blocked", signatureBlocked)
                .put("installing", installing).put("permission", awaitingPermission);
    }

    synchronized void setImmediately(boolean value) {
        if (!developmentEnabled()) throw new IllegalArgumentException("Development only");
        if (installing) return;
        preferences.edit().putBoolean("immediately", value).apply();
        // После выключения обхода ранее выбранный молодой выпуск не остаётся доступным.
        release = null;
        ready = false;
        error = "";
        state = "idle";
    }

    void check() throws IOException, JSONException {
        if (installing) return;
        state = "checking";
        error = "";
        ready = false;
        ReleaseCatalog.Release best = null;
        String base = BuildConfig.DEBUG && !BuildConfig.UPDATE_FIXTURE_ORIGIN.isEmpty()
                ? BuildConfig.UPDATE_FIXTURE_ORIGIN + "/repos/mixeme/wynd/releases" : UpdatePolicy.RELEASES_API;
        boolean exhausted = false;
        for (int page = 1; page <= 20; page++) {
            String json = transport.text(base + "?per_page=100&page=" + page, 4 << 20);
            ReleaseCatalog.Release candidate = ReleaseCatalog.select(json, BuildConfig.VERSION_CODE,
                    transport.responseTimeMillis(), BuildConfig.DEBUG, developmentEnabled(),
                    preferences.getBoolean("immediately", false), BuildConfig.UPDATE_FIXTURE_ORIGIN);
            if (candidate != null && (best == null || candidate.code > best.code)) best = candidate;
            if (new org.json.JSONArray(json).length() < 100) { exhausted = true; break; }
        }
        if (!exhausted) throw new IOException("Release pagination limit");
        if (release == null || best == null || release.code != best.code) ready = false;
        release = best;
        state = best == null ? "latest" : "available";
        if (signatureBlocked) error = "signature";
    }

    private boolean developmentEnabled() {
        if (BuildConfig.DEBUG) return true;
        try {
            return Settings.Global.getInt(context.getContentResolver(),
                    Settings.Global.DEVELOPMENT_SETTINGS_ENABLED, 0) == 1;
        } catch (SecurityException e) {
            return false;
        }
    }

    private synchronized void revokeUnavailableBypass() {
        if (!installing && release != null && release.requiresBypass
                && !(developmentEnabled() && preferences.getBoolean("immediately", false))) {
            release = null;
            ready = false;
            state = "idle";
        }
    }

    void prepare() throws IOException {
        revokeUnavailableBypass();
        if (installing || signatureBlocked) return;
        if (release == null) throw new IOException("No update selected");
        ready = false;
        state = "downloading";
        error = "";
        String expected = transport.text(release.checksumUrl, 256).trim().toLowerCase(Locale.ROOT);
        if (!expected.matches("[a-f0-9]{64}")) throw new ApkVerifier.Rejected("checksum");
        transport.download(release.apkUrl, apk, 200L << 20);
        if (!ApkVerifier.sha256(apk).equals(expected)) throw new ApkVerifier.Rejected("checksum");
        ApkVerifier.verify(context, apk, release);
        ready = true;
        state = "ready";
    }

    synchronized void failure(Exception exception) {
        error = exception instanceof ApkVerifier.Rejected ? ((ApkVerifier.Rejected) exception).code : "network";
        signatureBlocked |= error.equals("signature") || error.equals("checksum") || error.equals("package") || error.equals("version");
        ready = false;
        installing = false;
        state = "failed";
        if (apk.exists()) apk.delete();
    }

    void install(Activity activity) {
        synchronized (this) {
            revokeUnavailableBypass();
            if (!ready || release == null || installing || signatureBlocked) return;
            error = "";
            if (Build.VERSION.SDK_INT >= 26 && !context.getPackageManager().canRequestPackageInstalls()) {
                awaitingPermission = true;
                state = "permission";
                activity.startActivity(new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                        Uri.parse("package:" + context.getPackageName())));
                return;
            }
            awaitingPermission = false;
            installing = true;
            state = "installing";
        }
        worker.execute(() -> {
            try { commitSession(); }
            catch (IOException | RuntimeException e) { synchronized (this) { failure(e); } }
        });
    }

    private void commitSession() throws IOException {
        ApkVerifier.verify(context, apk, release);
        PackageInstaller installer = context.getPackageManager().getPackageInstaller();
        PackageInstaller.SessionParams parameters = new PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL);
        parameters.setAppPackageName(context.getPackageName());
        parameters.setSize(apk.length());
        if (Build.VERSION.SDK_INT >= 31) parameters.setRequireUserAction(PackageInstaller.SessionParams.USER_ACTION_REQUIRED);
        int sessionId = installer.createSession(parameters);
        boolean committed = false;
        try (PackageInstaller.Session session = installer.openSession(sessionId)) {
            activeSessionId = sessionId;
            if (!preferences.edit().putInt("activeSessionId", sessionId).commit()) throw new IOException("Could not persist install session");
            try (FileInputStream input = new FileInputStream(apk);
                 OutputStream output = session.openWrite("base.apk", 0, apk.length())) {
                byte[] buffer = new byte[32768];
                int read;
                while ((read = input.read(buffer)) != -1) output.write(buffer, 0, read);
                session.fsync(output);
            }
            Intent result = new Intent(context, InstallResultReceiver.class)
                    .setAction(context.getPackageName() + ".INSTALL_RESULT").putExtra("wyndSessionId", sessionId);
            int flags = PendingIntent.FLAG_UPDATE_CURRENT;
            if (Build.VERSION.SDK_INT >= 31) flags |= PendingIntent.FLAG_MUTABLE;
            PendingIntent callback = PendingIntent.getBroadcast(context, sessionId, result, flags);
            session.commit(callback.getIntentSender());
            committed = true;
        } finally {
            if (!committed) {
                installer.abandonSession(sessionId);
                activeSessionId = -1;
                preferences.edit().remove("activeSessionId").apply();
            }
        }
    }

    synchronized void resume(Activity activity) {
        foreground = new WeakReference<>(activity);
        if (confirmation != null) {
            Intent intent = confirmation;
            confirmation = null;
            activity.startActivity(intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
        } else if (awaitingPermission && Build.VERSION.SDK_INT >= 26
                && context.getPackageManager().canRequestPackageInstalls()) install(activity);
    }

    synchronized void pause(Activity activity) {
        if (foreground.get() == activity) foreground.clear();
    }

    synchronized void installationResult(int sessionId, int status, Intent next) {
        // Запоздавший ответ прежней попытки не отменяет новый сеанс.
        // -1 допускается только при переходе со старого каркаса без ID.
        if (sessionId >= 0 ? sessionId != activeSessionId : activeSessionId >= 0) return;
        if (status == PackageInstaller.STATUS_PENDING_USER_ACTION && next != null) {
            Activity activity = foreground.get();
            if (activity != null) activity.startActivity(next.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
            else confirmation = next;
        } else if (status == PackageInstaller.STATUS_SUCCESS) {
            installing = false;
            ready = false;
            release = null;
            state = "latest";
            activeSessionId = -1;
            preferences.edit().remove("activeSessionId").apply();
            if (apk.exists()) apk.delete();
        } else {
            installing = false;
            state = "ready";
            // STATUS_FAILURE_ABORTED бывает и при отказе системной проверки,
            // а не только при нажатии человеком «Отмена».
            error = "install";
            activeSessionId = -1;
            preferences.edit().remove("activeSessionId").apply();
        }
    }
}
