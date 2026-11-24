package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONObject;
import org.junit.Test;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertTrue;

public class NativeQueueSenderTest {
    private static final class Fake implements NativeQueueSender.Transport {
        String method;
        String url;
        String token;
        String body;
        NativeQueueSender.Reply reply = new NativeQueueSender.Reply(200, "{}");
        IOException failure;

        @Override
        public NativeQueueSender.Reply send(String method, String url, String token, String body)
                throws IOException {
            this.method = method;
            this.url = url;
            this.token = token;
            this.body = body;
            if (failure != null) throw failure;
            return reply;
        }
    }

    private static final class UploadFake implements NativeQueueSender.Transport {
        final List<String> calls = new ArrayList<>();
        final List<Long> offsets = new ArrayList<>();
        int created;
        int completed;
        String postBody;

        @Override
        public NativeQueueSender.Reply send(String method, String url, String token, String body)
                throws IOException {
            calls.add(method + " " + url);
            if (method.equals("HEAD")) return new NativeQueueSender.Reply(200, "", 2);
            if (url.endsWith("/uploads") && method.equals("POST")) {
                return new NativeQueueSender.Reply(201, "{\"id\":\"s" + (++created) + "\"}");
            }
            if (url.endsWith("/complete")) {
                return new NativeQueueSender.Reply(200, "{\"id\":\"b" + (++completed) + "\"}");
            }
            if (url.endsWith("/posts")) {
                postBody = body;
                return new NativeQueueSender.Reply(200, "{}");
            }
            return new NativeQueueSender.Reply(200, "{}");
        }

        @Override
        public NativeQueueSender.Reply chunk(String url, String token, long offset, byte[] body) {
            offsets.add(offset);
            return new NativeQueueSender.Reply(200,
                    "{\"received_bytes\":" + (offset + body.length) + "}", offset + body.length);
        }
    }

    private static NativeQueue.Record fileRecord(boolean poster) throws Exception {
        JSONObject payload = new JSONObject()
                .put("body", "файл")
                .put("entry_date", "2026-11-24")
                .put("media_meta", new JSONArray().put(new JSONObject()
                        .put("kind", poster ? "video" : "photo")));
        JSONObject request = new JSONObject()
                .put("type", "post")
                .put("origin", "https://srv.test")
                .put("circle_id", "circle1")
                .put("client_id", UUID.randomUUID().toString())
                .put("payload", payload)
                .put("token", "token-1");
        NativeQueue.Part file = NativeQueue.Part.main(
                new JSONObject().put("name", poster ? "v.mp4" : "a.jpg")
                        .put("mime", poster ? "video/mp4" : "image/jpeg").put("size", 4), 0);
        List<NativeQueue.Part> aux = poster
                ? List.of(NativeQueue.Part.aux(new JSONObject()
                        .put("name", "poster.jpg").put("mime", "image/jpeg").put("size", 2)
                        .put("media_index", 0).put("field", "video_poster"), 0))
                : List.of();
        return NativeQueue.Record.fromWithFiles(request, List.of(file), aux, true);
    }

    private static NativeQueue.Record record(String type, JSONObject payload) throws Exception {
        JSONObject request = new JSONObject()
                .put("type", type)
                .put("origin", "https://srv.test")
                .put("circle_id", "circle1")
                .put("client_id", UUID.randomUUID().toString())
                .put("payload", payload)
                .put("token", "token-1");
        return NativeQueue.Record.from(request, true);
    }

    @Test
    public void postCarriesClientIdAndNoMedia() throws Exception {
        Fake transport = new Fake();
        NativeQueue.Record record = record("post",
                new JSONObject().put("body", "hello").put("entry_date", "2026-11-24"));
        NativeQueueSender.Outcome outcome = NativeQueueSender.send(record, transport);

        assertEquals("sent", outcome.state);
        assertEquals("POST", transport.method);
        assertEquals("https://srv.test/api/v1/circles/circle1/posts", transport.url);
        assertEquals("token-1", transport.token);
        JSONObject body = new JSONObject(transport.body);
        assertEquals(record.clientId, body.getString("client_id"));
        assertEquals(0, body.getJSONArray("media").length());
        assertFalse(body.has("media_meta"));
    }

    @Test
    public void commentAndReactionUseTheirRoutes() throws Exception {
        Fake transport = new Fake();
        NativeQueue.Record comment = record("comment",
                new JSONObject().put("post_id", "post1").put("body", "привет"));
        assertEquals("sent", NativeQueueSender.send(comment, transport).state);
        assertEquals("POST", transport.method);
        assertEquals("https://srv.test/api/v1/circles/circle1/posts/post1/comments", transport.url);

        NativeQueue.Record reaction = record("reaction",
                new JSONObject().put("post_id", "post1").put("emoji", "🙂"));
        assertEquals("sent", NativeQueueSender.send(reaction, transport).state);
        assertEquals("PUT", transport.method);
        assertEquals("https://srv.test/api/v1/circles/circle1/posts/post1/reactions", transport.url);
        assertEquals("🙂", new JSONObject(transport.body).getString("emoji"));

        NativeQueue.Record remove = record("reaction_remove",
                new JSONObject().put("post_id", "post1"));
        assertEquals("sent", NativeQueueSender.send(remove, transport).state);
        assertEquals("DELETE", transport.method);
        assertNull(transport.body);
    }

