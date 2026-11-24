package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.io.RandomAccessFile;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Arrays;

/** Отправка одной записи нативной очереди тем же API, что и очередь страницы. */
final class NativeQueueSender {
    static final int CHUNK_SIZE = 1024 * 1024;

    static final class Reply {
        final int status;
        final String body;
        final long uploadOffset;

        Reply(int status, String body) {
            this(status, body, -1);
        }

        Reply(int status, String body, long uploadOffset) {
            this.status = status;
            this.body = body;
            this.uploadOffset = uploadOffset;
        }
    }

    interface Transport {
        Reply send(String method, String url, String token, String body) throws IOException;

        /** Тело порции — уже готовые байты файла, не JSON. */
        default Reply chunk(String url, String token, long offset, byte[] body) throws IOException {
            throw new IOException("chunk unsupported");
        }
    }

    interface Parts {
        File file(NativeQueue.Record record, NativeQueue.Part part);

        void saved(NativeQueue.Record record, NativeQueue.Part part);
    }

    static final class Outcome {
        final String state;
        final String error;

        private Outcome(String state, String error) {
            this.state = state;
            this.error = error;
        }

        static Outcome sent() {
            return new Outcome("sent", "");
        }

        static Outcome unauthorized() {
            return new Outcome("unauthorized", "");
        }

        static Outcome retry() {
            return new Outcome("retry", "");
        }

        static Outcome failed(String error) {
            return new Outcome("failed", error);
        }
    }

    static Outcome send(NativeQueue.Record record, Transport transport) {
        return send(record, transport, null);
    }

    static Outcome send(NativeQueue.Record record, Transport transport, Parts parts) {
        try {
            JSONObject payload = new JSONObject(record.payload);
            if (record.files.isEmpty() && record.aux.isEmpty()) {
                return sendJson(record, payload, transport);
            }
            if (parts == null) return Outcome.failed("invalid");
            for (NativeQueue.Part file : record.files) upload(record, file, transport, parts);
            for (NativeQueue.Part aux : record.aux) upload(record, aux, transport, parts);
            return sendJson(record, payload, transport);
        } catch (Fail fail) {
            return fail.outcome;
        } catch (IOException error) {
            // Сеть/сервер не ответил: запись остаётся и уйдёт следующим прогоном.
            return Outcome.retry();
        } catch (Exception error) {
            return Outcome.failed("invalid");
        }
    }

    private static Outcome sendJson(NativeQueue.Record record, JSONObject payload, Transport transport)
            throws Exception {
        String method;
        String path;
        String body;
        switch (record.type) {
            case "post": {
                JSONObject out = new JSONObject(record.payload);
                out.remove("media_meta");
                out.put("client_id", record.clientId);
                out.put("media", media(record, payload));
                method = "POST";
                path = "/api/v1/circles/" + record.circleId + "/posts";
                body = out.toString();
                break;
            }
            case "comment": {
                JSONObject out = new JSONObject(record.payload);
                out.remove("media_meta");
                out.put("client_id", record.clientId);
                out.put("media", commentMedia(record, payload));
                method = "POST";
                path = "/api/v1/circles/" + record.circleId + "/posts/"
                        + payload.getString("post_id") + "/comments";
                body = out.toString();
                break;
            }
            case "reaction": {
                method = "PUT";
                path = "/api/v1/circles/" + record.circleId + "/posts/"
                        + payload.getString("post_id") + "/reactions";
                body = new JSONObject().put("emoji", payload.getString("emoji")).toString();
                break;
            }
            case "reaction_remove": {
                method = "DELETE";
                path = "/api/v1/circles/" + record.circleId + "/posts/"
                        + payload.getString("post_id") + "/reactions";
                body = null;
                break;
            }
            default:
                throw new Fail(Outcome.failed("invalid"));
        }

        Reply reply = transport.send(method, record.origin + path, record.token, body);
        if (reply.status >= 200 && reply.status < 300) return Outcome.sent();
        if (reply.status == 401) return Outcome.unauthorized();
        if (reply.status == 408 || reply.status == 425 || reply.status == 429
                || reply.status == 507 || reply.status >= 500) {
            return Outcome.retry();
        }
        return Outcome.failed(errorCode(reply));
    }

