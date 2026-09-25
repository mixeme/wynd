package api_test

import (
	"net/http"
	"testing"
)

// Аудит 2026-09-22: настройки уведомлений круга заводятся только для своего
// круга — иначе участник насыпал строк по произвольным circle_id.
func TestCircleNotifyPrefsRequireMembership(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	ownerTok, _ := registerSession(t, srv, caps, "owner@example.com")
	strangerTok, _ := registerSession(t, srv, caps, "stranger@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", ownerTok, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	body := map[string]any{"posts": false}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/notify_prefs", strangerTok, body)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("stranger prefs: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/no-such-circle/notify_prefs", ownerTok, body)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("unknown circle prefs: %d %s", rec.Code, rec.Body.String())
	}
	var rows int
	if err := srv.Chronicle.DB().QueryRowContext(t.Context(), `SELECT count(*) FROM circle_notify_prefs`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("строки настроек чужого круга созданы: %d", rows)
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/circles/"+circleID+"/notify_prefs", ownerTok, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("own prefs: %d %s", rec.Code, rec.Body.String())
	}
}
