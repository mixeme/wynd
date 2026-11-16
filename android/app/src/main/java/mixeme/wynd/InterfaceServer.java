package mixeme.wynd;

import org.json.JSONObject;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

/** Как в Настольном: /api/v1/instance без входа; редирект не меняет проверяемый адрес. */
final class InterfaceServer {
    static JSONObject check(String address, boolean debug) throws Exception {
        ServerOrigin origin = ServerOrigin.parse(address, debug);
        HttpURLConnection connection = (HttpURLConnection) new URL(origin.url() + "api/v1/instance").openConnection();
        connection.setConnectTimeout(8000);
        connection.setReadTimeout(8000);
        connection.setInstanceFollowRedirects(false);
        connection.setUseCaches(false);
        connection.setRequestProperty("Accept", "application/json");
        try {
            if (connection.getResponseCode() != 200) throw new IOException("Unavailable interface");
            String contentType = connection.getContentType();
            if (contentType == null || !contentType.toLowerCase(java.util.Locale.ROOT).startsWith("application/json")) throw new IOException("Invalid content type");
            long deadline = System.nanoTime() + java.util.concurrent.TimeUnit.SECONDS.toNanos(12);
            ByteArrayOutputStream body = new ByteArrayOutputStream();
            try (InputStream input = connection.getInputStream()) {
                byte[] buffer = new byte[4096];
                int count;
                while ((count = input.read(buffer)) != -1) {
                    if (body.size() + count > 65536 || System.nanoTime() > deadline) throw new IOException("Instance limit");
                    body.write(buffer, 0, count);
                }
            }
            return parse(origin, body.toString(StandardCharsets.UTF_8.name()));
        } finally { connection.disconnect(); }
    }
    static JSONObject parse(ServerOrigin origin, String body) throws Exception {
        JSONObject info = new JSONObject(body);
        String name = info.getString("name").trim();
        String version = info.getString("version");
        String mode = info.getString("registration_mode");
        if (name.length() > 512 || version.isEmpty() || version.length() > 80
                || !(mode.equals("open") || mode.equals("invite") || mode.equals("closed"))) {
            throw new IOException("Invalid instance");
        }
        String canonical = origin.url().substring(0, origin.url().length() - 1);
        if (name.isEmpty()) name = canonical.substring(canonical.indexOf("://") + 3);
        return new JSONObject().put("origin", canonical).put("name", name);
    }
}
