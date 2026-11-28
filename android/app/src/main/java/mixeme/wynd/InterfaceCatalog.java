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
            String selected = data.getString("selected");
            if (!origins.contains(selected) && !(selected.isEmpty() && servers.length() == 0)) throw new IOException("Invalid selection");
            normalizeSource();
        } else {
            data = new JSONObject().put("selected", "").put("source", "embedded").put("servers", new JSONArray());
            if (initial != null && !initial.trim().isEmpty()) {
                String origin = canonical(initial);
                data.put("selected", origin).getJSONArray("servers")
                        .put(new JSONObject().put("origin", origin).put("name", origin.substring(origin.indexOf("://") + 3)));
            }
            storage.write(data.toString());
        }
    }

    /**
     * Откуда страницы (план 50, схема Настольного, решение владельца 28.11):
     * embedded по умолчанию — в том числе для файлов без поля; серверный режим
     * требует выбранного сервера, как sourceLocked в Настольном.
     */
    synchronized String source() {
        return "server".equals(data.optString("source")) && !data.optString("selected").isEmpty()
                ? "server" : "embedded";
    }

    /** Чужое значение поля в файле не ломает запуск: читаем как вшитый. */
    private void normalizeSource() throws JSONException {
        if (!"server".equals(data.optString("source"))) data.put("source", "embedded");
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
        commit(withServer(checked));
    }
    /** Проверенный источник и выбор записываются одной атомарной операцией. */
    synchronized void choose(JSONObject checked) throws IOException, JSONException {
        JSONObject next = withServer(checked);
        commit(next.put("selected", canonical(checked.getString("origin"))));
    }
    private JSONObject withServer(JSONObject checked) throws IOException, JSONException {
        String origin = canonical(checked.getString("origin"));
        JSONObject next = snapshot();
        JSONArray servers = next.getJSONArray("servers");
        int index = find(servers, origin);
        if (index < 0 && servers.length() >= 32) throw new IOException("Interface catalog full");
        JSONObject item = new JSONObject().put("origin", origin).put("name", checked.getString("name"));
        if (index < 0) servers.put(item); else servers.put(index, item);
        return next;
    }
    /**
     * Выбор сервера из списка открывает его серверный интерфейс (как
     * SelectServer в Настольном); он же остаётся источником данных.
     */
    synchronized void select(String origin) throws IOException, JSONException {
        origin = canonical(origin);
        if (!contains(origin)) throw new IOException("Unknown interface");
        commit(snapshot().put("selected", origin).put("source", "server"));
    }

    /** Вшитый интерфейс из APK; выбранный сервер остаётся источником данных. */
    synchronized void selectEmbedded() throws IOException, JSONException {
        commit(snapshot().put("source", "embedded"));
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
