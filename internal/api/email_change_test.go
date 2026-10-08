package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
)

func sendJSON(t *testing.T, srv *api.Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("X-Forwarded-For", "203.0.113.77")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func circlesStatus(t *testing.T, srv *api.Server, token string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/circles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec.Code
}

// Смена почты самим участником (кадры 7.13–7.14): код уходит на новую почту,
// до ввода кода ничего не меняется; после — это устройство вошло, остальные
// нет, а прежний адрес на сервере больше не известен.
func TestEmailChangeByParticipant(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	here, accountID := registerSession(t, srv, caps, "anya@example.com")
	there, _ := registerSession(t, srv, caps, "anya@example.com")

	rec := sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email", here, map[string]string{"email": "Anya@NewMail.example"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	code := caps.Last("anya@newmail.example")
	if code == "" {
		t.Fatal("код на новую почту не ушёл")
	}
	// Код смены сессию не даёт: обычный вход его не видит.
	rec = sendJSON(t, srv, http.MethodPost, "/api/v1/auth/verify", "", map[string]string{"email": "anya@newmail.example", "code": code})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("вход кодом смены: %d %s", rec.Code, rec.Body.String())
	}
	if got := circlesStatus(t, srv, there); got != http.StatusOK {
		t.Fatalf("до ввода кода второе устройство должно работать: %d", got)
	}

	rec = sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email/verify", here, map[string]string{"code": code})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["email"] != "anya@newmail.example" {
		t.Fatalf("email: %q", out["email"])
	}
	if got := circlesStatus(t, srv, here); got != http.StatusOK {
		t.Fatalf("это устройство должно остаться вошедшим: %d", got)
	}
	if got := circlesStatus(t, srv, there); got != http.StatusForbidden {
		t.Fatalf("другое устройство должно выйти: %d", got)
	}
	acc, err := srv.Auth.AccountByID(t.Context(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Email != "anya@newmail.example" {
		t.Fatalf("почта учётки: %q", acc.Email)
	}
	// Код одноразовый.
	rec = sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email/verify", here, map[string]string{"code": code})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("повтор кода: %d %s", rec.Code, rec.Body.String())
	}
	// Вход новой почтой — та же учётка.
	_, again := registerSession(t, srv, caps, "anya@newmail.example")
	if again != accountID {
		t.Fatalf("новая почта привела в другую учётку: %s != %s", again, accountID)
	}
}

func TestEmailChangeRefusals(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	anya, _ := registerSession(t, srv, caps, "anya@example.com")
	registerSession(t, srv, caps, "mama@example.com")

	for _, c := range []struct {
		name  string
		email string
		want  int
	}{
		{"занятый адрес", "mama@example.com", http.StatusConflict},
		{"свой же адрес", "ANYA@example.com", http.StatusBadRequest},
		{"не адрес", "anya", http.StatusBadRequest},
		{"адрес панели", "admin@wynd.local", http.StatusBadRequest},
	} {
		rec := sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email", anya, map[string]string{"email": c.email})
		if rec.Code != c.want {
			t.Fatalf("%s: %d %s", c.name, rec.Code, rec.Body.String())
		}
	}
	rec := sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email", "", map[string]string{"email": "x@example.com"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("без сессии: %d", rec.Code)
	}

	// Три попытки, как у входа: после трёх неверных кодов верный не подходит.
	rec = sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email", anya, map[string]string{"email": "anya@newmail.example"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	code := caps.Last("anya@newmail.example")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < 3; i++ {
		rec = sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email/verify", anya, map[string]string{"code": wrong})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("неверный код %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec = sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email/verify", anya, map[string]string{"code": code})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("после трёх попыток: %d %s", rec.Code, rec.Body.String())
	}
}

// Адрес заняли, пока код шёл: смена отказывает, а не сливает два входа.
func TestEmailChangeAddressTakenMeanwhile(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	anya, _ := registerSession(t, srv, caps, "anya@example.com")
	rec := sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email", anya, map[string]string{"email": "new@example.com"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	code := caps.Last("new@example.com")
	if _, err := srv.Auth.DB().Exec(
		`INSERT INTO accounts (id, email, created_at) VALUES ('other', 'new@example.com', '2026-08-30T09:00:00.000000000Z')`,
	); err != nil {
		t.Fatal(err)
	}
	rec = sendJSON(t, srv, http.MethodPost, "/api/v1/auth/email/verify", anya, map[string]string{"code": code})
	if rec.Code != http.StatusConflict {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body.String())
	}
}

// Запасной путь (кадр 9.11): администратор меняет без кода, все устройства
// человека выходят, вход — кодом на новую почту.
func TestAdminSetAccountEmail(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	tok, accountID := registerSession(t, srv, caps, "anya@example.com")
	registerSession(t, srv, caps, "mama@example.com")
	admin := adminToken(t, srv)
	path := "/api/v1/admin/accounts/" + accountID + "/email"

	rec := sendJSON(t, srv, http.MethodPut, path, admin, map[string]string{"email": "mama@example.com"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("занятый адрес: %d %s", rec.Code, rec.Body.String())
	}
	rec = sendJSON(t, srv, http.MethodPut, path, tok, map[string]string{"email": "x@example.com"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("участник в панели: %d", rec.Code)
	}
	rec = sendJSON(t, srv, http.MethodPut, path, admin, map[string]string{"email": "anya@newmail.example"})
	if rec.Code != http.StatusOK {
		t.Fatalf("смена: %d %s", rec.Code, rec.Body.String())
	}
	if got := circlesStatus(t, srv, tok); got != http.StatusForbidden {
		t.Fatalf("устройства человека должны выйти: %d", got)
	}
	_, again := registerSession(t, srv, caps, "anya@newmail.example")
	if again != accountID {
		t.Fatalf("новая почта привела в другую учётку: %s != %s", again, accountID)
	}
	rec = sendJSON(t, srv, http.MethodPut, "/api/v1/admin/accounts/nope/email", admin, map[string]string{"email": "y@example.com"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("нет учётки: %d", rec.Code)
	}
}
