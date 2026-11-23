package mixeme.wynd;

import android.content.Context;
import android.util.AtomicFile;
import android.util.Log;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Locale;
import java.util.UUID;
import java.util.regex.Pattern;

/**
 * Очередь, которая живёт в приложении, а не в странице (план 50, этап 5.1):
 * записи без вложений уходят и после закрытия окна. Источник истины для
 * экрана по-прежнему очередь IndexedDB; здесь — её нативная копия на время
 * отправки. Токен и тело записи лежат в private-файле приложения, как сессии
 * WebView; наружу не отдаются.
 */
final class NativeQueue {
    private static final String TAG = "WyndAndroid";
    private static final int MAX_RECORDS = 100;
    private static final int MAX_PAYLOAD_CHARS = 65_536;
    private static final int MAX_TOKEN = 4_096;
    private static final int MAX_JSON = 8 * 1024 * 1024;
    private static final long STATUS_TTL_MS = 7L * 24 * 60 * 60 * 1000;
    private static final Pattern UUID_RE = Pattern.compile(
            "[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}");
    private static final Pattern ID_RE = Pattern.compile("[A-Za-z0-9_-]{1,128}");

    interface Listener {
        void onQueueChanged();
    }

    interface Storage {
        String read() throws IOException;

        void write(String value) throws IOException;
    }

    private static NativeQueue instance;

    static synchronized NativeQueue get(Context context) {
        if (instance == null) instance = new NativeQueue(context.getApplicationContext());
        return instance;
    }

    private final Storage storage;
    private final boolean debug;
    private final List<Record> records = new ArrayList<>();
    private final List<Status> statuses = new ArrayList<>();
    private Listener listener;

    private NativeQueue(Context context) {
        this(new AtomicStorage(new AtomicFile(
                new File(context.getFilesDir(), "native-queue.json"))), BuildConfig.DEBUG);
    }

    NativeQueue(Storage storage, boolean debug) {
        this.storage = storage;
        this.debug = debug;
        load();
    }

    synchronized void setListener(Listener value) {
        listener = value;
    }

    synchronized void clearListener(Listener value) {
        if (listener == value) listener = null;
    }

    /** Добавляет запись; повтор того же client_id не создаёт вторую. */
    synchronized String add(JSONObject request) throws Exception {
        Record record = Record.from(request, debug);
        for (Record existing : records) {
            if (existing.clientId.equals(record.clientId)) return existing.id;
        }
        for (Status status : statuses) {
            if (status.clientId.equals(record.clientId)) throw new IllegalArgumentException("duplicate");
        }
        if (records.size() >= MAX_RECORDS) throw new IllegalArgumentException("queue_full");
        records.add(record);
        save();
        notifyChanged();
        return record.id;
    }

    synchronized List<Record> records() {
        return new ArrayList<>(records);
    }

    synchronized boolean hasRecords() {
        return !records.isEmpty();
    }

    synchronized JSONObject status() throws JSONException {
        JSONArray pending = new JSONArray();
        for (Record record : records) {
            pending.put(new JSONObject().put("client_id", record.clientId).put("state", "pending"));
        }
        JSONArray done = new JSONArray();
        for (Status status : statuses) done.put(status.toJson());
        return new JSONObject().put("records", pending).put("statuses", done);
    }

    synchronized void markSent(Record record) {
        if (!removeRecord(record.clientId)) return;
        addStatus(new Status(record.clientId, record.origin, "sent", "", System.currentTimeMillis()));
        save();
        notifyChanged();
    }

    synchronized void markFailed(Record record, String error) {
        if (!removeRecord(record.clientId)) return;
        addStatus(new Status(record.clientId, record.origin, "failed", error, System.currentTimeMillis()));
        save();
        notifyChanged();
    }

    synchronized void markUnauthorized(Record record) {
        if (!removeRecord(record.clientId)) return;
        addStatus(new Status(record.clientId, record.origin, "unauthorized", "", System.currentTimeMillis()));
        save();
        notifyChanged();
    }

    synchronized void retry(Record record) {
        for (Record existing : records) {
            if (existing.clientId.equals(record.clientId)) {
                existing.attempts++;
                save();
                notifyChanged();
                return;
            }
        }
    }

    /** Правка/удаление записи на экране: нативная копия больше не нужна. */
    synchronized void cancel(String clientId) {
        if (clientId == null || !UUID_RE.matcher(clientId.toLowerCase(Locale.ROOT)).matches()) return;
        boolean changed = removeRecord(clientId);
        changed |= removeIf(statuses, s -> s.clientId.equals(clientId));
        if (changed) {
            save();
            notifyChanged();
        }
    }

