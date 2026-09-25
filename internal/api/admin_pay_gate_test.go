package api_test

import (
	"net/http"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// expiredWithRequest: включённая подписка, истёкший участник и его поданная
// заявка со скриншотом.
func expiredWithRequest(t *testing.T, srv *api.Server, caps *auth.CaptureCodes, email string) (token, admin, requestID, blobID string) {
	t.Helper()
	token, _ = registerSession(t, srv, caps, email)
	admin = adminToken(t, srv)
	enableSubscription(t, srv, admin)
	blobID = uploadBlob(t, srv, token, []byte("screenshot-"+email), "image/png")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/pay/requests", token, map[string]any{
		"blob_id": blobID, "comment": "оплатил",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("заявка: %d %s", rec.Code, rec.Body.String())
	}
	return token, admin, jsonStr(t, rec, "id"), blobID
}

// Инвариант: отказ по заявке ничего не открывает. Approve даёт срок и
// журнал, reject — оставляет участника истёкшим; повторное решение по уже
// решённой заявке не проходит.
func TestAdminPayApproveGrantsAccessRejectDoesNot(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, admin, requestID, _ := expiredWithRequest(t, srv, caps, "ana@example.com")

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/pay/requests/"+requestID+"/reject", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("отказ: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/pay/status", token)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"expired":true`) {
		t.Fatalf("после отказа участник не истёкший: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doGET(t, srv, "/api/v1/circles", token); rec.Code != http.StatusForbidden {
		t.Fatalf("отказ открыл журнал: %d %s", rec.Code, rec.Body.String())
	}
	// Отклонённую заявку нельзя утвердить задним числом.
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/pay/requests/"+requestID+"/approve", admin, map[string]any{"days": 30})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("утверждение отклонённой заявки: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doGET(t, srv, "/api/v1/circles", token); rec.Code != http.StatusForbidden {
		t.Fatalf("журнал открылся после отказа: %d %s", rec.Code, rec.Body.String())
	}

	// Новая заявка того же участника утверждается и открывает журнал.
	blob2 := uploadBlob(t, srv, token, []byte("screenshot-2"), "image/png")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/pay/requests", token, map[string]any{"blob_id": blob2})
	if rec.Code != http.StatusCreated {
		t.Fatalf("вторая заявка после отказа: %d %s", rec.Code, rec.Body.String())
	}
	second := jsonStr(t, rec, "id")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/pay/requests/"+second+"/approve", admin, map[string]any{"days": 30})
	if rec.Code != http.StatusOK {
		t.Fatalf("утверждение: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/pay/status", token)
	if strings.Contains(rec.Body.String(), `"expired":true`) {
		t.Fatalf("после утверждения участник истёкший: %s", rec.Body.String())
	}
	if rec := doGET(t, srv, "/api/v1/circles", token); rec.Code != http.StatusOK {
		t.Fatalf("журнал не открылся: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант: маршруты оплаты в панели — только для админской сессии, а
// /admin/pay/blob отдаёт лишь скриншоты заявок. Участник с любым токеном
// сюда не проходит, и чужой блоб через панель не достать.
func TestAdminPayRoutesRequireAdminAndBlobScoped(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, admin, requestID, screenshot := expiredWithRequest(t, srv, caps, "ana@example.com")

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/pay"},
		{http.MethodPut, "/api/v1/admin/pay"},
		{http.MethodGet, "/api/v1/admin/pay/donate"},
		{http.MethodPut, "/api/v1/admin/pay/donate"},
		{http.MethodGet, "/api/v1/admin/pay/subscription"},
		{http.MethodPut, "/api/v1/admin/pay/subscription"},
		{http.MethodGet, "/api/v1/admin/pay/requests"},
		{http.MethodGet, "/api/v1/admin/pay/requests/" + requestID},
		{http.MethodPost, "/api/v1/admin/pay/requests/" + requestID + "/approve"},
		{http.MethodPost, "/api/v1/admin/pay/requests/" + requestID + "/reject"},
		{http.MethodGet, "/api/v1/admin/pay/blob/" + screenshot},
		{http.MethodGet, "/api/v1/admin/pay/accounts"},
		{http.MethodGet, "/api/v1/admin/pay/accounts/x"},
		{http.MethodPut, "/api/v1/admin/pay/accounts/x"},
	}
	for _, rt := range routes {
		// Ни без токена, ни с участниковым — панель это отдельная сессия.
		for name, tok := range map[string]string{"без токена": "", "участник": token} {
			rec := doJSON(t, srv, rt.method, rt.path, tok, map[string]any{})
			if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s (%s): %d %s", rt.method, rt.path, name, rec.Code, rec.Body.String())
			}
		}
	}

	// Заявка осталась ожидающей: ни одно из отклонённых обращений не прошло.
	rec := doGET(t, srv, "/api/v1/admin/pay/requests/"+requestID, admin)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"pending"`) {
		t.Fatalf("заявка изменилась: %d %s", rec.Code, rec.Body.String())
	}

	// Блоб заявки админу доступен, любой другой блоб того же участника — нет.
	if rec := doGET(t, srv, "/api/v1/admin/pay/blob/"+screenshot, admin); rec.Code != http.StatusOK {
		t.Fatalf("скриншот заявки: %d %s", rec.Code, rec.Body.String())
	}
	other := uploadBlob(t, srv, token, []byte("private-photo"), "image/jpeg")
	if rec := doGET(t, srv, "/api/v1/admin/pay/blob/"+other, admin); rec.Code != http.StatusForbidden {
		t.Fatalf("чужой блоб через панель оплаты: %d %s", rec.Code, rec.Body.String())
	}
}
