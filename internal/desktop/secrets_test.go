package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type memStore struct {
	data map[string][]byte
}

func newMemStore() *memStore { return &memStore{data: map[string][]byte{}} }

func (m *memStore) Get(origin string) ([]byte, bool) {
	blob, ok := m.data[origin]
	return blob, ok
}

func (m *memStore) Put(origin string, blob []byte) error {
	m.data[origin] = append([]byte(nil), blob...)
	return nil
}

func (m *memStore) Delete(origin string) bool {
	_, ok := m.data[origin]
	delete(m.data, origin)
	return ok
}

func (m *memStore) List() []string {
	out := make([]string, 0, len(m.data))
	for k := range m.data {
		out = append(out, k)
	}
	return out
}

func doSecret(t *testing.T, store *SecretsCache, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	SessionsHandler(store).ServeHTTP(rec, req)
	return rec
}

func TestSecretsSessionsBag(t *testing.T) {
	store := NewSecretsCache(newMemStore())
	session := `{"origin":"https://home.example.org","name":"Аня","email":"a@x.ru","token":"tok","account_id":"acc1"}`
	if rec := doSecret(t, store, http.MethodPut, "/__secrets/session", session); rec.Code != http.StatusNoContent {
		t.Fatalf("put session: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doSecret(t, store, http.MethodPut, "/__secrets/admin", `{"origin":"","token":"adm"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("put admin: %d %s", rec.Code, rec.Body.String())
	}

	rec := doSecret(t, store, http.MethodGet, "/__secrets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bag: %d %s", rec.Code, rec.Body.String())
	}
	var bag struct {
		Sessions []json.RawMessage `json:"sessions"`
		Admin    json.RawMessage   `json:"admin"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &bag) != nil {
		t.Fatalf("bag json: %s", rec.Body.String())
	}
	if len(bag.Sessions) != 1 || !strings.Contains(string(bag.Sessions[0]), "home.example.org") {
		t.Fatalf("сессии в мешке: %s", rec.Body.String())
	}
	if bag.Admin == nil || !strings.Contains(string(bag.Admin), "adm") {
		t.Fatalf("админ-сессия в мешке: %s", rec.Body.String())
	}

	// Один и тот же ключ — перезапись, а не дубль.
	if rec := doSecret(t, store, http.MethodPut, "/__secrets/session", session); rec.Code != http.StatusNoContent {
		t.Fatalf("re-put: %d", rec.Code)
	}
	if got := store.snapshot(); len(got) != 2 {
		t.Fatalf("дубль ключа: %v", keysOf(got))
	}

	// Удаление по origin и панели; отсутствующее — не ошибка.
	if rec := doSecret(t, store, http.MethodDelete, "/__secrets/session/https%3A%2F%2Fhome.example.org", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete session: %d", rec.Code)
	}
	if rec := doSecret(t, store, http.MethodDelete, "/__secrets/admin", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete admin: %d", rec.Code)
	}
	if rec := doSecret(t, store, http.MethodDelete, "/__secrets/session/https%3A%2F%2Fgone.example.org", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete gone: %d", rec.Code)
	}
	if got := store.snapshot(); len(got) != 0 {
		t.Fatalf("мешок не пуст: %v", keysOf(got))
	}
}

// Зарезервированный ключ панели не должен оказаться чьим-то origin.
func TestSecretsAdminKeyReserved(t *testing.T) {
	store := NewSecretsCache(newMemStore())
	if rec := doSecret(t, store, http.MethodPut, "/__secrets/session", `{"origin":"admin","token":"x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("origin admin принят: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doSecret(t, store, http.MethodDelete, "/__secrets/session/admin", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete origin admin принят: %d", rec.Code)
	}
}

// Сессия больше учётной записи Windows — отказ, не молчаливая потеря хвоста.
func TestSecretsBlobLimit(t *testing.T) {
	store := NewSecretsCache(newMemStore())
	big := `{"origin":"https://home.example.org","token":"` + strings.Repeat("w", 1200) + `"}`
	if rec := doSecret(t, store, http.MethodPut, "/__secrets/session", big); rec.Code != http.StatusBadRequest {
		t.Fatalf("большая сессия принята: %d", rec.Code)
	}
}

// Путь сессии с чужим методом и мусорный путь — не ручки.
func TestSecretsRejectsWrongPaths(t *testing.T) {
	store := NewSecretsCache(newMemStore())
	if rec := doSecret(t, store, http.MethodPost, "/__secrets/session", "{}"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST session: %d", rec.Code)
	}
	if rec := doSecret(t, store, http.MethodGet, "/__secrets/unknown", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path: %d", rec.Code)
	}
}

func keysOf(bag map[string][]byte) []string {
	out := make([]string, 0, len(bag))
	for k := range bag {
		out = append(out, k)
	}
	return out
}