    /** Один файл: сессия, HEAD-возобновление, порции, SHA по ушедшим байтам. */
    private static void upload(NativeQueue.Record record, NativeQueue.Part part,
                               Transport transport, Parts parts) throws Exception {
        if (part.blobId != null && !part.blobId.isEmpty()) return;
        File file = parts.file(record, part);
        if (file == null || !file.isFile() || file.length() != part.size) {
            throw new Fail(Outcome.failed("invalid"));
        }

        String session = part.sessionId;
        for (int attempt = 0; attempt < 2; attempt++) {
            long offset = 0;
            if (session != null && !session.isEmpty()) {
                Reply head = transport.send("HEAD",
                        record.origin + "/api/v1/uploads/" + session, record.token, null);
                if (head.status == 404 || head.status == 410) {
                    session = null;
                    part.sessionId = null;
                    part.blobId = null;
                    parts.saved(record, part);
                } else if (head.status == 401) {
                    throw new Fail(Outcome.unauthorized());
                } else if (head.status < 200 || head.status >= 300) {
                    throw new Fail(classify(head));
                } else {
                    offset = head.uploadOffset > 0 ? head.uploadOffset : 0;
                    if (offset > part.size) throw new Fail(Outcome.failed("invalid"));
                }
            }
            if (session == null || session.isEmpty()) {
                JSONObject createBody = new JSONObject()
                        .put("expected_size", part.size)
                        .put("mime_type", part.mime)
                        .put("filename", part.name);
                Reply create = transport.send("POST", record.origin + "/api/v1/uploads",
                        record.token, createBody.toString());
                if (create.status < 200 || create.status >= 300) throw new Fail(classify(create));
                session = new JSONObject(create.body).optString("id", "");
                if (session.isEmpty()) throw new Fail(Outcome.failed("invalid"));
                part.sessionId = session;
                part.blobId = null;
                parts.saved(record, part);
                offset = 0;
            }

            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            RandomAccessFile input = new RandomAccessFile(file, "r");
            boolean reset = false;
            try {
                byte[] buffer = new byte[CHUNK_SIZE];
                long hashed = 0;
                while (hashed < offset) {
                    int read = input.read(buffer, 0, (int) Math.min(buffer.length, offset - hashed));
                    if (read <= 0) throw new Fail(Outcome.failed("invalid"));
                    digest.update(buffer, 0, read);
                    hashed += read;
                }
                while (offset < part.size) {
                    input.seek(offset);
                    int read = input.read(buffer, 0,
                            (int) Math.min(buffer.length, part.size - offset));
                    if (read <= 0) throw new Fail(Outcome.failed("invalid"));
                    byte[] chunk = Arrays.copyOf(buffer, read);
                    digest.update(chunk);
                    Reply reply = transport.chunk(
                            record.origin + "/api/v1/uploads/" + session, record.token, offset, chunk);
                    if (reply.status == 404 || reply.status == 410) {
                        session = null;
                        part.sessionId = null;
                        part.blobId = null;
                        parts.saved(record, part);
                        reset = true;
                        break;
                    }
                    if (reply.status == 401) throw new Fail(Outcome.unauthorized());
                    if (reply.status < 200 || reply.status >= 300) throw new Fail(classify(reply));
                    long received = new JSONObject(reply.body).optLong("received_bytes", -1);
                    if (received <= offset || received > part.size) {
                        throw new Fail(Outcome.retry());
                    }
                    offset = received;
                }
            } finally {
                input.close();
            }
            if (reset) continue;

            String sha = hex(digest.digest());
            Reply complete = transport.send("POST",
                    record.origin + "/api/v1/uploads/" + session + "/complete",
                    record.token, new JSONObject().put("sha256", sha).toString());
            if (complete.status == 404 || complete.status == 410) {
                session = null;
                part.sessionId = null;
                part.blobId = null;
                parts.saved(record, part);
                continue;
            }
            if (complete.status == 401) throw new Fail(Outcome.unauthorized());
            if (complete.status < 200 || complete.status >= 300) throw new Fail(classify(complete));
            String blobId = new JSONObject(complete.body).optString("id", "");
            if (blobId.isEmpty()) throw new Fail(Outcome.failed("invalid"));
            part.blobId = blobId;
            parts.saved(record, part);
            return;
        }
        throw new Fail(Outcome.retry());
    }

