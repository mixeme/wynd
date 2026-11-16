package mixeme.wynd;

import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

/** Только явно выбранные внешние источники; ни путей приложения, ни записи. */
final class SelectedMediaRegistry {
    static final class Entry {
        final String owner, source, name, mime;
        final long size;
        Entry(String owner, String source, String name, String mime, long size) {
            this.owner = owner; this.source = source; this.name = name; this.mime = mime; this.size = size;
        }
    }
    private final ConcurrentHashMap<String, Entry> items = new ConcurrentHashMap<>();
    String register(Entry entry) {
        if (items.size() >= 4096) throw new IllegalStateException("Too many selected media");
        String token = UUID.randomUUID().toString(); items.put(token, entry); return token;
    }
    Entry get(String token) { return PhotoPolicy.token(token) ? items.get(token) : null; }
    void clear(String owner) {
        for (java.util.Map.Entry<String, Entry> item : items.entrySet()) {
            if (item.getValue().owner.equals(owner)) items.remove(item.getKey(), item.getValue());
        }
    }
}
