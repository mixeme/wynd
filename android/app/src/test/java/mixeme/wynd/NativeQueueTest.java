package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONObject;
import org.junit.Test;

import java.nio.file.Files;
import java.util.List;
import java.util.UUID;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;
import static org.junit.Assert.fail;

public class NativeQueueTest {
    static final class Memory implements NativeQueue.Storage {
        String value;

        @Override
        public String read() {
            return value;
        }

        @Override
        public void write(String next) {
            value = next;
        }
    }

    private static JSONObject post(String client) throws Exception {
        return new JSONObject()
                .put("type", "post")
                .put("origin", "https://srv.test")
                .put("circle_id", "circle1")
                .put("client_id", client)
                .put("payload", new JSONObject().put("body", "hello")
                        .put("entry_date", "2026-11-24"))
                .put("token", "token-1");
    }

    private static String client() {
        return UUID.randomUUID().toString();
    }

    @Test
    public void storesAndRestoresRecords() throws Exception {
        Memory storage = new Memory();
        NativeQueue queue = new NativeQueue(storage, true);
        String id = queue.add(post(client()));
        assertTrue(queue.hasRecords());
        assertEquals(1, queue.status().getJSONArray("records").length());

        NativeQueue restored = new NativeQueue(storage, true);
        assertEquals(1, restored.records().size());
        assertEquals(id, restored.records().get(0).id);
        assertEquals("token-1", restored.records().get(0).token);
    }

    @Test
    public void repeatedClientIdDoesNotCreateATwin() throws Exception {
        NativeQueue queue = new NativeQueue(new Memory(), true);
        JSONObject request = post(client());
        String first = queue.add(request);
        String second = queue.add(request);
        assertEquals(first, second);
        assertEquals(1, queue.records().size());
    }

    @Test
    public void sentClientIsNotAcceptedAgain() throws Exception {
        NativeQueue queue = new NativeQueue(new Memory(), true);
        JSONObject request = post(client());
        queue.add(request);
        queue.markSent(queue.records().get(0));
        try {
            queue.add(request);
            fail("duplicate after sent must be rejected");
        } catch (IllegalArgumentException expected) {
            assertEquals("duplicate", expected.getMessage());
        }
    }

    @Test
    public void cancelAndClearRemoveRecordsAndStatuses() throws Exception {
        NativeQueue queue = new NativeQueue(new Memory(), true);
        JSONObject first = post(client());
        JSONObject second = post(client());
        queue.add(first);
        queue.add(second);
        String firstClient = first.getString("client_id");
        queue.markSent(queue.records().get(0));
        queue.cancel(firstClient);
        assertEquals(1, queue.records().size());
        assertEquals(0, queue.status().getJSONArray("statuses").length());

        queue.clear("https://srv.test");
        assertFalse(queue.hasRecords());
    }

    @Test
    public void rejectsFilesAndUnsafeInput() throws Exception {
        NativeQueue queue = new NativeQueue(new Memory(), true);
        JSONObject media = post(client());
        media.getJSONObject("payload").put("media_meta",
                new org.json.JSONArray().put(new JSONObject().put("kind", "photo")));
        reject(queue, media);

        JSONObject token = post(client()).put("token", "a\nb");
        reject(queue, token);

        JSONObject badClient = post("not-a-uuid");
        reject(queue, badClient);

        JSONObject loopback = post(client()).put("origin", "http://127.0.0.1:7676");
        NativeQueue release = new NativeQueue(new Memory(), false);
        reject(release, loopback);
        new NativeQueue(new Memory(), true).add(loopback); // В debug-петле http разрешён.
    }

    @Test
    public void statusesTellThePageWhatHappened() throws Exception {
        NativeQueue queue = new NativeQueue(new Memory(), true);
        queue.add(post(client()));
        queue.markUnauthorized(queue.records().get(0));
        assertEquals("unauthorized",
                queue.status().getJSONArray("statuses").getJSONObject(0).getString("state"));

        queue.add(post(client()));
        queue.markFailed(queue.records().get(0), "invalid");
        JSONObject failed = queue.status().getJSONArray("statuses").getJSONObject(1);
        assertEquals("failed", failed.getString("state"));
        assertEquals("invalid", failed.getString("error"));
    }

    @Test
    public void commitFilesStoresPartsAndDeletesThemAfterSend() throws Exception {
        Memory storage = new Memory();
        java.io.File base = Files.createTempDirectory("wynd-queue").toFile();
        NativeQueue queue = new NativeQueue(storage, base, true);
        JSONObject request = post(client());
        request.getJSONObject("payload").put("media_meta",
                new JSONArray().put(new JSONObject().put("kind", "photo")));
        NativeQueue.Part file = NativeQueue.Part.main(
                new JSONObject().put("name", "a.jpg").put("mime", "image/jpeg").put("size", 4), 0);
        String id = queue.begin();
        Files.write(queue.partFile(id, file).toPath(), new byte[]{1, 2, 3, 4});
        queue.commit(id, request, List.of(file), List.of());
        assertEquals(1, queue.records().size());
        assertEquals(id, queue.records().get(0).id);
        assertEquals(4, queue.partFile(id, file).length());

        NativeQueue restored = new NativeQueue(storage, base, true);
        assertEquals(1, restored.records().size());
        assertEquals(1, restored.records().get(0).files.size());

        queue.markSent(queue.records().get(0));
        assertTrue(queue.records().isEmpty());
        assertFalse(queue.partFile(id, file).exists());
    }

    @Test
    public void auxPosterTravelsWithItsMediaIndex() throws Exception {
        java.io.File base = Files.createTempDirectory("wynd-queue").toFile();
        NativeQueue queue = new NativeQueue(new Memory(), base, true);
        JSONObject request = post(client());
        request.getJSONObject("payload").put("media_meta",
                new JSONArray().put(new JSONObject().put("kind", "video")));
        NativeQueue.Part video = NativeQueue.Part.main(
                new JSONObject().put("name", "v.mp4").put("mime", "video/mp4").put("size", 4), 0);
        NativeQueue.Part poster = NativeQueue.Part.aux(
                new JSONObject().put("name", "poster.jpg").put("mime", "image/jpeg")
                        .put("size", 2).put("media_index", 0).put("field", "video_poster"), 0);
        String id = queue.begin();
        Files.write(queue.partFile(id, video).toPath(), new byte[]{1, 2, 3, 4});
        Files.write(queue.partFile(id, poster).toPath(), new byte[]{5, 6});
        queue.commit(id, request, List.of(video), List.of(poster));
        NativeQueue.Record stored = queue.records().get(0);
        assertEquals(1, stored.aux.size());
        assertEquals("video_poster", stored.aux.get(0).field);
        assertEquals(0, stored.aux.get(0).mediaIndex);
    }

    private static void reject(NativeQueue queue, JSONObject request) throws Exception {
        try {
            queue.add(request);
            fail("request must be rejected");
        } catch (Exception expected) {
            // Ожидаемо: запись не проходит проверку.
        }
    }
}
