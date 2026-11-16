package mixeme.wynd;

import org.json.JSONObject;
import org.json.JSONTokener;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

/** Публичная карточка нового источника: без WebView cookies и переходов по redirect. */
final class InterfaceProbe {
    static JSONObject check(ServerOrigin origin) throws Exception {
        HttpURLConnection connection = (HttpURLConnection) new URL(origin.url() + "api/v1/instance").openConnection();
        try {
            connection.setInstanceFollowRedirects(false);
            connection.setConnectTimeout(5000);
            connection.setReadTimeout(5000);
            connection.setUseCaches(false);
            connection.setRequestProperty("Accept", "application/json");
            return read(connection).put("origin", origin.url());
        } finally { connection.disconnect(); }
    }

    static JSONObject read(HttpURLConnection connection) throws Exception {
        long deadline = System.nanoTime() + 12000000000L;
        if (connection.getResponseCode() != 200) throw new IOException("instance status");
        String type = connection.getContentType();
        if (type == null || !type.split(";", 2)[0].trim().equalsIgnoreCase("application/json")) throw new IOException("instance type");
        if (connection.getContentLength() > 65536) throw new IOException("instance size");
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        try (InputStream input = connection.getInputStream()) {
            byte[] buffer = new byte[4096]; int count;
            while (true) {
                long remaining = (deadline - System.nanoTime()) / 1000000;
                if (remaining <= 0) throw new IOException("instance timeout");
                connection.setReadTimeout((int) Math.min(5000, remaining));
                count = input.read(buffer);
                if (count == -1) break;
                if (bytes.size() + count > 65536) throw new IOException("instance size");
                bytes.write(buffer, 0, count);
            }
        }
        JSONTokener json = new JSONTokener(bytes.toString(StandardCharsets.UTF_8.name()));
        JSONObject card = new JSONObject(json);
        if (json.nextClean() != 0) throw new IOException("instance json");
        if (!(card.get("name") instanceof String) || !(card.get("version") instanceof String)
                || card.getString("version").isEmpty() || !(card.get("bootstrapped") instanceof Boolean)
                || !(card.get("registration_mode") instanceof String)
                || !java.util.Arrays.asList("open", "invite", "closed").contains(card.getString("registration_mode"))) {
            throw new IOException("instance schema");
        }
        return new JSONObject().put("name", card.getString("name"))
                .put("allow_name_erasure", card.optBoolean("allow_name_erasure", false));
    }
}
