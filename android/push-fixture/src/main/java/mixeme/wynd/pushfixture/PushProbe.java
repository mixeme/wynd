package mixeme.wynd.pushfixture;

import android.app.Activity;
import android.app.Instrumentation;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.util.Base64;

import org.json.JSONObject;
import org.unifiedpush.android.connector.UnifiedPush;
import org.unifiedpush.android.connector.data.PublicKeySet;
import org.unifiedpush.android.connector.keys.DefaultKeyManager;

import java.nio.charset.StandardCharsets;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import javax.crypto.AEADBadTagException;

/** Two synthetic registrations with the library's real on-device keys. */
public final class PushProbe extends Instrumentation {
    private static final String FIRST = "first";
    private static final String SECOND = "second";
    private Bundle arguments;

    @Override public void onCreate(Bundle arguments) {
        super.onCreate(arguments);
        this.arguments = arguments;
        start();
    }

    @Override public void onStart() {
        Bundle result = new Bundle();
        try {
            Context context = getTargetContext();
            DefaultKeyManager manager = new DefaultKeyManager(context);
            switch (arguments.getString("step", "")) {
                case "keys":
                    UnifiedPush.saveDistributor(context, context.getPackageName());
                    UnifiedPush.register(context, FIRST, null, null);
                    UnifiedPush.register(context, SECOND, null, null);
                    if (!FixtureDistributor.registrations.await(8, TimeUnit.SECONDS)) {
                        throw new IllegalStateException("Did not receive two separate registrations");
                    }
                    JSONObject keys = new JSONObject();
                    for (String instance : new String[]{FIRST, SECOND}) {
                        PublicKeySet set = manager.getPublicKeySet(instance);
                        if (set == null) throw new IllegalStateException("Missing connector keys");
                        keys.put(instance, new JSONObject().put("p256dh", set.getPubKey()).put("auth", set.getAuth()));
                    }
                    result.putString("keys", keys.toString());
                    break;
                case "verify":
                    verify(manager, FIRST, SECOND, "comment", "fixture-post");
                    verify(manager, SECOND, FIRST, "mention", "fixture-other-post");
                    result.putString("verified", "two_server_signals_wrong_key_and_tampering_rejected");
                    break;
                default:
                    throw new IllegalArgumentException("Unknown probe step");
            }
            finish(Activity.RESULT_OK, result);
        } catch (Exception error) {
            result.putString("error", error.getClass().getSimpleName() + ": " + error.getMessage());
            finish(Activity.RESULT_CANCELED, result);
        }
    }

    private void verify(DefaultKeyManager manager, String instance, String other, String type, String post)
            throws Exception {
        byte[] sealed = Base64.decode(arguments.getString(instance, ""), Base64.URL_SAFE);
        byte[] clear = manager.decrypt(instance, sealed);
        if (clear == null) throw new IllegalStateException("Signal could not be decrypted for " + instance);
        JSONObject signal = new JSONObject(new String(clear, StandardCharsets.UTF_8));
        if (!type.equals(signal.getString("type")) || !post.equals(signal.getString("post_id"))
                || !instance.equals(signal.getString("circle_id")) || signal.getInt("count") != 1) {
            throw new IllegalStateException("Unexpected signal for " + instance);
        }
        reject(manager, other, sealed);
        sealed[sealed.length - 1] ^= 1;
        reject(manager, instance, sealed);
    }

    private static void reject(DefaultKeyManager manager, String instance, byte[] sealed) throws Exception {
        try {
            if (manager.decrypt(instance, sealed) != null) {
                throw new IllegalStateException("Unauthenticated signal decrypted");
            }
        } catch (Exception error) {
            // Kotlin can propagate this checked Android cipher exception without a Java throws clause.
            if (!(error instanceof AEADBadTagException)) throw error;
        }
    }

    /** Only records register intents from the connector; no network or real distributor account. */
    public static final class FixtureDistributor extends BroadcastReceiver {
        private static final CountDownLatch registrations = new CountDownLatch(2);
        private static final Set<String> tokens = new HashSet<>();

        @Override public void onReceive(Context context, Intent intent) {
            if (!"org.unifiedpush.android.distributor.REGISTER".equals(intent.getAction())) return;
            String token = intent.getStringExtra("token");
            if (token != null && !token.isEmpty() && tokens.add(token)) registrations.countDown();
        }
    }
}
