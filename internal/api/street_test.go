package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Инвариант (план 46, A2): закрепления и группы кругов — у учётки на
// сервере: второе устройство той же почтой видит их в списке кругов.
func TestStreetLayoutFollowsAccount(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "street@example.com")
	newCircle := func(name string) string {
		t.Helper()
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{"name": name, "owner_name": "Аня"})
		if rec.Code != http.StatusCreated {
			t.Fatalf("circle: %d %s", rec.Code, rec.Body.String())
		}
		return jsonStr(t, rec, "id")
	}
	dacha, work := newCircle("Дача"), newCircle("Работа")
	ok := func(method, path string, body any) {
		t.Helper()
		if rec := doJSON(t, srv, method, path, tok, body); rec.Code != http.StatusOK {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
	}
	ok(http.MethodPut, "/api/v1/circles/"+dacha+"/pin", map[string]any{"pinned_at": "2026-10-01T10:00:00Z"})
	ok(http.MethodPut, "/api/v1/circles/"+dacha+"/group", map[string]any{"name": "Семья"})
	ok(http.MethodPut, "/api/v1/circles/"+work+"/group", map[string]any{"name": "Дела"})
	ok(http.MethodPost, "/api/v1/groups/rename", map[string]any{"name": "Семья", "new_name": "Родные"})

	type list struct {
		Circles []struct {
			ID       string `json:"id"`
			PinnedAt string `json:"pinned_at"`
		} `json:"circles"`
		Groups []struct {
			Name      string   `json:"name"`
			CircleIDs []string `json:"circle_ids"`
		} `json:"groups"`
	}
	read := func(token string) list {
		t.Helper()
		var out list
		rec := doJSON(t, srv, http.MethodGet, "/api/v1/circles", token, nil)
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("list: %v %s", err, rec.Body.String())
		}
		return out
	}
	tok2, _ := registerSession(t, srv, caps, "street@example.com")
	got := read(tok2)
	pinned := map[string]string{}
	for _, c := range got.Circles {
		pinned[c.ID] = c.PinnedAt
	}
	if pinned[dacha] != "2026-10-01T10:00:00Z" || pinned[work] != "" {
		t.Fatalf("закрепления на втором устройстве: %v", pinned)
	}
	if len(got.Groups) != 2 || got.Groups[0].Name != "Дела" || got.Groups[1].Name != "Родные" ||
		got.Groups[1].CircleIDs[0] != dacha {
		t.Fatalf("группы на втором устройстве: %+v", got.Groups)
	}

	ok(http.MethodDelete, "/api/v1/circles/"+dacha+"/pin", nil)
	ok(http.MethodDelete, "/api/v1/circles/"+dacha+"/group", nil)
	ok(http.MethodPost, "/api/v1/groups/delete", map[string]any{"name": "Дела"})
	got = read(tok)
	if len(got.Groups) != 0 || got.Circles[0].PinnedAt != "" {
		t.Fatalf("после снятия: %+v", got)
	}

	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+dacha+"/group", tok, map[string]any{"name": " "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("пустое название: %d", rec.Code)
	}
	other, _ := registerSession(t, srv, caps, "other@example.com")
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+dacha+"/pin", other, map[string]any{}); rec.Code != http.StatusNotFound {
		t.Fatalf("чужой круг: %d", rec.Code)
	}
}
