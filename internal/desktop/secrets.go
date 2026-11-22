package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Сессии окна в системном хранилище (план 66, функция 3): токены уходят из
// IndexedDB профиля WebView2 в учётные записи Windows (Credential Manager,
// по одной на origin). Клиент ходит по локальному порту окна (apiBase) —
// тому же, через который идут и его API-запросы с заголовком Authorization;
// CORS порта разрешает только страницу окна. В браузере моста нет — клиент
// продолжает держать сессии в IndexedDB, как раньше.

// secretAdminKey — зарезервированный ключ панели администратора: origin,
// равный ему, у сервера быть не может, конфликтов нет.
const secretAdminKey = "admin"

// secretBlobLimit — потолок сессии: Credential Manager принимает блобы до
// ~1 КБ (пробник 22.11: 1024 — да, 4096 — отказ RPC); JSON сессии вместе с
// токеном в него помещается, превышение — отказ хранилища, а не молчаливая
// потеря хвоста.
const secretBlobLimit = 1024

// SecretStore — хранилище секретов окна. Blob — JSON сессии целиком
// (origin, имя, почта, токен, signed_in_at): по учётке на origin.
type SecretStore interface {
	Get(origin string) ([]byte, bool)
	Put(origin string, blob []byte) error
	Delete(origin string) bool
	List() []string
}

// SecretsCache — хранилище с кэшем процесса: Credential Manager читается на
// первом касании, дальше память. Окно единственный писатель своего хранилища.
type SecretsCache struct {
	mu    sync.Mutex
	store SecretStore
	bag   map[string][]byte
}

func NewSecretsCache(store SecretStore) *SecretsCache {
	return &SecretsCache{store: store}
}

func (c *SecretsCache) load() map[string][]byte {
	if c.bag != nil {
		return c.bag
	}
	bag := make(map[string][]byte)
	for _, origin := range c.store.List() {
		if blob, ok := c.store.Get(origin); ok {
			bag[origin] = blob
		}
	}
	c.bag = bag
	return bag
}

// Get возвращает сессию origin или ключ admin.
func (c *SecretsCache) Get(origin string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	blob, ok := c.load()[origin]
	return blob, ok
}

// Put сохраняет сессию; сбой хранилища не портит кэш.
func (c *SecretsCache) Put(origin string, blob []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.store.Put(origin, blob); err != nil {
		return err
	}
	c.load()[origin] = append([]byte(nil), blob...)
	return nil
}

// Delete убирает сессию; отсутствующая — не ошибка.
func (c *SecretsCache) Delete(origin string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store.Delete(origin)
	delete(c.load(), origin)
}

func (c *SecretsCache) snapshot() map[string][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string][]byte, len(c.load()))
	for k, v := range c.load() {
		out[k] = v
	}
	return out
}

// SessionsHandler — HTTP-ручки сессий окна на локальном порту:
//
//	GET    /__secrets          → весь мешок: {"sessions":[...], "admin":{…}|null}
//	PUT    /__secrets/session  → SessionRecord (origin внутри) — сохранить
//	PUT    /__secrets/admin    → AdminSessionRecord — сохранить
//	DELETE /__secrets/session/<origin>
//	DELETE /__secrets/admin
func SessionsHandler(store *SecretsCache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/__secrets")
		switch {
		case path == "" || path == "/":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			sessions := []json.RawMessage{}
			for origin, blob := range store.snapshot() {
				if origin == secretAdminKey {
					continue
				}
				sessions = append(sessions, json.RawMessage(blob))
			}
			var admin json.RawMessage
			if blob, ok := store.Get(secretAdminKey); ok {
				admin = json.RawMessage(blob)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"sessions": sessions, "admin": admin})
		case path == "/session":
			if r.Method != http.MethodPut {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			body, ok := secretBody(w, r)
			if !ok {
				return
			}
			var rec struct {
				Origin string `json:"origin"`
			}
			if json.Unmarshal(body, &rec) != nil || rec.Origin == "" || rec.Origin == secretAdminKey {
				http.Error(w, "bad origin", http.StatusBadRequest)
				return
			}
			if err := store.Put(rec.Origin, body); err != nil {
				http.Error(w, "store failed", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case path == "/admin":
			switch r.Method {
			case http.MethodPut:
				body, ok := secretBody(w, r)
				if !ok {
					return
				}
				if err := store.Put(secretAdminKey, body); err != nil {
					http.Error(w, "store failed", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			case http.MethodDelete:
				store.Delete(secretAdminKey)
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		case strings.HasPrefix(path, "/session/"):
			if r.Method != http.MethodDelete {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			origin, err := url.PathUnescape(strings.TrimPrefix(path, "/session/"))
			if err != nil || origin == "" || origin == secretAdminKey {
				http.Error(w, "bad origin", http.StatusBadRequest)
				return
			}
			store.Delete(origin)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
}

// secretBody читает тело сессии с потолком secretBlobLimit: больше в
// учётную запись не влезает — отказ сразу, а не хвост молча.
func secretBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, secretBlobLimit+1))
	if err != nil || len(body) == 0 || len(body) > secretBlobLimit {
		http.Error(w, "bad body", http.StatusBadRequest)
		return nil, false
	}
	return body, true
}
