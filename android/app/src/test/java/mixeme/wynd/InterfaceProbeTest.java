package mixeme.wynd;

import org.junit.Test;
import org.json.JSONObject;
import java.io.ByteArrayInputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import static org.junit.Assert.*;

public final class InterfaceProbeTest {
    private static final String CARD = "{\"name\":\"Дом Ани\",\"version\":\"0.53.0\",\"registration_mode\":\"invite\",\"bootstrapped\":true,\"allow_name_erasure\":true}";
    private HttpURLConnection response(int status, String type, String body, int length) throws Exception {
        return new HttpURLConnection(new URL("https://home.example.org/api/v1/instance")) {
            @Override public int getResponseCode() { return status; }
            @Override public String getContentType() { return type; }
            @Override public int getContentLength() { return length; }
            @Override public ByteArrayInputStream getInputStream() { return new ByteArrayInputStream(body.getBytes(StandardCharsets.UTF_8)); }
            @Override public void connect() { }
            @Override public void disconnect() { }
            @Override public boolean usingProxy() { return false; }
        };
    }
    @Test public void readsPublicMetadataIncludingAnUnnamedFreshServer() throws Exception {
        JSONObject result = InterfaceProbe.read(response(200, "application/json; charset=utf-8", CARD, -1));
        assertEquals("Дом Ани", result.getString("name"));
        assertTrue(result.getBoolean("allow_name_erasure"));
        assertEquals(2, result.length());
        assertEquals("", InterfaceProbe.read(response(200, "application/json", CARD.replace("Дом Ани", ""), -1)).getString("name"));
    }
    @Test public void rejectsRedirectsHtmlAndUnrelatedJson() throws Exception {
        for (int status : new int[] {301, 302, 307, 308, 401, 500})
            assertThrows(Exception.class, () -> InterfaceProbe.read(response(status, "application/json", CARD, -1)));
        for (String type : new String[] {"text/html", "text/plain", "", null})
            assertThrows(Exception.class, () -> InterfaceProbe.read(response(200, type, CARD, -1)));
        for (String json : new String[] {"{}", "[]", "<html>", CARD.replace("\"version\":\"0.53.0\"", "\"version\":0"), CARD.replace("invite", "unknown")})
            assertThrows(Exception.class, () -> InterfaceProbe.read(response(200, "application/json", json, -1)));
    }
    @Test public void boundsDeclaredAndStreamingBodies() throws Exception {
        assertThrows(Exception.class, () -> InterfaceProbe.read(response(200, "application/json", CARD, 65537)));
        String large = CARD + " ".repeat(65537);
        assertThrows(Exception.class, () -> InterfaceProbe.read(response(200, "application/json", large, -1)));
    }
    @Test public void invitationCannotBecomeAnExternalOrDeviceNavigation() {
        for (String path : new String[] {"/", "/invite/token_123-abc", "/join/token?circle=abc&x=1"}) assertTrue(path, InterfaceBridge.validPath(path));
        for (String path : new String[] {"//evil.org", "/invite/../settings", "/settings", "https://evil.org/", "/invite/token#fragment", "/join/token%2fsettings", "/join/token?x=\\evil", "/join/"}) assertFalse(path, InterfaceBridge.validPath(path));
    }
}
