package mixeme.wynd;

import org.json.JSONObject;
import org.junit.Test;

import java.io.IOException;
import java.util.UUID;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNull;

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
}
