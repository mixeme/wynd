package mixeme.wynd;

import android.content.Context;

import androidx.annotation.NonNull;
import androidx.work.BackoffPolicy;
import androidx.work.Constraints;
import androidx.work.ExistingWorkPolicy;
import androidx.work.NetworkType;
import androidx.work.OneTimeWorkRequest;
import androidx.work.WorkManager;
import androidx.work.Worker;
import androidx.work.WorkerParameters;

import java.io.File;
import java.util.concurrent.TimeUnit;

/**
 * Фоновый прогон нативной очереди (план 50, этап 5.1). WorkManager будит
 * приложение после закрытия окна; записи и токены уже лежат в private-файле,
 * поэтому страница для отправки не нужна.
 */
public final class NativeQueueWorker extends Worker {
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