    @Test
    public void failuresAreClassifiedForTheQueue() throws Exception {
        Fake transport = new Fake();
        NativeQueue.Record record = record("post",
                new JSONObject().put("body", "hello").put("entry_date", "2026-11-24"));

        transport.reply = new NativeQueueSender.Reply(401, "{}");
        assertEquals("unauthorized", NativeQueueSender.send(record, transport).state);

        transport.reply = new NativeQueueSender.Reply(507, "{\"error\":\"quota_exceeded\"}");
        assertEquals("retry", NativeQueueSender.send(record, transport).state);

        transport.reply = new NativeQueueSender.Reply(429, "{}");
        assertEquals("retry", NativeQueueSender.send(record, transport).state);

        transport.reply = new NativeQueueSender.Reply(500, "{}");
        assertEquals("retry", NativeQueueSender.send(record, transport).state);

        transport.reply = new NativeQueueSender.Reply(400, "{\"error\":\"invalid\"}");
        NativeQueueSender.Outcome failed = NativeQueueSender.send(record, transport);
        assertEquals("failed", failed.state);
        assertEquals("invalid", failed.error);

        transport.reply = new NativeQueueSender.Reply(404, "not json");
        NativeQueueSender.Outcome missing = NativeQueueSender.send(record, transport);
        assertEquals("failed", missing.state);
        assertEquals("http_404", missing.error);

        transport.failure = new IOException("offline");
        assertEquals("retry", NativeQueueSender.send(record, transport).state);
    }

    @Test
    public void uploadsFileChunksAndPostsBlob() throws Exception {
        NativeQueue.Record record = fileRecord(false);
        File file = Files.createTempFile("wynd-upload", ".jpg").toFile();
        Files.write(file.toPath(), new byte[]{1, 2, 3, 4});
        UploadFake transport = new UploadFake();
        NativeQueueSender.Parts parts = new NativeQueueSender.Parts() {
            @Override
            public File file(NativeQueue.Record value, NativeQueue.Part part) {
                return file;
            }

            @Override
            public void saved(NativeQueue.Record value, NativeQueue.Part part) {
                // Очередь сохраняет сессию сама; в тесте не нужна.
            }
        };

        NativeQueueSender.Outcome outcome = NativeQueueSender.send(record, transport, parts);
        assertEquals("sent", outcome.state);
        assertEquals(List.of(0L), transport.offsets);
        assertEquals("b1", record.files.get(0).blobId);
        assertTrue(transport.calls.stream()
                .anyMatch(call -> call.startsWith("POST https://srv.test/api/v1/uploads")));
        assertTrue(transport.calls.contains(
                "POST https://srv.test/api/v1/circles/circle1/posts"));
        JSONObject post = new JSONObject(transport.postBody);
        assertEquals(record.clientId, post.getString("client_id"));
        assertEquals("b1",
                post.getJSONArray("media").getJSONObject(0).getString("blob_id"));
    }

    @Test
    public void resumesFromUploadOffset() throws Exception {
        NativeQueue.Record record = fileRecord(false);
        record.files.get(0).sessionId = "s0";
        File file = Files.createTempFile("wynd-resume", ".jpg").toFile();
        Files.write(file.toPath(), new byte[]{1, 2, 3, 4});
        UploadFake transport = new UploadFake();
        NativeQueueSender.Parts parts = new NativeQueueSender.Parts() {
            @Override
            public File file(NativeQueue.Record value, NativeQueue.Part part) {
                return file;
            }

            @Override
            public void saved(NativeQueue.Record value, NativeQueue.Part part) {
                // В тесте не нужна.
            }
        };

        assertEquals("sent", NativeQueueSender.send(record, transport, parts).state);
        assertEquals(List.of(2L), transport.offsets);
        assertFalse(transport.calls.contains("POST https://srv.test/api/v1/uploads"));
    }

    @Test
    public void posterBlobGoesToMedia() throws Exception {
        NativeQueue.Record record = fileRecord(true);
        record.files.get(0).blobId = "b1";
        record.aux.get(0).blobId = "b2";
        UploadFake transport = new UploadFake();
        NativeQueueSender.Parts parts = new NativeQueueSender.Parts() {
            @Override
            public File file(NativeQueue.Record value, NativeQueue.Part part) {
                return null;
            }

            @Override
            public void saved(NativeQueue.Record value, NativeQueue.Part part) {
                // Уже загружено.
            }
        };

        assertEquals("sent", NativeQueueSender.send(record, transport, parts).state);
        JSONObject media = new JSONObject(transport.postBody)
                .getJSONArray("media").getJSONObject(0);
        assertEquals("b1", media.getString("blob_id"));
        assertEquals("b2", media.getString("video_poster_blob_id"));
    }
}
