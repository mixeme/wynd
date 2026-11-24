package mixeme.wynd;

import android.graphics.Bitmap;
import android.media.MediaMetadataRetriever;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;

/**
 * Кадр ролика на телефоне (план 50, 5.5): первый кадр через
 * MediaMetadataRetriever, JPEG до 1024 px. Сервер кадры больше не делает.
 */
final class PosterExtractor {
    private static final int MAX_EDGE = 1024;
    private static final int QUALITY = 80;
    private static final long FIRST_FRAME_US = 1_000_000;

    private PosterExtractor() {
    }

    /** Пишет JPEG в output; null — кадр снять не удалось. */
    static File extract(File video, File output) {
        if (video == null || !video.isFile()) return null;
        MediaMetadataRetriever retriever = new MediaMetadataRetriever();
        Bitmap frame = null;
        Bitmap scaled = null;
        try {
            retriever.setDataSource(video.getAbsolutePath());
            frame = retriever.getFrameAtTime(FIRST_FRAME_US,
                    MediaMetadataRetriever.OPTION_CLOSEST_SYNC);
            if (frame == null) {
                frame = retriever.getFrameAtTime(0, MediaMetadataRetriever.OPTION_CLOSEST_SYNC);
            }
            if (frame == null) return null;
            int width = frame.getWidth();
            int height = frame.getHeight();
            float scale = Math.min(1f, (float) MAX_EDGE / Math.max(width, height));
            scaled = scale < 1f
                    ? Bitmap.createScaledBitmap(frame, Math.round(width * scale),
                    Math.round(height * scale), true)
                    : frame;
            ByteArrayOutputStream bytes = new ByteArrayOutputStream();
            if (!scaled.compress(Bitmap.CompressFormat.JPEG, QUALITY, bytes)) return null;
            try (FileOutputStream stream = new FileOutputStream(output)) {
                stream.write(bytes.toByteArray());
            }
            return output;
        } catch (RuntimeException | IOException error) {
            if (output.exists()) output.delete();
            return null;
        } finally {
            if (scaled != null && scaled != frame) scaled.recycle();
            if (frame != null) frame.recycle();
            try {
                retriever.release();
            } catch (IOException | RuntimeException ignored) {
                // Уже отпущен.
            }
        }
    }
}
