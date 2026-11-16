package mixeme.wynd;

import android.Manifest;
import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import org.json.JSONArray;

/** Уведомление не открывает стороннее приложение без отдельного нажатия пользователя. */
final class SavedFiles {
    private static final String CHANNEL = "saved-files";

    @SuppressWarnings("deprecation")
    static boolean notify(Activity activity, Uri uri, String name, String mime) {
        if (Build.VERSION.SDK_INT >= 33 && activity.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) return false;
        NotificationManager manager = (NotificationManager) activity.getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager == null || Build.VERSION.SDK_INT >= 24 && !manager.areNotificationsEnabled()) return false;
        if (Build.VERSION.SDK_INT >= 26) {
            manager.createNotificationChannel(new NotificationChannel(CHANNEL, "Сохранённые файлы", NotificationManager.IMPORTANCE_LOW));
            if (manager.getNotificationChannel(CHANNEL).getImportance() == NotificationManager.IMPORTANCE_NONE) return false;
        }
        remember(activity, manager, uri);
        int id = uri.toString().hashCode();
        Intent open = new Intent(activity, OpenExportActivity.class).setDataAndType(uri, mime).putExtra("name", name);
        PendingIntent action = PendingIntent.getActivity(activity, id, open, PendingIntent.FLAG_UPDATE_CURRENT
                | (Build.VERSION.SDK_INT >= 23 ? PendingIntent.FLAG_IMMUTABLE : 0));
        Notification.Builder builder = Build.VERSION.SDK_INT >= 26
                ? new Notification.Builder(activity, CHANNEL) : new Notification.Builder(activity);
        manager.notify(id, builder.setSmallIcon(R.drawable.wynd_notification).setContentTitle("Файл сохранён")
                .setContentText(name).setContentIntent(action).setAutoCancel(true)
                .setVisibility(Notification.VISIBILITY_PRIVATE)
                .addAction(R.drawable.wynd_notification, "Открыть в…", action).build());
        return true;
    }

    private static void remember(Activity activity, NotificationManager manager, Uri uri) {
        // Только доступ к выбранным документам; ограничить число постоянных grants.
        try { activity.getContentResolver().takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION); }
        catch (SecurityException ignored) { /* У такого provider доступ может закончиться вместе с задачей. */ }
        SharedPreferences preferences = activity.getSharedPreferences("saved-files", Context.MODE_PRIVATE);
        try {
            JSONArray previous = new JSONArray(preferences.getString("uris", "[]")), next = new JSONArray();
            for (int i = 0; i < previous.length(); i++) {
                String old = previous.getString(i);
                if (!old.equals(uri.toString())) next.put(old);
            }
            next.put(uri.toString());
            while (next.length() > 16) {
                String old = next.getString(0);
                JSONArray trimmed = new JSONArray();
                for (int i = 1; i < next.length(); i++) trimmed.put(next.getString(i));
                next = trimmed;
                manager.cancel(old.hashCode());
                try { activity.getContentResolver().releasePersistableUriPermission(Uri.parse(old), Intent.FLAG_GRANT_READ_URI_PERMISSION); }
                catch (SecurityException ignored) { }
            }
            preferences.edit().putString("uris", next.toString()).apply();
        } catch (Exception ignored) { /* Повреждённая история не мешает сохранить файл. */ }
    }
}
