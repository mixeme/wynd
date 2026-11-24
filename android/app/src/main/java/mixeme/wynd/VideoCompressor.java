package mixeme.wynd;

import android.content.Context;
import android.net.Uri;
import android.os.Handler;
import android.os.HandlerThread;

import androidx.media3.common.Effect;
import androidx.media3.common.MediaItem;
import androidx.media3.common.MimeTypes;
import androidx.media3.common.audio.AudioProcessor;
import androidx.media3.common.util.UnstableApi;
import androidx.media3.effect.Presentation;
import androidx.media3.transformer.Composition;
import androidx.media3.transformer.EditedMediaItem;
import androidx.media3.transformer.Effects;
import androidx.media3.transformer.ExportException;
import androidx.media3.transformer.ExportResult;
import androidx.media3.transformer.Transformer;

import java.io.File;
import java.util.Collections;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Фоновое сжатие видео (план 50, 5.3): Media3 Transformer перекодирует
 * оригинал в H.264/AAC и уменьшает высоту. Без ffmpeg и сервера; медиа
 * остаётся на телефоне до загрузки.
 */
@UnstableApi
final class VideoCompressor {
    private static final long TIMEOUT_MINUTES = 30;

    private VideoCompressor() {
    }

    static void compress(Context context, File input, File output, int maxHeight) throws Exception {
        if (!input.isFile() || input.length() <= 0) throw new IllegalArgumentException("input");
        if (output.exists() && !output.delete()) throw new IllegalStateException("output");
        HandlerThread thread = new HandlerThread("wynd-video-compress");
        thread.start();
        CountDownLatch done = new CountDownLatch(1);
        AtomicReference<Exception> failure = new AtomicReference<>();
        AtomicReference<Transformer> running = new AtomicReference<>();
        Handler handler = new Handler(thread.getLooper());
        handler.post(() -> {
            try {
                MediaItem item = MediaItem.fromUri(Uri.fromFile(input));
                EditedMediaItem edited = new EditedMediaItem.Builder(item)
                        .setEffects(effects(maxHeight))
                        .build();
                Transformer transformer = new Transformer.Builder(context.getApplicationContext())
                        .setVideoMimeType(MimeTypes.VIDEO_H264)
                        .setAudioMimeType(MimeTypes.AUDIO_AAC)
                        .addListener(new Transformer.Listener() {
                            @Override
                            public void onCompleted(Composition composition, ExportResult result) {
                                done.countDown();
                            }

                            @Override
                            public void onError(Composition composition, ExportResult result,
                                                ExportException error) {
                                failure.set(error);
                                done.countDown();
                            }
                        })
                        .build();
                running.set(transformer);
                transformer.start(edited, output.getAbsolutePath());
            } catch (Exception error) {
                failure.set(error);
                done.countDown();
            }
        });
        boolean finished = done.await(TIMEOUT_MINUTES, TimeUnit.MINUTES);
        if (!finished) {
            Transformer transformer = running.get();
            if (transformer != null) transformer.cancel();
            thread.quitSafely();
            throw new IllegalStateException("compression timeout");
        }
        thread.quitSafely();
        Exception error = failure.get();
        if (error != null) throw error;
        if (!output.isFile() || output.length() <= 0) {
            throw new IllegalStateException("compression produced no file");
        }
    }

    private static Effects effects(int maxHeight) {
        if (maxHeight <= 0) return new Effects(
                Collections.<AudioProcessor>emptyList(), Collections.<Effect>emptyList());
        return new Effects(Collections.<AudioProcessor>emptyList(),
                Collections.<Effect>singletonList(Presentation.createForHeight(maxHeight)));
    }
}
