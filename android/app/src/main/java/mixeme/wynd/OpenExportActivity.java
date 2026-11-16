package mixeme.wynd;

import android.app.Activity;
import android.app.NotificationManager;
import android.content.ActivityNotFoundException;
import android.content.ClipData;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.widget.Toast;

/** Только явное нажатие PendingIntent своего уведомления; Activity не экспортируется. */
public final class OpenExportActivity extends Activity {
    @Override protected void onCreate(Bundle state) {
        super.onCreate(state);
        Uri uri = getIntent().getData();
        try {
            if (uri == null || !MediaPolicy.selectable(uri.toString(), getPackageName() + ".files")) throw new SecurityException();
            String name = ExportPolicy.filename(getIntent().getStringExtra("name"));
            Intent open = new Intent(Intent.ACTION_VIEW).setDataAndType(uri, ExportPolicy.mime(getIntent().getType()))
                    .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
            open.setClipData(ClipData.newRawUri(name, uri));
            startActivity(Intent.createChooser(open, "Открыть в…"));
            NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (manager != null) manager.cancel(uri.toString().hashCode());
        } catch (ActivityNotFoundException | SecurityException e) {
            Toast.makeText(this, "Откройте сохранённый файл в приложении «Файлы»", Toast.LENGTH_LONG).show();
        }
        finish();
    }
}
