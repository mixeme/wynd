package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Инвариант (аудит 2026-09-22): account_id соседа — учётка на сервере, а не
// лицо в круге. Её видят только те, кому она нужна для действий: владелец и
// участник с правом на настройки.
func TestCircleMembersHideAccountIDFromOrdinaryMember(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, memberTok, circleID, memberID := circleWithMember(t, srv, caps)

	hasAccountIDs := func(token string) bool {
		t.Helper()
		rec := doGET(t, srv, "/api/v1/circles/"+circleID+"/members", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Members []map[string]any `json:"members"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Members) != 2 {
			t.Fatalf("участников: %d", len(out.Members))
		}
		seen := false
		for _, m := range out.Members {
			if _, ok := m["account_id"]; ok {
				seen = true
			}
			// Лицо и имя нужны всем — они и остаются.
			if m["identity_id"] == nil || m["name"] == nil {
				t.Fatalf("участник без лица или имени: %+v", m)
			}
		}
		return seen
	}

	if !hasAccountIDs(ownerTok) {
		t.Fatal("владелец не получил account_id — ему нечем распоряжаться")
	}
	if hasAccountIDs(memberTok) {
		t.Fatal("обычный участник получил учётки соседей")
	}

	// Право на настройки открывает учётки: с ним исключают и передают.
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/members/"+memberID, ownerTok, map[string]any{
		"can_settings": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("права: %d %s", rec.Code, rec.Body.String())
	}
	if !hasAccountIDs(memberTok) {
		t.Fatal("участник с правом на настройки не получил account_id")
	}
}

// Инвариант (аудит 2026-09-22): у записи есть потолок числа вложений.
func TestPostMediaCapped(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _, circleID, _ := circleWithMember(t, srv, caps)

	media := make([]map[string]any, 51)
	for i := range media {
		media[i] = map[string]any{"blob_id": "b", "kind": "photo"}
	}
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "много", "entry_date": "2026-08-30", "media": media,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("51 вложение: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант (аудит 2026-09-22): отрицательное окно правок отвергается и при
// создании круга, а не только в настройках.
func TestCreateCircleRejectsNegativeEditWindow(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "ana@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", tok, map[string]any{
		"name": "Семья", "owner_name": "Аня", "edit_window_sec": -1,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("отрицательное окно: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles", tok)
	if strings.Contains(rec.Body.String(), "Семья") {
		t.Fatalf("круг всё-таки создан: %s", rec.Body.String())
	}
}

// Инвариант (аудит 2026-09-22): название дня приходит обрезанным, из одних
// пробелов — не название.
func TestDayTitleTrimmed(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _, circleID, _ := circleWithMember(t, srv, caps)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", ownerTok, map[string]any{
		"body": "запись", "entry_date": "2026-08-30",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("запись: %d %s", rec.Code, rec.Body.String())
	}
	titlePath := "/api/v1/circles/" + circleID + "/days/2026-08-30/title"

	if rec := doJSON(t, srv, http.MethodPut, titlePath, ownerTok, map[string]any{"title": "   "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("название из пробелов: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, srv, http.MethodPut, titlePath, ownerTok, map[string]any{"title": "  Купание  "}); rec.Code != http.StatusOK {
		t.Fatalf("название с пробелами: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles/"+circleID+"/days", ownerTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("дни: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"Купание"`) {
		t.Fatalf("название не обрезано: %s", rec.Body.String())
	}
}

// Инвариант (аудит 2026-09-22): mute_until хранится разобранным. Строка, в
// которой времени нет, молча означала «не заглушено».
func TestNotifyPrefsMuteUntilMustBeTime(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, _ := registerSession(t, srv, caps, "ana@example.com")

	rec := doJSON(t, srv, http.MethodPut, "/api/v1/notify_prefs", tok, map[string]any{
		"mute_until": "завтра",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("непонятное время: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/notify_prefs", tok, map[string]any{
		"mute_until": "2026-09-30T10:00:00+03:00",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("время по RFC3339: %d %s", rec.Code, rec.Body.String())
	}
	if got := jsonStr(t, rec, "mute_until"); got != "2026-09-30T07:00:00Z" {
		t.Fatalf("mute_until сохранён как %q", got)
	}
	// Пустая строка снимает заглушку.
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/notify_prefs", tok, map[string]any{
		"mute_until": "",
	})
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "mute_until") {
		t.Fatalf("снятие заглушки: %d %s", rec.Code, rec.Body.String())
	}
}
