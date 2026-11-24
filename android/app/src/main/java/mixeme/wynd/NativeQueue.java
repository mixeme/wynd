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
 * Очередь, которая живёт в приложении, а не в странице (план 50, этап 5):
 * записи уходят и после закрытия окна. 5.1 — без вложений; 5.2 — с файлами,
 * постерами и обложками. Источник истины для экрана по-прежнему очередь
 * IndexedDB; здесь — её нативная копия. Токен, тело и файлы лежат в private
 * каталоге приложения, как сессии WebView; наружу не отдаются.
 */
final class NativeQueue {
    private static final String TAG = "WyndAndroid";
    private static final int MAX_RECORDS = 100;
    private static final int MAX_FILES = 32;
    private static final long MAX_FILE_BYTES = 512L * 1024 * 1024;
    private static final int MAX_PAYLOAD_CHARS = 65_536;
    private static final int MAX_TOKEN = 4_096;
    private static final int MAX_JSON = 8 * 1024 * 1024;
    private static final long STATUS_TTL_MS = 7L * 24 * 60 * 60 * 1000;
    private static final Pattern UUID_RE = Pattern.compile(
            "[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}");
    private static final Pattern ID_RE = Pattern.compile("[A-Za-z0-9_-]{1,128}");
    private static final Pattern FILE_NAME_RE = Pattern.compile("[^/\\\\\\x00]{1,255}");

    interface Listener {
        void onQueueChanged();
    }

    interface Storage {
        String read() throws IOException;

        void write(String value) throws IOException;
    }

    private static NativeQueue instance;

    static synchronized NativeQueue get(Context context) {
        if (instance == null) {
            instance = new NativeQueue(
                    new AtomicStorage(new AtomicFile(
                            new File(context.getFilesDir(), "native-queue.json"))),
                    new File(context.getFilesDir(), "native-queue"),
                    BuildConfig.DEBUG);
        }
        return instance;
    }

    private final Storage storage;
    private final File baseDir;
    private final boolean debug;
    private final List<Record> records = new ArrayList<>();
    private final List<Status> statuses = new ArrayList<>();
    private Listener listener;

    NativeQueue(Storage storage, boolean debug) {
        this(storage, null, debug);
    }

    NativeQueue(Storage storage, File baseDir, boolean debug) {
        this.storage = storage;
        this.baseDir = baseDir;
        this.debug = debug;
        if (baseDir != null && !baseDir.exists()) baseDir.mkdirs();
        load();
        cleanupDirs();
    }

    synchronized void setListener(Listener value) {
        listener = value;
    }

    synchronized void clearListener(Listener value) {
        if (listener == value) listener = null;
    }

    /** Добавляет запись без вложений; повтор того же client_id не создаёт вторую. */
    synchronized String add(JSONObject request) throws Exception {
        Record record = Record.from(request, debug);
        return addRecord(record);
    }

    /** Каталог для файлов новой записи; наполняется мостом до commit. */
    synchronized String begin() throws IOException {
        if (baseDir == null) throw new IOException("no queue directory");
        String id = UUID.randomUUID().toString();
        File dir = recordDir(id);
        if (!dir.mkdirs() && !dir.isDirectory()) throw new IOException("cannot create queue directory");
        return id;
    }

    /** Завершает передачу файлов: проверяет размеры и ставит запись в очередь. */
    synchronized String commit(String id, JSONObject request, List<Part> files, List<Part> aux)
            throws Exception {
        if (!UUID_RE.matcher(id).matches()) throw new IllegalArgumentException("id");
        Record record = Record.fromWithFiles(request, files, aux, debug);
        record.id = id;
        File dir = recordDir(id);
        if (!dir.isDirectory()) throw new IllegalArgumentException("directory");
        validateFiles(record);
        return addRecord(record);
    }

    synchronized void abort(String id) {
        if (id == null || !UUID_RE.matcher(id).matches()) return;
        deleteDir(id);
    }

