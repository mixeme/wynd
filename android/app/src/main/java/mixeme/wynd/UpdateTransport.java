package mixeme.wynd;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

final class UpdateTransport {
    private final boolean debug;
    private final String fixtureOrigin;
    private volatile long responseTimeMillis;

    UpdateTransport(boolean debug, String fixtureOrigin) {
        this.debug = debug;
        this.fixtureOrigin = fixtureOrigin;
    }

    long responseTimeMillis() throws IOException {
        if (responseTimeMillis <= 0) throw new IOException("Release server time unavailable");
        return responseTimeMillis;
    }

    String text(String url, int limit) throws IOException {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        transfer(url, out, limit);
        return new String(out.toByteArray(), StandardCharsets.UTF_8);
    }

    void download(String url, File file, long limit) throws IOException {
        try (OutputStream out = new FileOutputStream(file)) {
            transfer(url, out, limit);
        } catch (IOException e) {
            if (file.exists() && !file.delete()) file.deleteOnExit();
            throw e;
        }
    }

    private void transfer(String address, OutputStream out, long limit) throws IOException {
        URL url = new URL(address);
        for (int redirects = 0; redirects <= 5; redirects++) {
            if (!UpdatePolicy.transportUrlAllowed(url.toString(), debug, fixtureOrigin)) throw new IOException("Untrusted update URL");
            HttpURLConnection connection = (HttpURLConnection) url.openConnection();
            connection.setConnectTimeout(15000);
            connection.setReadTimeout(30000);
            connection.setInstanceFollowRedirects(false);
            connection.setRequestProperty("Accept", "application/vnd.github+json");
            connection.setRequestProperty("User-Agent", "Wynd-Android");
            try {
                int status = connection.getResponseCode();
                if (status == 301 || status == 302 || status == 303 || status == 307 || status == 308) {
                    String location = connection.getHeaderField("Location");
                    if (location == null) throw new IOException("Missing redirect target");
                    url = new URL(url, location);
                    continue;
                }
                if (status != 200) throw new IOException("Update HTTP error");
                responseTimeMillis = connection.getHeaderFieldDate("Date", -1);
                if (connection.getContentLength() > limit) throw new IOException("Update response too large");
                try (InputStream input = connection.getInputStream()) {
                    byte[] buffer = new byte[32768];
                    long count = 0;
                    int read;
                    while ((read = input.read(buffer)) != -1) {
                        count += read;
                        if (count > limit) throw new IOException("Update response too large");
                        out.write(buffer, 0, read);
                    }
                }
                return;
            } finally {
                connection.disconnect();
            }
        }
        throw new IOException("Too many update redirects");
    }
}
