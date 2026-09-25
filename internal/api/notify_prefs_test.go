package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// Инвариант: упоминания всегда включены — выключив всё остальное,
// участник всё равно узнает, что его позвали.
func TestNotifyPrefsMentionsAlwaysOn(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "prefs@example.com")

	body, _ := json.Marshal(map[string]bool{"posts": false})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notify_prefs", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("prefs: %d %s", rec.Code, rec.Body.String())
	}
	var res map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if res["mentions"] != true {
		t.Fatalf("mentions must stay on: %v", res["mentions"])
	}
}
