package mixeme.wynd;

import org.junit.Test;
import java.io.BufferedReader;
import java.io.File;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import static org.junit.Assert.*;

public final class UpdateTransportTest {
    @Test public void usesResponseDateInsteadOfDeviceClock() throws Exception {
        try (TestServer server = new TestServer("200 OK", "Date: Sat, 01 Jan 2000 00:00:00 GMT\r\nContent-Length: 2\r\n", "[]")) {
            UpdateTransport transport = new UpdateTransport(true, server.origin());
            assertEquals("[]", transport.text(server.origin(), 1024));
            assertEquals(946_684_800_000L, transport.responseTimeMillis());
        }
    }

    @Test public void refusesToCallAnUndatedResponseTheLatestVersion() throws Exception {
        try (TestServer server = new TestServer("200 OK", "Date: invalid-date\r\nContent-Length: 2\r\n", "[]")) {
            UpdateTransport transport = new UpdateTransport(true, server.origin());
            transport.text(server.origin(), 1024);
            assertThrows(IOException.class, transport::responseTimeMillis);
        }
    }

    @Test public void enforcesActualDownloadLimitAndRemovesPartialApk() throws Exception {
        File directory = Files.createTempDirectory("wynd-update-bound-").toFile();
        File apk = new File(directory, "candidate.apk");
        try (TestServer server = new TestServer("200 OK", "Transfer-Encoding: chunked\r\n",
                "20\r\naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r\n0\r\n\r\n")) {
            UpdateTransport transport = new UpdateTransport(true, server.origin());
            assertThrows(IOException.class, () -> transport.download(server.origin(), apk, 16));
            assertFalse(apk.exists());
        } finally { Files.deleteIfExists(apk.toPath()); Files.deleteIfExists(directory.toPath()); }
    }

    @Test public void rejectsRedirectBeforeContactingForeignHost() throws Exception {
        try (TestServer server = new TestServer("302 Found", "Location: https://not-a-wynd-update.invalid/apk\r\nContent-Length: 0\r\n", "")) {
            UpdateTransport transport = new UpdateTransport(true, server.origin());
            IOException error = assertThrows(IOException.class, () -> transport.text(server.origin(), 1024));
            assertEquals("Untrusted update URL", error.getMessage());
        }
    }

    /** Одноразовый настоящий HTTP-ответ без дополнительных библиотек или внешней сети. */
    private static final class TestServer implements AutoCloseable {
        private final ServerSocket listener;
        private final ExecutorService worker = Executors.newSingleThreadExecutor();
        private final Future<?> request;

        TestServer(String status, String headers, String body) throws IOException {
            listener = new ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"));
            listener.setSoTimeout(5000);
            request = worker.submit(() -> {
                try (Socket client = listener.accept()) {
                    client.setSoTimeout(5000);
                    BufferedReader input = new BufferedReader(new InputStreamReader(client.getInputStream(), StandardCharsets.US_ASCII));
                    String line;
                    while ((line = input.readLine()) != null && !line.isEmpty()) { /* Заголовки GET. */ }
                    client.getOutputStream().write(("HTTP/1.1 " + status + "\r\nConnection: close\r\n" + headers + "\r\n" + body).getBytes(StandardCharsets.US_ASCII));
                    client.getOutputStream().flush();
                } catch (IOException e) { throw new RuntimeException(e); }
            });
        }

        String origin() { return "http://127.0.0.1:" + listener.getLocalPort(); }
        @Override public void close() throws Exception {
            try { request.get(5, TimeUnit.SECONDS); }
            finally { listener.close(); worker.shutdownNow(); }
        }
    }
}