    /** Выход из учётной записи: не отправлять её очередь после выхода. */
    synchronized void clear(String origin) throws Exception {
        String normalized = origin == null ? "" : origin.replaceAll("/+$", "");
        ServerOrigin.parse(normalized, debug);
        boolean changed = removeIf(records, r -> r.origin.equals(normalized));
        changed |= removeIf(statuses, s -> s.origin.equals(normalized));
        if (changed) {
            save();
            notifyChanged();
        }
    }

    private boolean removeRecord(String clientId) {
        return removeIf(records, r -> r.clientId.equals(clientId));
    }

    private void addStatus(Status status) {
        removeIf(statuses, s -> s.clientId.equals(status.clientId));
        statuses.add(status);
        prune();
    }

    private void prune() {
        long cutoff = System.currentTimeMillis() - STATUS_TTL_MS;
        removeIf(statuses, s -> s.at < cutoff);
        while (statuses.size() > 500) statuses.remove(0);
    }

    /**
     * Collection.removeIf — API 24; на minSdk 21 идём итератором. Свой
     * интерфейс, а не java.util.function (тоже API 24).
     */
    private interface Match<T> {
        boolean test(T value);
    }

    private static <T> boolean removeIf(List<T> values, Match<T> match) {
        boolean removed = false;
        for (Iterator<T> iterator = values.iterator(); iterator.hasNext(); ) {
            if (match.test(iterator.next())) {
                iterator.remove();
                removed = true;
            }
        }
        return removed;
    }

    private void load() {
        try {
            String text = storage.read();
            if (text == null || text.isEmpty()) return;
            JSONObject root = new JSONObject(text);
            JSONArray storedRecords = root.optJSONArray("records");
            if (storedRecords != null) {
                for (int i = 0; i < storedRecords.length() && records.size() < MAX_RECORDS; i++) {
                    try {
                        records.add(Record.fromStored(storedRecords.getJSONObject(i), debug));
                    } catch (Exception ignored) {
                        // Одна испорченная запись не уносит остальные.
                    }
                }
            }
            JSONArray storedStatuses = root.optJSONArray("statuses");
            if (storedStatuses != null) {
                for (int i = 0; i < storedStatuses.length(); i++) {
                    try {
                        statuses.add(Status.from(storedStatuses.getJSONObject(i)));
                    } catch (Exception ignored) {
                        // То же для статусов.
                    }
                }
            }
            prune();
        } catch (Exception error) {
            Log.w(TAG, "native queue: cannot read", error);
        }
    }

    private void save() {
        JSONObject root = new JSONObject();
        try {
            JSONArray storedRecords = new JSONArray();
            for (Record record : records) storedRecords.put(record.toJson());
            JSONArray storedStatuses = new JSONArray();
            for (Status status : statuses) storedStatuses.put(status.toJson());
            root.put("records", storedRecords).put("statuses", storedStatuses);
            String text = root.toString();
            if (text.length() > MAX_JSON) {
                Log.w(TAG, "native queue: file too large");
                return;
            }
            storage.write(text);
        } catch (Exception error) {
            Log.w(TAG, "native queue: cannot write", error);
        }
    }

    private void notifyChanged() {
        Listener current = listener;
        if (current == null) return;
        try {
            current.onQueueChanged();
        } catch (RuntimeException error) {
            Log.w(TAG, "native queue: listener failed", error);
        }
    }

    static final class Record {
        final String id;
        final String type;
        final String origin;
        final String circleId;
        final String clientId;
        final String payload;
        final String token;
        final long createdAt;
        int attempts;

        Record(String id, String type, String origin, String circleId, String clientId,
               String payload, String token, long createdAt, int attempts) {
            this.id = id;
            this.type = type;
            this.origin = origin;
            this.circleId = circleId;
            this.clientId = clientId;
            this.payload = payload;
            this.token = token;
            this.createdAt = createdAt;
            this.attempts = attempts;
        }

