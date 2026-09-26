package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	for _, kind := range []string{"nginx", "caddy", "traefik", "apache"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/proxy/"+kind, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("proxy %s: %d %s", kind, rec.Code, rec.Body.String())
		}
		var body map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		snippet := body["snippet"]
		switch kind {
		case "nginx", "caddy":
			if !strings.Contains(snippet, "X-Forwarded-For") {
				t.Fatalf("%s snippet missing X-Forwarded-For: %s", kind, snippet)
			}
		case "traefik":
			if !strings.Contains(snippet, "X-Forwarded-For") {
				t.Fatalf("%s snippet missing X-Forwarded-For: %s", kind, snippet)
			}
			if !strings.Contains(snippet, "X-Forwarded-Proto") {
				t.Fatalf("%s snippet missing X-Forwarded-Proto: %s", kind, snippet)
			}
			if strings.Contains(snippet, "wynd-forwarded") {
				t.Fatalf("%s snippet must not put forwardedHeaders under http.middlewares: %s", kind, snippet)
			}
		case "apache":
			// mod_proxy дописывает адрес сам; клиентский заголовок снимается.
			for _, want := range []string{"RequestHeader unset X-Forwarded-For", "X-Forwarded-Proto \"https\"", "flushpackets=on", "LimitRequestBody"} {
				if !strings.Contains(snippet, want) {
					t.Fatalf("%s snippet missing %q: %s", kind, want, snippet)
				}
			}
		}
		if !strings.Contains(snippet, "300") {
			t.Fatalf("%s snippet missing read timeout: %s", kind, snippet)
		}
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

	// Push-подписка учётки: блокировка должна её снять (аудит 2026-09-22).
	if _, err := srv.Chronicle.DB().ExecContext(t.Context(), `
		INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, created_at)
		VALUES ('sub1', ?, 'https://push.example/x', 'k', 'a', '2026-08-30T10:00:00.000000000Z')
	`, accountID); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/accounts/"+accountID+"/block", admin, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("block: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGET(t, srv, "/api/v1/circles", userTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("blocked session: %d %s", rec.Code, rec.Body.String())
	}
	var subs int
	if err := srv.Chronicle.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM push_subscriptions WHERE account_id = ?`, accountID).Scan(&subs); err != nil {
		t.Fatal(err)
	}
	if subs != 0 {
		t.Fatalf("push-подписки заблокированного остались: %d", subs)
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

func TestAdminChangePassword(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	token := adminToken(t, srv)
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/password", token, map[string]string{
		"current": "wrong-pass", "new": "new-admin-pass",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong current: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPut, "/api/v1/admin/password", token, map[string]string{
		"current": "admin-pass", "new": "new-admin-pass",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/login", "", map[string]string{
		"password": "new-admin-pass",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login with new: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSetPublicURL(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	token := adminToken(t, srv)
	rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/access", token, map[string]any{
		"public_url": "home.example.org",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("set url: %d %s", rec.Code, rec.Body.String())
	}
	if srv.PublicURL() != "https://home.example.org" {
		t.Fatalf("public url: %s", srv.PublicURL())
	}
	if srv.Loopback() {
		t.Fatal("expected public instance")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("instance: %d %s", rec.Code, rec.Body.String())
	}
	var inst map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&inst); err != nil {
		t.Fatal(err)
	}
	if inst["loopback"] != false {
		t.Fatalf("instance loopback after public_url: %v", inst["loopback"])
	}
	rec = doJSON(t, srv, http.MethodGet, "/api/v1/admin/access", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if jsonStr(t, rec, "public_url") != "https://home.example.org" {
		t.Fatalf("get url: %s", rec.Body.String())
	}
}

func TestBootstrapSMTPTest(t *testing.T) {
	srv, _, _, _ := setupFreshAPI(t)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/bootstrap/smtp-test", "", map[string]any{
		"token": "wrong", "host": "127.0.0.1",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong token: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/bootstrap/smtp-test", "", map[string]any{
		"token": "bootstrap",
	})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "smtp_not_configured") {
		t.Fatalf("empty host: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, srv, http.MethodPost, "/api/v1/admin/bootstrap/smtp-test", "", map[string]any{
		"token": "bootstrap", "host": "127.0.0.1", "port": 1,
	})
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "smtp_failed") {
		t.Fatalf("dial fail: %d %s", rec.Code, rec.Body.String())
	}

	done, _, _, _ := setupAPI(t)
	rec = doJSON(t, done, http.MethodPost, "/api/v1/admin/bootstrap/smtp-test", "", map[string]any{
		"token": "bootstrap", "host": "127.0.0.1",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("already bootstrapped: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBootstrapLoopbackWithoutMail(t *testing.T) {
	srv, _, _, _ := setupFreshAPI(t)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/bootstrap", "", map[string]any{
		"token": "bootstrap", "password": "admin-pass",
		"from": "wynd@example.com", "smtp_password": "x",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap: %d %s", rec.Code, rec.Body.String())
	}
	token := adminToken(t, srv)
	rec = doGET(t, srv, "/api/v1/admin/smtp", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("smtp: %d %s", rec.Code, rec.Body.String())
	}
	var smtp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&smtp); err != nil {
		t.Fatal(err)
	}
	if smtp["configured"] != false {
		t.Fatalf("smtp should stay unset: %v", smtp)
	}
}

// Инвариант: до проверки токена установка не трогает ничего. Анонимный вызов
// с чужим релеем и чужим public_url на работающем инстансе не должен ни
// сохранить SMTP (перехват кодов входа), ни переписать config.json.
func TestBootstrapRejectedLeavesSMTPAndPublicURLUntouched(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	publicBefore := srv.PublicURL()
	rec := postJSON(t, srv, "/api/v1/admin/bootstrap", "", map[string]any{
		"token": "WRONG", "password": "whatever-pass",
		"public_url": "https://evil.example",
		"host":       "127.0.0.1", "port": 1,
		"username": "u", "smtp_password": "p", "from": "attacker@evil.example",
	}, map[string]string{"X-Forwarded-For": "203.0.113.90"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong token: %d %s", rec.Code, rec.Body.String())
	}

	cfg, err := srv.Mail.LoadConfig(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "" || cfg.From != "" {
		t.Fatalf("smtp relay overwritten: %+v", cfg)
	}
	token := adminToken(t, srv)
	smtpRec := doGET(t, srv, "/api/v1/admin/smtp", token)
	if smtpRec.Code != http.StatusOK {
		t.Fatalf("smtp: %d %s", smtpRec.Code, smtpRec.Body.String())
	}
	var smtp map[string]any
	if err := json.NewDecoder(smtpRec.Body).Decode(&smtp); err != nil {
		t.Fatal(err)
	}
	if smtp["configured"] != false {
		t.Fatalf("smtp must stay unset: %v", smtp)
	}
	if _, err := os.Stat(filepath.Join(srv.DataDir, "config.json")); !os.IsNotExist(err) {
		data, _ := os.ReadFile(filepath.Join(srv.DataDir, "config.json"))
		t.Fatalf("config.json written by rejected bootstrap: %s (%v)", data, err)
	}
	if srv.PublicURL() != publicBefore {
		t.Fatalf("public_url changed: %q want %q", srv.PublicURL(), publicBefore)
	}
}

// Инвариант: флаг bootstrapped ставится одним UPDATE, поэтому из двух
// одновременных установок завершается ровно одна.
func TestBootstrapConcurrentOnlyOneSucceeds(t *testing.T) {
	srv, _, _, _ := setupFreshAPI(t)
	const n = 2
	codes := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{
				"token": "bootstrap", "instance_name": "Дом " + strconv.Itoa(i),
				"password": "admin-pass",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/bootstrap", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Forwarded-For", "203.0.113."+strconv.Itoa(100+i))
			rec := httptest.NewRecorder()
			<-start
			srv.ServeHTTP(rec, req)
			codes[i] = rec.Code
		}()
	}
	close(start)
	wg.Wait()

	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("exactly one bootstrap must win: %v", codes)
	}
}

// Обязателен только пароль: публичный инстанс ставится без почты, релей
// настраивается позже в панели.
func TestBootstrapPublicURLWithoutMail(t *testing.T) {
	srv, _, _, _ := setupFreshAPI(t)
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/bootstrap", "", map[string]any{
		"token": "bootstrap", "password": "admin-pass",
		"public_url": "home.example.org",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("public without smtp: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"mail_sent":false`) {
		t.Fatalf("mail_sent: %s", rec.Body.String())
	}
	if srv.PublicURL() != "https://home.example.org" || srv.Loopback() {
		t.Fatalf("public url: %s loopback=%v", srv.PublicURL(), srv.Loopback())
	}
	token := adminToken(t, srv)
	rec = doJSON(t, srv, http.MethodGet, "/api/v1/admin/smtp", token, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"configured":false`) {
		t.Fatalf("smtp must stay empty: %d %s", rec.Code, rec.Body.String())
	}
}

// Инвариант (аудит 2026-09-22): адрес проверяется строго и до установки.
// Раньше принималась любая строка с «://», она уходила в config.json — и
// сервер после перезапуска не стартовал, потому что config.Load проверяет
// адрес строго. Установка при этом оставалась незавершённой.
func TestBootstrapRejectsInvalidPublicURL(t *testing.T) {
	for _, bad := range []string{"ftp://home.example.org", "https://", "https://home.example.org/wynd"} {
		srv, _, _, _ := setupFreshAPI(t)
		rec := doJSON(t, srv, http.MethodPost, "/api/v1/admin/bootstrap", "", map[string]any{
			"token": "bootstrap", "password": "admin-pass",
			"public_url": bad,
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", bad, rec.Code, rec.Body.String())
		}
		// Установка не состоялась: токен по-прежнему работает.
		rec = doGET(t, srv, "/api/v1/instance", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("instance: %d", rec.Code)
		}
		var info map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
			t.Fatal(err)
		}
		if info["bootstrapped"] == true {
			t.Fatalf("%s: инстанс считается установленным", bad)
		}
	}
}
