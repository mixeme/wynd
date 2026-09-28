package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Инвариант (план 43, B3): «Место со снимков» — настройка участника в круге,
// хранится на сервере и приходит в карточке круга на любое устройство.
// По умолчанию включена; без поля share_place запрос отклоняется.
func TestSharePlaceFollowsAccount(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "place@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Дача", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")

	sharePlace := func(token string) bool {
		t.Helper()
		rec := doJSON(t, srv, http.MethodGet, "/api/v1/circles/"+circleID, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
		}
		var out struct {
			SharePlace *bool `json:"share_place"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.SharePlace == nil {
			t.Fatalf("share_place в карточке: %v %s", err, rec.Body.String())
		}
		return *out.SharePlace
	}

	if !sharePlace(tok) {
		t.Fatal("по умолчанию место должно быть включено")
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/place", tok, map[string]any{"share_place": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	// Вторая сессия той же учётки — как второе устройство.
	tok2, _ := registerSession(t, srv, caps, "place@example.com")
	if sharePlace(tok2) {
		t.Fatal("выключенное место не пришло на второе устройство")
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/place", tok, map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("без share_place: %d, want 400", rec.Code)
	}
}
