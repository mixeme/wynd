package api_test

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// waitPayPush ждёт один запрос к фейковому push-сервису.
func waitPayPush(t *testing.T, hits chan []byte) {
	t.Helper()
	select {
	case body := <-hits:
		if len(body) == 0 {
			t.Fatal("пустое тело пуша")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("пуш не доставлен")
	}
}

// Решения по оплате доходят до участника пушем: утверждение заявки, отказ и
// выдача без заявки. Тело пуша зашифровано — проверяется сама доставка;
// тексты писем и сигналов покрыты тестами mail и журналом стенда.
func TestPayDecisionsPushParticipant(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)

	hits := make(chan []byte, 8)
	ep := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		hits <- body
		w.WriteHeader(http.StatusCreated)
	}))
	defer ep.Close()
	srv.Push.AllowLoopbackDeliveryForTest()

	token, admin, requestID, _ := expiredWithRequest(t, srv, caps, "ana@example.com")

	// /push/subscribe закрыт неплатившему (paid) и не принимает loopback;
	// подписка осталась от оплаченных месяцев — вставляем её как сохранившуюся.
	var accountID string
	if err := srv.Auth.DB().QueryRowContext(t.Context(),
		`SELECT id FROM accounts WHERE email = ?`, "ana@example.com").Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Push.DB().ExecContext(t.Context(), `
		INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, user_agent, created_at)
		VALUES ('pay-notify-test', ?, ?, ?, ?, '', ?)
	`, accountID, ep.URL+"/ep",
		base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		base64.RawURLEncoding.EncodeToString(secret),
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/pay/requests/"+requestID+"/approve", admin, map[string]any{"days": 30})
	if rec.Code != http.StatusOK {
		t.Fatalf("утверждение: %d %s", rec.Code, rec.Body.String())
	}
	waitPayPush(t, hits)

	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/pay/accounts/"+accountID, admin, map[string]any{"days": 7})
	if rec.Code != http.StatusOK {
		t.Fatalf("выдача без заявки: %d %s", rec.Code, rec.Body.String())
	}
	waitPayPush(t, hits)

	blob3 := uploadBlob(t, srv, token, []byte("screenshot-3"), "image/png")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/pay/requests", token, map[string]any{"blob_id": blob3})
	if rec.Code != http.StatusCreated {
		t.Fatalf("вторая заявка: %d %s", rec.Code, rec.Body.String())
	}
	third := jsonStr(t, rec, "id")
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/pay/requests/"+third+"/reject", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("отказ: %d %s", rec.Code, rec.Body.String())
	}
	waitPayPush(t, hits)

	select {
	case extra := <-hits:
		t.Fatalf("лишний пуш: %d байт", len(extra))
	case <-time.After(300 * time.Millisecond):
	}
}
