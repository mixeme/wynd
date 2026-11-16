package mixeme.wynd.sharefixture;

import android.app.Activity;
import android.content.ClipData;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import java.util.ArrayList;

/** Только собственные синтетические assets. Не читает общую галерею. */
public final class Sender extends Activity {
    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        String phase = getIntent().getStringExtra("phase");
        if ("resolve".equals(phase)) {
            try {
                android.content.IntentFilter filter = new android.content.IntentFilter(Intent.ACTION_SEND_MULTIPLE);
                filter.addDataType("image/*"); filter.addDataType("video/*");
                int match = filter.matchData("*/*", null, null);
                int receivers = getPackageManager().queryIntentActivities(new Intent(Intent.ACTION_SEND_MULTIPLE)
                        .setType("*/*").setPackage("mixeme.wynd.debug"), android.content.pm.PackageManager.MATCH_DEFAULT_ONLY).size();
                android.util.Log.i("WyndShareFixture", "narrowMixedMatch=" + match + ", receivers=" + receivers);
            } catch (Exception e) { throw new IllegalStateException(e); }
            finish(); return;
        }
        Uri photo = Uri.parse("content://mixeme.wynd.sharefixture.media/photo");
        Intent send = new Intent("multi".equals(phase) ? Intent.ACTION_SEND_MULTIPLE : Intent.ACTION_SEND)
                .setClassName("mixeme.wynd.debug", "mixeme.wynd.MainActivity");
        if ("large-text".equals(phase)) {
            StringBuilder text = new StringBuilder(); for (int i = 0; i < 16385; i++) text.append('я');
            send.setType("text/plain").putExtra(Intent.EXTRA_TEXT, text.toString());
        } else if ("text".equals(phase)) {
            send.setType("text/plain").putExtra(Intent.EXTRA_TEXT, "Тест «Поделиться» <b>без HTML</b>\nhttps://example.org/fixture");
        } else {
            send.setType("image/jpeg").putExtra(Intent.EXTRA_STREAM, photo);
            if (!"no-grant".equals(phase)) {
                send.setClipData(ClipData.newUri(getContentResolver(), "fixture", photo));
                send.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
            } else send.setClipData(ClipData.newPlainText("fixture", "no URI grants"));
            if ("multi".equals(phase)) {
                Uri video = Uri.parse("content://mixeme.wynd.sharefixture.media/video");
                ArrayList<Uri> streams = new ArrayList<>(); streams.add(photo); streams.add(video);
                send.setType("*/*").putParcelableArrayListExtra(Intent.EXTRA_STREAM, streams);
                send.getClipData().addItem(new ClipData.Item(video));
            }
        }
        startActivity(send); finish();
    }
}