    private String addRecord(Record record) throws Exception {
        for (Record existing : records) {
            if (existing.clientId.equals(record.clientId)) {
                deleteDir(record.id);
                return existing.id;
            }
        }
        for (Status status : statuses) {
            if (status.clientId.equals(record.clientId)) {
                deleteDir(record.id);
                throw new IllegalArgumentException("duplicate");
            }
        }
        if (records.size() >= MAX_RECORDS) {
            deleteDir(record.id);
            throw new IllegalArgumentException("queue_full");
        }
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
            pending.put(new JSONObject()
                    .put("client_id", record.clientId)
                    .put("native_id", record.id)
                    .put("state", "pending"));
        }
        JSONArray done = new JSONArray();
        for (Status status : statuses) done.put(status.toJson());
        return new JSONObject().put("records", pending).put("statuses", done);
    }

    synchronized void markSent(Record record) {
        if (!removeRecord(record.clientId)) return;
        addStatus(new Status(record.clientId, record.id, record.origin,
                "sent", "", System.currentTimeMillis()));
        deleteDir(record.id);
        save();
        notifyChanged();
    }

    synchronized void markFailed(Record record, String error) {
        if (!removeRecord(record.clientId)) return;
        addStatus(new Status(record.clientId, record.id, record.origin,
                "failed", error, System.currentTimeMillis()));
        deleteDir(record.id);
        save();
        notifyChanged();
    }

    synchronized void markUnauthorized(Record record) {
        if (!removeRecord(record.clientId)) return;
        addStatus(new Status(record.clientId, record.id, record.origin,
                "unauthorized", "", System.currentTimeMillis()));
        deleteDir(record.id);
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

    /** Состояние одного файла после чанка/полной загрузки. */
    synchronized void setPart(String recordId, Part part) {
        for (Record record : records) {
            if (!record.id.equals(recordId)) continue;
            for (Part current : record.parts()) {
                if (current.aux == part.aux && current.slot == part.slot) {
                    current.sessionId = part.sessionId;
                    current.blobId = part.blobId;
                    save();
                    return;
                }
            }
        }
    }

    /**
     * Сжатие видео закончилось: подменяем файл в каталоге записи и не даём
     * отправить старые байты. Сессия и блоб сбрасываются — размер изменился.
     */
    synchronized void markCompressed(Record record, Part part, File output,
                                     String name, String mime, long size) throws IOException {
        Part current = findPart(record.id, part);
        if (current == null) {
            output.delete();
            return;
        }
        File destination = partFile(record.id, current);
        if (destination == null) throw new IOException("no queue directory");
        File backup = new File(destination.getParentFile(), destination.getName() + ".orig");
        if (backup.exists() && !backup.delete()) throw new IOException("backup");
        if (destination.exists() && !destination.renameTo(backup)) {
            throw new IOException("old file");
        }
        if (!output.renameTo(destination)) {
            backup.renameTo(destination);
            throw new IOException("rename");
        }
        backup.delete();
        current.name = name;
        current.mime = mime;
        current.size = size;
        current.compressed = true;
        current.sessionId = null;
        current.blobId = null;
        save();
    }

    /** Сжатие не удалось: отправляем оригинал, повторять перекодирование незачем. */
    synchronized void markCompressionDone(Record record, Part part) {
        Part current = findPart(record.id, part);
        if (current == null) return;
        current.compressed = true;
        save();
    }

    private Part findPart(String recordId, Part wanted) {
        for (Record record : records) {
            if (!record.id.equals(recordId)) continue;
            for (Part part : record.parts()) {
                if (part.aux == wanted.aux && part.slot == wanted.slot) return part;
            }
        }
        return null;
    }

    /** Правка/удаление записи на экране: нативная копия больше не нужна. */
    synchronized void cancel(String clientId) {
        if (clientId == null || !UUID_RE.matcher(clientId.toLowerCase(Locale.ROOT)).matches()) return;
        String id = null;
        for (Record record : records) {
            if (record.clientId.equals(clientId.toLowerCase(Locale.ROOT))) id = record.id;
        }
        boolean changed = removeRecord(clientId.toLowerCase(Locale.ROOT));
        changed |= removeIf(statuses, s -> s.clientId.equals(clientId.toLowerCase(Locale.ROOT)));
        if (id != null) deleteDir(id);
        if (changed) {
            save();
            notifyChanged();
        }
    }

    /** Выход из учётной записи: не отправлять её очередь после выхода. */
    synchronized void clear(String origin) throws Exception {
        String normalized = origin == null ? "" : origin.replaceAll("/+$", "");
        ServerOrigin.parse(normalized, debug);
        List<String> dirs = new ArrayList<>();
        for (Record record : records) {
            if (record.origin.equals(normalized)) dirs.add(record.id);
        }
        boolean changed = removeIf(records, r -> r.origin.equals(normalized));
        changed |= removeIf(statuses, s -> s.origin.equals(normalized));
        for (String id : dirs) deleteDir(id);
        if (changed) {
            save();
            notifyChanged();
        }
    }

    File recordDir(String id) {
        return baseDir == null ? null : new File(baseDir, id);
    }

    File partFile(String recordId, Part part) {
        File dir = recordDir(recordId);
        return dir == null ? null : new File(dir, part.fileName());
    }

    private void validateFiles(Record record) throws Exception {
        long total = 0;
        int count = 0;
        for (Part part : record.parts()) {
            File file = partFile(record.id, part);
            if (file == null || !file.isFile() || file.length() != part.size) {
                throw new IllegalArgumentException("file");
            }
            total += part.size;
            count++;
        }
        if (count > MAX_FILES || total > MAX_FILE_BYTES) throw new IllegalArgumentException("files");
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

    /** Убирает каталоги незавершённых передач и удалённых записей. */
    private void cleanupDirs() {
        if (baseDir == null) return;
        File[] children = baseDir.listFiles();
        if (children == null) return;
        for (File child : children) {
            if (!child.isDirectory()) continue;
            boolean known = false;
            for (Record record : records) {
                if (record.id.equals(child.getName())) {
                    known = true;
                    break;
                }
            }
            if (!known) deleteDir(child.getName());
        }
    }

    private void deleteDir(String id) {
        File dir = recordDir(id);
        if (dir == null) return;
        File[] files = dir.listFiles();
        if (files != null) for (File file : files) file.delete();
        dir.delete();
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

    /** Файл записи: основной (порядок media_meta) или постер/обложка. */
    static final class Part {
        String name;
        String mime;
        long size;
        final int slot;
        final int mediaIndex;
        final String field;
        final boolean aux;
        String sessionId;
        String blobId;
        boolean compressed;

        Part(String name, String mime, long size, int slot, int mediaIndex,
             String field, boolean aux, String sessionId, String blobId, boolean compressed) {
            this.name = name;
            this.mime = mime;
            this.size = size;
            this.slot = slot;
            this.mediaIndex = mediaIndex;
            this.field = field;
            this.aux = aux;
            this.sessionId = sessionId;
            this.blobId = blobId;
            this.compressed = compressed;
        }

        static Part main(JSONObject value, int index) throws Exception {
            return check(value, index, index, null, false, null, null, false);
        }

        static Part aux(JSONObject value, int index) throws Exception {
            String field = value.getString("field");
            int mediaIndex = value.getInt("media_index");
            if (!field.equals("video_poster") && !field.equals("audio_cover")) {
                throw new IllegalArgumentException("field");
            }
            return check(value, index, mediaIndex, field, true, null, null, false);
        }

        private static Part check(JSONObject value, int slot, int mediaIndex, String field,
                                  boolean aux, String sessionId, String blobId,
                                  boolean compressed) throws Exception {
            String name = value.getString("name");
            String mime = value.getString("mime");
            long size = value.getLong("size");
            if (!FILE_NAME_RE.matcher(name).matches() || mime.isEmpty()
                    || mime.length() > 255 || size <= 0 || size > MAX_FILE_BYTES) {
                throw new IllegalArgumentException("file");
            }
            return new Part(name, mime, size, slot, mediaIndex, field, aux,
                    sessionId, blobId, compressed);
        }

        static Part fromStored(JSONObject value, int slot, boolean aux) throws Exception {
            return check(value, slot, value.optInt("media_index", slot),
                    aux ? value.getString("field") : null, aux,
                    value.optString("session_id", ""), value.optString("blob_id", ""),
                    value.optBoolean("compressed", false));
        }

        String fileName() {
            return (aux ? "a" : "f") + slot;
        }

        JSONObject toJson() throws JSONException {
            JSONObject out = new JSONObject()
                    .put("name", name)
                    .put("mime", mime)
                    .put("size", size)
                    .put("media_index", mediaIndex);
            if (aux) out.put("field", field);
            if (sessionId != null && !sessionId.isEmpty()) out.put("session_id", sessionId);
            if (blobId != null && !blobId.isEmpty()) out.put("blob_id", blobId);
            if (compressed) out.put("compressed", true);
            return out;
        }
    }

    static final class Record {
        String id;
        final String type;
        final String origin;
        final String circleId;
        final String clientId;
        final String payload;
        final String token;
        final long createdAt;
        int attempts;
        final List<Part> files;
        final List<Part> aux;
        final boolean compress;
        final int maxHeight;
        final int bitrateKbps;

        Record(String id, String type, String origin, String circleId, String clientId,
               String payload, String token, long createdAt, int attempts,
               List<Part> files, List<Part> aux, boolean compress, int maxHeight,
               int bitrateKbps) {
            this.id = id;
            this.type = type;
            this.origin = origin;
            this.circleId = circleId;
            this.clientId = clientId;
            this.payload = payload;
            this.token = token;
            this.createdAt = createdAt;
            this.attempts = attempts;
            this.files = files;
            this.aux = aux;
            this.compress = compress;
            this.maxHeight = maxHeight;
            this.bitrateKbps = bitrateKbps;
        }

        List<Part> parts() {
            List<Part> all = new ArrayList<>(files.size() + aux.size());
            all.addAll(files);
            all.addAll(aux);
            return all;
        }

        static Record from(JSONObject request, boolean debug) throws Exception {
            return fromWithFiles(request, new ArrayList<>(), new ArrayList<>(), debug);
        }

        static Record fromWithFiles(JSONObject request, List<Part> files, List<Part> aux,
                                    boolean debug) throws Exception {
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
            if (files.size() + aux.size() > MAX_FILES) throw new IllegalArgumentException("files");
            long total = 0;
            for (Part part : files) total += part.size;
            for (Part part : aux) total += part.size;
            if (total > MAX_FILE_BYTES) throw new IllegalArgumentException("files");

            JSONArray media = payload.optJSONArray("media_meta");
            if (type.equals("post") || type.equals("comment")) {
                if (files.isEmpty()) {
                    if (media != null && media.length() > 0) throw new IllegalArgumentException("media");
                } else {
                    if (media == null || media.length() != files.size()) {
                        throw new IllegalArgumentException("media");
                    }
                    for (int i = 0; i < media.length(); i++) {
                        JSONObject meta = media.getJSONObject(i);
                        if (meta.has("video_poster") || meta.has("audio_cover")) {
                            throw new IllegalArgumentException("media");
                        }
                    }
                }
                if (type.equals("comment") && !ID_RE.matcher(payload.optString("post_id", "")).matches()) {
                    throw new IllegalArgumentException("post");
                }
            } else {
                if (!files.isEmpty() || !aux.isEmpty()) throw new IllegalArgumentException("media");
                if (!ID_RE.matcher(payload.optString("post_id", "")).matches()) {
                    throw new IllegalArgumentException("post");
                }
                if (type.equals("reaction")) {
                    String emoji = payload.optString("emoji", "");
                    if (emoji.isEmpty() || emoji.length() > 64) throw new IllegalArgumentException("emoji");
                }
            }
            for (Part part : aux) {
                if (part.mediaIndex < 0 || part.mediaIndex >= files.size()) {
                    throw new IllegalArgumentException("aux");
                }
            }
            // 5.3: офлайн-видео приходит оригиналом; native сжимает его после
            // закрытия окна. Настройки лежат в media_meta.native_compress и в
            // тело записи не попадают.
            boolean compress = false;
            int maxHeight = 0;
            int bitrateKbps = 0;
            if (media != null) {
                for (int i = 0; i < media.length(); i++) {
                    JSONObject meta = media.optJSONObject(i);
                    JSONObject settings = meta == null ? null : meta.optJSONObject("native_compress");
                    if (settings == null) continue;
                    compress = true;
                    maxHeight = Math.max(0, Math.min(2160, settings.optInt("max_height", 0)));
                    bitrateKbps = Math.max(0, Math.min(50_000,
                            settings.optInt("video_bitrate_kbps", 0)));
                    break;
                }
            }
            if (files.isEmpty()) compress = false;
            String token = request.getString("token");
            if (token.isEmpty() || token.length() > MAX_TOKEN
                    || token.indexOf('\n') >= 0 || token.indexOf('\r') >= 0) {
                throw new IllegalArgumentException("token");
            }
            return new Record(UUID.randomUUID().toString(), type, origin, circleId, clientId,
                    payload.toString(), token, System.currentTimeMillis(), 0, files, aux,
                    compress, maxHeight, bitrateKbps);
        }

        static Record fromStored(JSONObject stored, boolean debug) throws Exception {
            JSONObject request = new JSONObject()
                    .put("type", stored.getString("type"))
                    .put("origin", stored.getString("origin"))
                    .put("circle_id", stored.getString("circle_id"))
                    .put("client_id", stored.getString("client_id"))
                    .put("payload", new JSONObject(stored.getString("payload")))
                    .put("token", stored.getString("token"));
            List<Part> files = new ArrayList<>();
            List<Part> aux = new ArrayList<>();
            JSONArray storedFiles = stored.optJSONArray("files");
            if (storedFiles != null) {
                for (int i = 0; i < storedFiles.length(); i++) {
                    files.add(Part.fromStored(storedFiles.getJSONObject(i), i, false));
                }
            }
            JSONArray storedAux = stored.optJSONArray("aux");
            if (storedAux != null) {
                for (int i = 0; i < storedAux.length(); i++) {
                    aux.add(Part.fromStored(storedAux.getJSONObject(i), i, true));
                }
            }
            Record checked = fromWithFiles(request, files, aux, debug);
            String id = stored.optString("id", "");
            if (!UUID_RE.matcher(id).matches()) throw new IllegalArgumentException("id");
            checked.id = id;
            checked.attempts = stored.optInt("attempts", 0);
            return checked;
        }

        JSONObject toJson() throws JSONException {
            JSONArray storedFiles = new JSONArray();
            for (Part part : files) storedFiles.put(part.toJson());
            JSONArray storedAux = new JSONArray();
            for (Part part : aux) storedAux.put(part.toJson());
            return new JSONObject()
                    .put("id", id)
                    .put("type", type)
                    .put("origin", origin)
                    .put("circle_id", circleId)
                    .put("client_id", clientId)
                    .put("payload", payload)
                    .put("token", token)
                    .put("created_at", createdAt)
                    .put("attempts", attempts)
                    .put("files", storedFiles)
                    .put("aux", storedAux);
        }
    }

    static final class Status {
        final String clientId;
        final String nativeId;
        final String origin;
        final String state;
        final String error;
        final long at;

        Status(String clientId, String nativeId, String origin, String state, String error, long at) {
            this.clientId = clientId;
            this.nativeId = nativeId;
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
            return new Status(clientId, stored.optString("native_id", ""),
                    stored.getString("origin").replaceAll("/+$", ""),
                    state, stored.optString("error", ""), stored.optLong("at", 0));
        }

        JSONObject toJson() throws JSONException {
            return new JSONObject()
                    .put("client_id", clientId)
                    .put("native_id", nativeId)
                    .put("origin", origin)
                    .put("state", state)
                    .put("error", error)
                    .put("at", at);
        }
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
