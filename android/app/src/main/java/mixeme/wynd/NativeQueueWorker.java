package mixeme.wynd;

import android.content.Context;
import android.util.Log;

import androidx.annotation.NonNull;
import androidx.annotation.OptIn;
import androidx.media3.common.util.UnstableApi;
import androidx.work.BackoffPolicy;
import androidx.work.Constraints;
import androidx.work.ExistingWorkPolicy;
import androidx.work.NetworkType;
import androidx.work.OneTimeWorkRequest;
import androidx.work.WorkManager;
import androidx.work.Worker;
import androidx.work.WorkerParameters;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.File;
import java.util.concurrent.TimeUnit;

/**
 * Фоновый прогон нативной очереди (план 50, этап 5.1). WorkManager будит
 * приложение после закрытия окна; записи и токены уже лежат в private-файле,
 * поэтому страница для отправки не нужна.
 */
public final class NativeQueueWorker extends Worker {
    private static final String TAG = "WyndAndroid";
    private static final String UNIQUE = "wynd-native-queue";

    public NativeQueueWorker(@NonNull Context context, @NonNull WorkerParameters parameters) {
        super(context, parameters);
    }

    @NonNull
    @Override
    public Result doWork() {
        NativeQueue queue = NativeQueue.get(getApplicationContext());
        NativeQueueSender.Parts parts = new NativeQueueSender.Parts() {
            @Override
            public File file(NativeQueue.Record record, NativeQueue.Part part) {
                return queue.partFile(record.id, part);
            }

            @Override
            public void saved(NativeQueue.Record record, NativeQueue.Part part) {
                queue.setPart(record.id, part);
            }
        };
        boolean retry = false;
        for (NativeQueue.Record record : queue.records()) {
            if (isStopped()) return Result.retry();
            compressVideos(queue, record);
            ensurePosters(queue, record);
            NativeQueueSender.Outcome outcome =
                    NativeQueueSender.send(record, NativeQueueSender.HttpTransport.INSTANCE, parts);
            switch (outcome.state) {
                case "sent":
                    queue.markSent(record);
                    break;
                case "unauthorized":
                    queue.markUnauthorized(record);
                    break;
                case "failed":
                    queue.markFailed(record, outcome.error);
                    break;
                default:
                    queue.retry(record);
                    retry = true;
                    break;
            }
        }
        return retry ? Result.retry() : Result.success();
    }

    /** 5.3: офлайн-видео сжимается здесь, до первой попытки загрузки. */
    @OptIn(markerClass = UnstableApi.class)
    private void compressVideos(NativeQueue queue, NativeQueue.Record record) {
        if (!record.compress || record.files.isEmpty()) return;
        JSONArray media;
        try {
            media = new JSONObject(record.payload).optJSONArray("media_meta");
        } catch (Exception error) {
            return;
        }
        for (int i = 0; i < record.files.size(); i++) {
            NativeQueue.Part part = record.files.get(i);
            if (part.compressed) continue;
            JSONObject meta = media == null ? null : media.optJSONObject(i);
            if (meta == null || !"video".equals(meta.optString("kind"))) continue;
            File input = queue.partFile(record.id, part);
            File output = new File(queue.recordDir(record.id), "c" + part.slot + ".mp4");
            try {
                VideoCompressor.compress(getApplicationContext(), input, output,
                        record.maxHeight);
                queue.markCompressed(record, part, output, "video.mp4", "video/mp4",
                        output.length());
            } catch (Exception error) {
                Log.w(TAG, "native compress failed, sending original", error);
                if (output.exists()) output.delete();
                queue.markCompressionDone(record, part);
            }
        }
    }

    /** 5.5: у ролика без кадра снимаем первый кадр на телефоне. */
    private void ensurePosters(NativeQueue queue, NativeQueue.Record record) {
        if (record.files.isEmpty()) return;
        JSONArray media;
        try {
            media = new JSONObject(record.payload).optJSONArray("media_meta");
        } catch (Exception error) {
            return;
        }
        if (media == null) return;
        for (int i = 0; i < record.files.size(); i++) {
            JSONObject meta = media.optJSONObject(i);
            if (meta == null || !"video".equals(meta.optString("kind"))) continue;
            if (!meta.optString("video_poster_blob_id", "").isEmpty()) continue;
            boolean hasPoster = false;
            for (NativeQueue.Part aux : record.aux) {
                if (aux.mediaIndex == i && "video_poster".equals(aux.field)) {
                    hasPoster = true;
                    break;
                }
            }
            if (hasPoster) continue;
            File video = queue.partFile(record.id, record.files.get(i));
            File output = new File(queue.recordDir(record.id), "p" + i + ".jpg");
            File jpeg = PosterExtractor.extract(video, output);
            if (jpeg == null) {
                Log.w(TAG, "poster: no frame for " + record.files.get(i).name);
                continue;
            }
            try {
                queue.addAuxPoster(record, i, jpeg);
            } catch (Exception error) {
                Log.w(TAG, "poster: cannot keep frame", error);
                if (output.exists()) output.delete();
            }
        }
    }

    /** Поставить прогон сети; REPLACE не даёт копиться одинаковым задачам. */
    static void schedule(Context context) {
        Constraints constraints = new Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build();
        OneTimeWorkRequest request = new OneTimeWorkRequest.Builder(NativeQueueWorker.class)
                .setConstraints(constraints)
                .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
                .build();
        WorkManager.getInstance(context.getApplicationContext())
                .enqueueUniqueWork(UNIQUE, ExistingWorkPolicy.REPLACE, request);
    }
}
