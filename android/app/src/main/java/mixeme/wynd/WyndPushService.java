package mixeme.wynd;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.service.notification.StatusBarNotification;

import org.json.JSONException;
import org.json.JSONObject;
import org.unifiedpush.android.connector.FailedReason;
import org.unifiedpush.android.connector.PushService;
import org.unifiedpush.android.connector.data.PushEndpoint;
import org.unifiedpush.android.connector.data.PushMessage;

import java.nio.charset.StandardCharsets;
import java.util.regex.Pattern;

/** Receives authenticated, decrypted signals from the UnifiedPush connector. */
public final class WyndPushService extends PushService {
    private static final String CHANNEL = "journal";
    private static final String OPEN = "mixeme.wynd.OPEN_PUSH";
    private static final Pattern ID = Pattern.compile("[A-Za-z0-9_-]{1,128}");

    @Override public void onNewEndpoint(PushEndpoint endpoint, String instance) {
        try {
            if (new PushRegistry(this).update(instance, endpoint)) PushBridge.changed();
        } catch (JSONException ignored) { /* An incomplete subscription is never published. */ }
    }

    @Override public void onRegistrationFailed(FailedReason reason, String instance) {
        new PushRegistry(this).failed(instance, false);
        PushBridge.changed();
    }

    @Override public void onUnregistered(String instance) {
        new PushRegistry(this).failed(instance, true);
        PushBridge.changed();
    }

    @Override public void onTempUnavailable(String instance) {
        new PushRegistry(this).failed(instance, false);
    }

    @android.annotation.SuppressLint("MissingPermission") // PushRegistry checks the current system permission.
    @Override public void onMessage(PushMessage message, String instance) {
        PushRegistry registry = new PushRegistry(this);
        JSONObject registration = registry.get(instance);
        if (!message.getDecrypted() || message.getContent().length > 4096 || registration == null
                || !registration.has("subscription") || !registry.enabled() || !registry.allowed()) return;

        JSONObject signal;
        try { signal = new JSONObject(new String(message.getContent(), StandardCharsets.UTF_8)); }
        catch (JSONException ignored) { return; }

        String circle = id(signal.optString("circle_id"));
        String post = id(signal.optString("post_id"));
        String type = id(signal.optString("type"));
        String tag = instance + ":" + circle + ":" + type;
        Intent open = new Intent(this, MainActivity.class).setAction(OPEN)
                .setData(Uri.fromParts("wynd-push", tag, null))
                .putExtra("instance", instance).putExtra("circle", circle).putExtra("post", post);
        PendingIntent click = PendingIntent.getActivity(this, 0, open,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        NotificationManager manager = (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        if (manager == null) return;
        Notification.Builder builder;
        if (Build.VERSION.SDK_INT >= 26) {
            manager.createNotificationChannel(new NotificationChannel(CHANNEL, "Журнал Wynd",
                    NotificationManager.IMPORTANCE_DEFAULT));
            builder = new Notification.Builder(this, CHANNEL);
        } else {
            builder = new Notification.Builder(this).setDefaults(Notification.DEFAULT_ALL);
        }
        String title = signal.optString("title");
        String body = signal.optString("body");
        try {
            manager.notify(tag, 1, builder.setSmallIcon(R.drawable.wynd_notification)
                    .setContentTitle(title.isEmpty() ? "Wynd" : title)
                    .setContentText(body.isEmpty() ? bodyFor(type) : body)
                    .setContentIntent(click).setAutoCancel(true).build());
        } catch (SecurityException ignored) { /* Permission may have changed while receiving the message. */ }
    }

    static boolean isOpen(Intent intent) { return intent != null && OPEN.equals(intent.getAction()); }

    static String route(Context context, Intent intent, String origin) {
        if (!isOpen(intent)) return null;
        PushRegistry registry = new PushRegistry(context);
        String instance = id(intent.getStringExtra("instance"));
        if (instance.isEmpty() || !registry.source().equals(origin) || registry.get(instance) == null) return null;
        String circle = id(intent.getStringExtra("circle"));
        String post = id(intent.getStringExtra("post"));
        return circle.isEmpty() ? "" : "circles/" + circle + (post.isEmpty() ? "" : "/posts/" + post);
    }

    static void cancel(Context context, String instance) {
        if (Build.VERSION.SDK_INT < 23) return;
        NotificationManager manager = (NotificationManager) context.getSystemService(NOTIFICATION_SERVICE);
        if (manager == null) return;
        for (StatusBarNotification note : manager.getActiveNotifications()) {
            String tag = note.getTag();
            if (tag != null && tag.startsWith(instance + ":")) manager.cancel(tag, note.getId());
        }
    }

    private static String id(String value) { return value != null && ID.matcher(value).matches() ? value : ""; }

    // Same wording as web/static/push-sw.js.
    private static String bodyFor(String type) {
        return switch (type) {
            case "post" -> "Новая запись в круге";
            case "comment" -> "Новый комментарий";
            case "reaction" -> "Новая реакция";
            case "mention" -> "Вас упомянули";
            case "invite" -> "Вас позвали в круг";
            case "event" -> "В круге изменения";
            default -> "Новое в Wynd";
        };
    }
}
