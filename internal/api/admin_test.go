package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

func adminToken(t *testing.T, srv *api.Server) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"password": "admin-pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login: %d %s", rec.Code, rec.Body.String())
	}
	var res map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	return res["token"]
}

func TestAdminStorageAndCheck(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	token := adminToken(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("storage: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("check: %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	checks, ok := body["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("checks: %v", body["checks"])
	}
}

func TestAdminProxySnippet(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	token := adminToken(t, srv)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/proxy/nginx", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("proxy: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminBlockAccountAndStorageCircles(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	userTok, accountID := registerSession(t, srv, caps, "ana@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", userTok, map[string]any{
		"name": "Семья", "owner_name": "Аня", "color": "olive",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}

	admin := adminToken(t, srv)
	rec = doGET(t, srv, "/api/v1/admin/storage", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("storage: %d %s", rec.Code, rec.Body.String())
	}
	var storage map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&storage); err != nil {
		t.Fatal(err)
	}
	circles, ok := storage["circles"].([]any)
	if !ok || len(circles) != 1 {
		t.Fatalf("circles: %v", storage["circles"])
	}
	row, _ := circles[0].(map[string]any)
	if row["name"] != "Семья" || row["color"] != "olive" {
		t.Fatalf("circle row: %v", row)
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/accounts/"+accountID+"/block", admin, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("block: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles", userTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("blocked session: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/admin/accounts", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("accounts: %d %s", rec.Code, rec.Body.String())
	}
	var list map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	accounts, _ := list["accounts"].([]any)
	found := false
	for _, raw := range accounts {
		acc, _ := raw.(map[string]any)
		if acc["id"] == accountID {
			found = true
			if acc["blocked"] != true {
				t.Fatalf("blocked flag: %v", acc)
			}
		}
	}
	if !found {
		t.Fatalf("account missing: %v", list)
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/accounts/"+accountID+"/unblock", admin, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("unblock: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSMTPTestNotConfigured(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	token := adminToken(t, srv)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/smtp/test", token, map[string]string{
		"to": "ana@example.com",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "error") != "smtp_not_configured" {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

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
