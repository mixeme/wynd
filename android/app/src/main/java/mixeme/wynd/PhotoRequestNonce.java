package mixeme.wynd;

import java.util.UUID;

/** Одноразовая связь main-frame RPC с ближайшим file chooser; не открывает файлы сама. */
final class PhotoRequestNonce {
    private String token;
    private long issuedAt;
    String issue(long now) { token = UUID.randomUUID().toString(); issuedAt = now; return token; }
    boolean consume(String presented, long now) {
        boolean valid = PhotoPolicy.token(presented) && presented.equals(token) && now >= issuedAt && now - issuedAt <= 10000;
        token = null; return valid;
    }
    void clear() { token = null; }
}
