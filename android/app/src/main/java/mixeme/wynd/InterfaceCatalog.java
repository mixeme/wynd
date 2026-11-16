package mixeme.wynd;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;
import java.io.IOException;

/** Только источники интерфейса. Ни токенов, ни входов, ни очистки WebView. */
final class InterfaceCatalog {
    interface Storage {
        String read() throws IOException;
        void write(String value) throws IOException;
    }
    private final Storage storage;
    private final boolean debug;
    private JSONObject data;

    InterfaceCatalog(Storage storage, boolean debug, String initial) throws IOException, JSONException {
        this.storage = storage;
        this.debug = debug;
        String saved = storage.read();
        if (saved != null) {
            data = new JSONObject(saved);
            JSONArray servers = data.getJSONArray("servers");
            if (servers.length() > 32) throw new IOException("Invalid interface catalog");
            java.util.HashSet<String> origins = new java.util.HashSet<>();
            for (int i = 0; i < servers.length(); i++) {
                JSONObject item = servers.getJSONObject(i);
                String origin = canonical(item.getString("origin"));
                if (!origin.equals(item.getString("origin")) || !origins.add(origin)
                        || item.getString("name").length() > 512) throw new IOException("Invalid interface catalog");
            }
            if (!origins.contains(data.getString("selected"))) throw new IOException("Invalid selection");
        } else {
            String origin = canonical(initial);
            data = new JSONObject().put("selected", origin).put("servers", new JSONArray()
                    .put(new JSONObject().put("origin", origin).put("name", origin.substring(origin.indexOf("://") + 3))));
            storage.write(data.toString());
        }
    }

    private String canonical(String value) {
        String url = ServerOrigin.parse(value, debug).url();
        return url.substring(0, url.length() - 1);
    }

    synchronized JSONObject snapshot() throws JSONException { return new JSONObject(data.toString()); }
    synchronized String selected() throws JSONException { return data.getString("selected"); }
    synchronized boolean contains(String origin) throws JSONException {
        return find(data.getJSONArray("servers"), canonical(origin)) >= 0;
    }
    private static int find(JSONArray servers, String origin) throws JSONException {
        for (int i = 0; i < servers.length(); i++) {
            if (servers.getJSONObject(i).getString("origin").equals(origin)) return i;
        }
        return -1;
    }
    private void commit(JSONObject next) throws IOException {
        storage.write(next.toString());
        data = next;
    }
    synchronized void add(JSONObject checked) throws IOException, JSONException {
        String origin = canonical(checked.getString("origin"));
        JSONObject next = snapshot();
        JSONArray servers = next.getJSONArray("servers");
        int index = find(servers, origin);
        if (index < 0 && servers.length() >= 32) throw new IOException("Interface catalog full");
        JSONObject item = new JSONObject().put("origin", origin).put("name", checked.getString("name"));
        if (index < 0) servers.put(item); else servers.put(index, item);
        commit(next);
    }
    synchronized void select(String origin) throws IOException, JSONException {
        origin = canonical(origin);
        if (!contains(origin)) throw new IOException("Unknown interface");
        commit(snapshot().put("selected", origin));
    }
    synchronized void remove(String origin) throws IOException, JSONException {
        origin = canonical(origin);
        if (selected().equals(origin)) throw new IOException("Selected interface");
        JSONObject next = snapshot();
        JSONArray servers = next.getJSONArray("servers");
        int index = find(servers, origin);
        if (index < 0) throw new IOException("Unknown interface");
        servers.remove(index);
        commit(next);
    }
}
