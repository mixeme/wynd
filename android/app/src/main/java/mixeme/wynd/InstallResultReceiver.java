package mixeme.wynd;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageInstaller;
import android.util.Log;

public final class InstallResultReceiver extends BroadcastReceiver {
    @Override
    @SuppressWarnings("deprecation")
    public void onReceive(Context context, Intent intent) {
        if (!(context.getPackageName() + ".INSTALL_RESULT").equals(intent.getAction())) return;
        int status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE);
        int sessionId = intent.getIntExtra("wyndSessionId", -1);
        if (BuildConfig.DEBUG) Log.i("WyndAndroid", "PackageInstaller status=" + status
                + " session=" + sessionId + " message=" + intent.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE));
        Intent next = intent.getParcelableExtra(Intent.EXTRA_INTENT);
        UpdateController.get(context).installationResult(sessionId, status, next);
    }
}