        static Record from(JSONObject request, boolean debug) throws Exception {
            String type = request.getString("type");
            if (!type.equals("post") && !type.equals("comment")
                    && !type.equals("reaction") && !type.equals("reaction_remove")) {
                throw new IllegalArgumentException("type");
            }
            String origin = request.getString("origin").replaceAll("/+$", "");
            ServerOrigin.parse(origin, debug);
            String circleId = request.getString("circle_id");
            if (!ID_RE.matcher(circleId).matches()) throw new IllegalArgumentException("circle");
            String clientId = request.getString("client_id").toLowerCase(Locale.ROOT);
            if (!UUID_RE.matcher(clientId).matches()) throw new IllegalArgumentException("client");
            JSONObject payload = request.getJSONObject("payload");
            if (payload.toString().length() > MAX_PAYLOAD_CHARS) {
                throw new IllegalArgumentException("payload");
            }
            if (type.equals("post") || type.equals("comment")) {
                JSONArray media = payload.optJSONArray("media_meta");
                if (media != null && media.length() > 0) throw new IllegalArgumentException("media");
                if (type.equals("comment") && !ID_RE.matcher(payload.optString("post_id", "")).matches()) {
                    throw new IllegalArgumentException("post");
                }
            } else {
                if (!ID_RE.matcher(payload.optString("post_id", "")).matches()) {
                    throw new IllegalArgumentException("post");
                }
                if (type.equals("reaction")) {
                    String emoji = payload.optString("emoji", "");
                    if (emoji.isEmpty() || emoji.length() > 64) throw new IllegalArgumentException("emoji");
                }
            }
            String token = request.getString("token");
            if (token.isEmpty() || token.length() > MAX_TOKEN
                    || token.indexOf('\n') >= 0 || token.indexOf('\r') >= 0) {
                throw new IllegalArgumentException("token");
            }
            return new Record(UUID.randomUUID().toString(), type, origin, circleId, clientId,
                    payload.toString(), token, System.currentTimeMillis(), 0);
        }

        static Record fromStored(JSONObject stored, boolean debug) throws Exception {
            JSONObject request = new JSONObject()
                    .put("type", stored.getString("type"))
                    .put("origin", stored.getString("origin"))
                    .put("circle_id", stored.getString("circle_id"))
                    .put("client_id", stored.getString("client_id"))
                    .put("payload", new JSONObject(stored.getString("payload")))
                    .put("token", stored.getString("token"));
            Record checked = from(request, debug);
            String id = stored.optString("id", "");
            if (id.isEmpty() || id.length() > 128) throw new IllegalArgumentException("id");
            return new Record(id, checked.type, checked.origin, checked.circleId, checked.clientId,
                    checked.payload, checked.token, stored.optLong("created_at", checked.createdAt),
                    stored.optInt("attempts", 0));
        }

        JSONObject toJson() throws JSONException {
            return new JSONObject()
                    .put("id", id)
                    .put("type", type)
                    .put("origin", origin)
                    .put("circle_id", circleId)
                    .put("client_id", clientId)
                    .put("payload", payload)
                    .put("token", token)
                    .put("created_at", createdAt)
                    .put("attempts", attempts);
        }
    }

    static final class Status {
        final String clientId;
        final String origin;
        final String state;
        final String error;
        final long at;

        Status(String clientId, String origin, String state, String error, long at) {
            this.clientId = clientId;
            this.origin = origin;
            this.state = state;
            this.error = error;
            this.at = at;
        }

        static Status from(JSONObject stored) throws JSONException {
            String clientId = stored.getString("client_id").toLowerCase(Locale.ROOT);
            if (!UUID_RE.matcher(clientId).matches()) throw new IllegalArgumentException("client");
            String state = stored.getString("state");
            if (!state.equals("sent") && !state.equals("failed") && !state.equals("unauthorized")) {
                throw new IllegalArgumentException("state");
            }
            return new Status(clientId, stored.getString("origin").replaceAll("/+$", ""),
                    state, stored.optString("error", ""), stored.optLong("at", 0));
        }

        JSONObject toJson() throws JSONException {
            return new JSONObject()
                    .put("client_id", clientId)
                    .put("origin", origin)
                    .put("state", state)
                    .put("error", error)
                    .put("at", at);
        }
    }

    private static final class AtomicStorage implements Storage {
        private final AtomicFile file;

        AtomicStorage(AtomicFile file) {
            this.file = file;
        }

        @Override
        public String read() throws IOException {
            if (!file.getBaseFile().exists()) return null;
            byte[] bytes = file.readFully();
            if (bytes.length > MAX_JSON) throw new IOException("queue too large");
            return new String(bytes, StandardCharsets.UTF_8);
        }

        @Override
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
    }
}
