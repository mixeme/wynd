package mixeme.wynd;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;

import org.json.JSONException;
import org.json.JSONObject;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Редакция F-Droid: обновления только из магазина приложений.
 *  API обязан совпадать с {@code src/standard/java/mixeme/wynd/UpdateController.java}:
 *  общие AndroidBridge и MainActivity используют класс без флейворных веток,
 *  и расхождение сигнатур ломает сборку одной из редакций на компиляции. */
final class UpdateController {
    private static UpdateController instance;
    final ExecutorService worker = Executors.newSingleThreadExecutor();

    static synchronized UpdateController get(Context context) {
        if (instance == null) instance = new UpdateController();
        return instance;
    }

    private UpdateController() { }

    synchronized JSONObject status() throws JSONException {
        return new JSONObject().put("version", BuildConfig.APP_VERSION).put("debug", BuildConfig.DEBUG)
                .put("development", false).put("protocol", 1)
                .put("capabilities", new JSONObject().put("updates", false).put("developmentUpdateBypass", false))
                .put("immediately", false).put("state", "idle").put("available", false)
                .put("nextVersion", "").put("ready", false).put("error", "").put("blocked", false)
                .put("installing", false).put("permission", false);
    }

    synchronized void setImmediately(boolean value) {
        throw new IllegalStateException("Updates unavailable in this build");
    }

    void check() {
        throw new IllegalStateException("Updates unavailable in this build");
    }

    void prepare() {
        throw new IllegalStateException("Updates unavailable in this build");
    }

    synchronized void failure(Exception exception) { /* Состояния обновления в редакции нет. */ }

    void install(Activity activity) { /* Магазин обновляет сам; страница видит updates:false. */ }

    /** Common InstallResultReceiver остаётся в main и ждёт этот no-op в обеих редакциях. */
    synchronized void installationResult(int sessionId, int status, Intent next) { }

    synchronized void resume(Activity activity) { }

    synchronized void pause(Activity activity) { }
}
