package api_test

import (
	"net/http"
	"testing"
)

func TestAdminPayAccountsGatedWhenSubscriptionOff(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	_, accountID := registerSession(t, srv, caps, "ana@example.com")
	admin := adminToken(t, srv)

	rec := doGET(t, srv, "/api/v1/admin/pay/accounts", admin)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("list off: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/admin/pay/accounts/"+accountID, admin)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("by id off: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/pay/accounts/"+accountID, admin, map[string]any{
		"days": 30,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("grant off: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/pay/subscription", admin, map[string]any{
		"required": true, "remind_days": 7,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("enable subscription: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/admin/pay/accounts", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list on: %d %s", rec.Code, rec.Body.String())
	}
}