    private static JSONArray media(NativeQueue.Record record, JSONObject payload) throws Exception {
        JSONArray out = new JSONArray();
        JSONArray meta = payload.optJSONArray("media_meta");
        for (int i = 0; i < record.files.size(); i++) {
            NativeQueue.Part file = record.files.get(i);
            JSONObject source = meta == null ? null : meta.optJSONObject(i);
            JSONObject item = new JSONObject();
            item.put("blob_id", file.blobId);
            item.put("kind", source == null ? "photo" : source.optString("kind", "photo"));
            if (source != null) {
                copy(source, item, "captured_at");
                copy(source, item, "geo_lat");
                copy(source, item, "geo_lng");
                copy(source, item, "audio_artist");
                copy(source, item, "audio_title");
                copy(source, item, "audio_duration_ms");
                copy(source, item, "audio_peaks");
                copy(source, item, "video_duration_ms");
                copy(source, item, "voice");
                item.put("is_cover", source.optBoolean("is_cover", false));
                if (source.optBoolean("is_cover", false) && source.has("crop")) {
                    item.put("crop", source.get("crop"));
                }
                for (NativeQueue.Part aux : record.aux) {
                    if (aux.mediaIndex != i || aux.blobId == null) continue;
                    if ("video_poster".equals(aux.field)) item.put("video_poster_blob_id", aux.blobId);
                    if ("audio_cover".equals(aux.field)) item.put("audio_cover_blob_id", aux.blobId);
                }
            }
            out.put(item);
        }
        return out;
    }

    private static JSONArray commentMedia(NativeQueue.Record record, JSONObject payload) throws Exception {
        JSONArray out = new JSONArray();
        JSONArray meta = payload.optJSONArray("media_meta");
        for (int i = 0; i < record.files.size(); i++) {
            NativeQueue.Part file = record.files.get(i);
            JSONObject source = meta == null ? null : meta.optJSONObject(i);
            JSONObject item = new JSONObject();
            item.put("blob_id", file.blobId);
            item.put("kind", source != null && "attachment".equals(source.optString("kind"))
                    ? "attachment" : "photo");
            if (source != null) {
                copy(source, item, "voice");
                copy(source, item, "audio_duration_ms");
                copy(source, item, "audio_peaks");
            }
            out.put(item);
        }
        return out;
    }

    private static void copy(JSONObject source, JSONObject target, String key) {
        if (!source.has(key)) return;
        try {
            target.put(key, source.get(key));
        } catch (Exception ignored) {
            // Поле необязательное; сервер применит умолчание.
        }
    }

    private static Outcome classify(Reply reply) {
        if (reply.status == 401) return Outcome.unauthorized();
        if (reply.status == 408 || reply.status == 425 || reply.status == 429
                || reply.status == 507 || reply.status >= 500) {
            return Outcome.retry();
        }
        return Outcome.failed(errorCode(reply));
    }

    private static String errorCode(Reply reply) {
        try {
            String code = new JSONObject(reply.body).optString("error", "");
            if (!code.isEmpty() && code.length() <= 64) return code;
        } catch (Exception ignored) {
            // Тело не JSON — останется код HTTP.
        }
        return "http_" + reply.status;
    }

