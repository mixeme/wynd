package mixeme.wynd;

import android.content.Context;
import android.util.AtomicFile;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Iterator;
import java.util.regex.Pattern;

/**
 * Общее хранилище входов оболочки Android (C127, вариант «б»): записи участника
 * по адресу сервера, а не в IndexedDB выбранного источника. Любой интерфейс
 * видит все входы, как окно для компьютера через /__secrets.
 */
final class SecretsStore {
    /** Файл оболочки: атомарная запись, как у списка источников. */
    interface Storage {
        String read() throws IOException;
        void write(String value) throws IOException;
    }

    private static final String SESSIONS = "sessions";
    private static final int VERSION = 1;
    private static final int MAX_SESSIONS = 32;
    private static final int MAX_BLOB = 4096;
    private static final Pattern ACCOUNT = Pattern.compile("[A-Za-z0-9_-]{1,128}");

    private final Storage storage;
    private final boolean debug;
    private JSONObject sessions = new JSONObject();

    SecretsStore(Storage storage, boolean debug) throws IOException, JSONException {
        this.storage = storage;
        this.debug = debug;
        String saved = storage.read();
        if (saved == null) return;
        JSONObject stored = new JSONObject(saved).optJSONObject(SESSIONS);
        if (stored == null) throw new JSONException("Invalid secrets");
        if (stored.length() > MAX_SESSIONS) throw new JSONException("Too many sessions");
        Iterator<String> keys = stored.keys();
        while (keys.hasNext()) {
            String key = keys.next();
            JSONObject record = stored.optJSONObject(key);
            if (record == null || !canonical(record.optString("origin")).equals(key)) {
                throw new JSONException("Invalid session");
            }
            sessions.put(key, record);
        }
    }

    @SuppressWarnings("deprecation") // AtomicFile: тот же файл и приёмы, что у списка источников.
    static SecretsStore open(Context context, boolean debug) throws IOException, JSONException {
        AtomicFile file = new AtomicFile(new File(context.getFilesDir(), "secrets.json"));
        return new SecretsStore(new Storage() {
            public String read() throws IOException {
                File base = file.getBaseFile();
                if (!base.exists() && !new File(base + ".bak").exists()) return null;
                byte[] bytes = file.readFully();
                if (bytes.length > 64 * 1024) throw new IOException("Secrets store too large");
                return new String(bytes, StandardCharsets.UTF_8);
            }

            public void write(String value) throws IOException {
                FileOutputStream output = file.startWrite();
                try {
                    output.write(value.getBytes(StandardCharsets.UTF_8));
                    file.finishWrite(output);
                } catch (IOException | RuntimeException error) {
                    file.failWrite(output);
                    throw error;
                }
            }
        }, debug);
    }

    synchronized JSONArray list() throws JSONException {
        JSONArray out = new JSONArray();
        Iterator<String> keys = sessions.keys();
        while (keys.hasNext()) out.put(new JSONObject(sessions.getJSONObject(keys.next()).toString()));
        return out;
    }

    synchronized void put(JSONObject session) throws IOException, JSONException {
        String origin = canonical(session.optString("origin"));
        JSONObject record = new JSONObject()
                .put("origin", origin)
                .put("name", text(session, "name", 512))
                .put("email", text(session, "email", 320))
                .put("token", text(session, "token", 1024))
                .put("account_id", account(session));
        String signedInAt = session.optString("signed_in_at", "");
        if (!signedInAt.isEmpty()) record.put("signed_in_at", text(session, "signed_in_at", 64));
        if (record.toString().getBytes(StandardCharsets.UTF_8).length > MAX_BLOB) {
            throw new JSONException("Session too large");
        }
        if (!sessions.has(origin) && sessions.length() >= MAX_SESSIONS) {
            throw new JSONException("Too many sessions");
        }
        JSONObject next = new JSONObject(sessions.toString());
        next.put(origin, record);
        commit(next);
    }

    synchronized void remove(String origin) throws IOException, JSONException {
        String key = canonical(origin);
        if (!sessions.has(key)) return;
        JSONObject next = new JSONObject(sessions.toString());
        next.remove(key);
        commit(next);
    }

    /** Запись идёт до смены состояния в памяти: сбой диска не оставляет «сохранённое». */
    private void commit(JSONObject next) throws IOException, JSONException {
        storage.write(new JSONObject().put(SESSIONS, next).put("version", VERSION).toString());
        sessions = next;
    }

    private String canonical(String value) {
        String url = ServerOrigin.parse(value, debug).url();
        return url.substring(0, url.length() - 1);
    }

    private static String text(JSONObject source, String field, int limit) throws JSONException {
        Object value = source.opt(field);
        if (!(value instanceof String) || ((String) value).length() > limit) {
            throw new JSONException("Invalid session field: " + field);
        }
        return (String) value;
    }

    private static String account(JSONObject source) throws JSONException {
        String account = text(source, "account_id", 128);
        if (!ACCOUNT.matcher(account).matches()) throw new JSONException("Invalid account");
        return account;
    }
}
