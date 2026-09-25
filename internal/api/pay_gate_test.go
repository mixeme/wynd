package api_test

import (
	"net/http"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

// enableSubscription включает обязательную подписку с реквизитами: с этого
// момента участник без срока — истёкший.
func enableSubscription(t *testing.T, srv *api.Server, admin string) {
	t.Helper()
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/pay", admin, map[string]any{"requisites": "Карта 0000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("requisites: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/pay/subscription", admin, map[string]any{
		"required": true, "remind_days": 7,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("enable subscription: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант (аудит 2026-09-22, план 42 волна 4): шлюз оплаты закрывает
// журнал, но оставляет открытыми выход, статус оплаты, загрузку и подачу
// заявки. Раньше /uploads* стояли под paid, и истёкший участник не мог
// приложить скриншот — включение подписки запирало всех до ручного продления.
func TestPaidGateBlocksJournalButNotPayRoutes(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "ana@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", token, map[string]any{
		"name": "Семья", "owner_name": "Аня",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	circleID := jsonStr(t, rec, "id")
	admin := adminToken(t, srv)
	enableSubscription(t, srv, admin)

	rec = doGET(t, srv, "/api/v1/pay/status", token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"expired":true`) {
		t.Fatalf("pay/status: %d %s", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"/api/v1/circles", "/api/v1/circles/" + circleID + "/feed", "/api/v1/sync"} {
		rec = doGET(t, srv, path, token)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "payment_required") {
			t.Fatalf("%s for expired: %d %s", path, rec.Code, rec.Body.String())
		}
	}

	// Скриншот загружается и заявка подаётся.
	blobID := uploadBlob(t, srv, token, []byte("screenshot"), "image/png")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/pay/requests", token, map[string]any{
		"blob_id": blobID, "comment": "оплатил",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("pay request: %d %s", rec.Code, rec.Body.String())
	}
	requestID := jsonStr(t, rec, "id")

	// Вторая ожидающая заявка — конфликт.
	blob2 := uploadBlob(t, srv, token, []byte("screenshot-2"), "image/png")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/pay/requests", token, map[string]any{"blob_id": blob2})
	if rec.Code != http.StatusConflict {
		t.Fatalf("second pending: %d %s", rec.Code, rec.Body.String())
	}

	// Блоб без оплаты в круг не уходит: запись под paid.
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/circles/"+circleID+"/posts", token, map[string]any{
		"body": "", "entry_date": "2026-08-30", "media": []map[string]any{{"blob_id": blob2, "kind": "photo"}},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("post for expired: %d %s", rec.Code, rec.Body.String())
	}

	// Админ видит скриншот как attachment с экранированным именем.
	rec = doGET(t, srv, "/api/v1/admin/pay/blob/"+blobID, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin blob: %d %s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	// Approve открывает журнал; повторный approve — 404, срок не удваивается.
	approve := func() int {
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/pay/requests/"+requestID+"/approve", admin, map[string]any{"days": 30})
		return rec.Code
	}
	if code := approve(); code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("approve: %d", code)
	}
	rec = doGET(t, srv, "/api/v1/pay/status", token)
	expires := jsonStr(t, rec, "expires_at")
	if code := approve(); code != http.StatusNotFound {
		t.Fatalf("second approve: %d", code)
	}
	rec = doGET(t, srv, "/api/v1/pay/status", token)
	if got := jsonStr(t, rec, "expires_at"); got != expires {
		t.Fatalf("повторный approve продлил срок: %s → %s", expires, got)
	}
	rec = doGET(t, srv, "/api/v1/circles", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("circles after approve: %d %s", rec.Code, rec.Body.String())
	}

	// Слишком длинный комментарий и слишком большой срок отвергаются.
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/pay/requests", token, map[string]any{
		"blob_id": blob2, "comment": strings.Repeat("я", 2001),
	})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too_long") {
		t.Fatalf("long comment: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/pay", admin, map[string]any{"requisites": strings.Repeat("я", 4001)})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("long requisites: %d %s", rec.Code, rec.Body.String())
	}
}
