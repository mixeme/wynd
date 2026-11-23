package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

/** Отправка одной записи нативной очереди тем же API, что и очередь страницы. */
final class NativeQueueSender {
    static final class Reply {
        final int status;
        final String body;

        Reply(int status, String body) {
            this.status = status;
            this.body = body;
        }
    }

    interface Transport {
        Reply send(String method, String url, String token, String body) throws IOException;
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
        try {
            JSONObject payload = new JSONObject(record.payload);
            String method;
            String path;
            String body;
            switch (record.type) {
                case "post": {
                    JSONObject out = new JSONObject(record.payload);
                    out.remove("media_meta");
                    out.put("client_id", record.clientId);
                    out.put("media", new JSONArray());
                    method = "POST";
                    path = "/api/v1/circles/" + record.circleId + "/posts";
                    body = out.toString();
                    break;
                }
                case "comment": {
                    JSONObject out = new JSONObject(record.payload);
                    out.remove("media_meta");
                    out.put("client_id", record.clientId);
                    out.put("media", new JSONArray());
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
                    return Outcome.failed("invalid");
            }

            Reply reply = transport.send(method, record.origin + path, record.token, body);
            if (reply.status >= 200 && reply.status < 300) return Outcome.sent();
            if (reply.status == 401) return Outcome.unauthorized();
            if (reply.status == 408 || reply.status == 425 || reply.status == 429
                    || reply.status == 507 || reply.status >= 500) {
                return Outcome.retry();
            }
            return Outcome.failed(errorCode(reply));
        } catch (IOException error) {
            // Сеть/сервер не ответил: запись остаётся и уйдёт следующим прогоном.
            return Outcome.retry();
        } catch (Exception error) {
            return Outcome.failed("invalid");
        }
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

    /** HTTP без сторонних библиотек; редиректы не продолжаем, TLS не ослабляем. */
    static final class HttpTransport implements Transport {
        static final HttpTransport INSTANCE = new HttpTransport();

        private static final int CONNECT_TIMEOUT_MS = 15_000;
        private static final int READ_TIMEOUT_MS = 30_000;
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
                InputStream input = status >= 400 ? connection.getErrorStream() : connection.getInputStream();
                return new Reply(status, read(input));
            } finally {
                connection.disconnect();
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