    private static String hex(byte[] bytes) {
        StringBuilder out = new StringBuilder(bytes.length * 2);
        for (byte value : bytes) out.append(String.format("%02x", value));
        return out.toString();
    }

    private static final class Fail extends Exception {
        final Outcome outcome;

        Fail(Outcome outcome) {
            this.outcome = outcome;
        }
    }

    /** HTTP без сторонних библиотек; редиректы не продолжаем, TLS не ослабляем. */
    static final class HttpTransport implements Transport {
        static final HttpTransport INSTANCE = new HttpTransport();

        private static final int CONNECT_TIMEOUT_MS = 15_000;
        private static final int READ_TIMEOUT_MS = 60_000;
        private static final int MAX_REPLY = 128 * 1024;

        @Override
        public Reply send(String method, String url, String token, String body) throws IOException {
            HttpURLConnection connection = (HttpURLConnection) new URL(url).openConnection();
            try {
                connection.setRequestMethod(method);
                connection.setConnectTimeout(CONNECT_TIMEOUT_MS);
                connection.setReadTimeout(READ_TIMEOUT_MS);
                connection.setInstanceFollowRedirects(false);
                connection.setRequestProperty("Authorization", "Bearer " + token);
                connection.setRequestProperty("Accept", "application/json");
                if (body != null) {
                    byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
                    connection.setDoOutput(true);
                    connection.setRequestProperty("Content-Type", "application/json");
                    connection.setFixedLengthStreamingMode(bytes.length);
                    try (OutputStream output = connection.getOutputStream()) {
                        output.write(bytes);
                    }
                }
                int status = connection.getResponseCode();
                return new Reply(status, read(status >= 400
                        ? connection.getErrorStream() : connection.getInputStream()),
                        offset(connection));
            } finally {
                connection.disconnect();
            }
        }

        @Override
        public Reply chunk(String url, String token, long offset, byte[] body) throws IOException {
            HttpURLConnection connection = (HttpURLConnection) new URL(url).openConnection();
            try {
                connection.setRequestMethod("PUT");
                connection.setConnectTimeout(CONNECT_TIMEOUT_MS);
                connection.setReadTimeout(READ_TIMEOUT_MS);
                connection.setInstanceFollowRedirects(false);
                connection.setRequestProperty("Authorization", "Bearer " + token);
                connection.setRequestProperty("Accept", "application/json");
                connection.setRequestProperty("Content-Type", "application/octet-stream");
                connection.setRequestProperty("Upload-Offset", Long.toString(offset));
                connection.setDoOutput(true);
                connection.setFixedLengthStreamingMode(body.length);
                try (OutputStream output = connection.getOutputStream()) {
                    output.write(body);
                }
                int status = connection.getResponseCode();
                return new Reply(status, read(status >= 400
                        ? connection.getErrorStream() : connection.getInputStream()),
                        offset(connection));
            } finally {
                connection.disconnect();
            }
        }

        private static long offset(HttpURLConnection connection) {
            String value = connection.getHeaderField("Upload-Offset");
            if (value == null) return -1;
            try {
                return Long.parseLong(value.trim());
            } catch (NumberFormatException error) {
                return -1;
            }
        }

        private static String read(InputStream input) throws IOException {
            if (input == null) return "";
            try (InputStream body = input) {
                ByteArrayOutputStream out = new ByteArrayOutputStream();
                byte[] buffer = new byte[4096];
                int total = 0;
                int read;
                while (total < MAX_REPLY && (read = body.read(buffer)) != -1) {
                    int keep = Math.min(read, MAX_REPLY - total);
                    out.write(buffer, 0, keep);
                    total += keep;
                }
                return new String(out.toByteArray(), StandardCharsets.UTF_8);
            }
        }
    }
}
