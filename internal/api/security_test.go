package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func postJSON(t *testing.T, srv *api.Server, path, token string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	switch b := body.(type) {
	case []byte:
		raw = b
	default:
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// An untrusted peer cannot pick its rate-limit bucket via X-Forwarded-For:
// every request lands on the real remote address and the per-IP cap holds.
func TestForwardedForIgnoredFromUntrustedPeer(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	srv.TrustedProxies = nil // loopback only; httptest peers from 192.0.2.1
	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	var limited bool
	for i := 0; i < 12; i++ {
		email := "spoof" + string(rune('a'+i)) + "@example.com"
		rec := postJSON(t, srv, "/api/v1/auth/register", "", map[string]string{"email": email},
			map[string]string{"X-Forwarded-For": "203.0.113." + string(rune('1'+i%9))})
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
		if rec.Code != http.StatusAccepted {
			t.Fatalf("register %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if !limited {
		t.Fatal("per-IP limit was bypassed through X-Forwarded-For")
	}
}

// With a trusted proxy the rightmost untrusted hop is the client, so a
// client-supplied prefix does not move the bucket either.
func TestForwardedForRightmostUntrusted(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	var limited bool
	for i := 0; i < 12; i++ {
		email := "hop" + string(rune('a'+i)) + "@example.com"
		// Attacker forges the left part; the proxy appends the real 198.51.100.7.
		xff := "203.0.113." + string(rune('1'+i%9)) + ", 198.51.100.7"
		rec := postJSON(t, srv, "/api/v1/auth/register", "", map[string]string{"email": email},
			map[string]string{"X-Forwarded-For": xff})
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
		if rec.Code != http.StatusAccepted {
			t.Fatalf("register %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if !limited {
		t.Fatal("forged X-Forwarded-For prefix moved the rate-limit bucket")
	}
}

// One mailbox cannot be flooded by rotating client addresses.
func TestPerEmailCodeLimit(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	var limited bool
	for i := 0; i < 8; i++ {
		rec := postJSON(t, srv, "/api/v1/auth/register", "", map[string]string{"email": "victim@example.com"},
			map[string]string{"X-Forwarded-For": "203.0.113." + string(rune('1'+i))})
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
		if rec.Code != http.StatusAccepted {
			t.Fatalf("register %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if !limited {
		t.Fatal("per-mailbox limit missing")
	}
}

func TestSixthRegisterReturnsRetryAfter(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		rec := postJSON(t, srv, "/api/v1/auth/register", "", map[string]string{"email": "sixth@example.com"},
			map[string]string{"X-Forwarded-For": "203.0.113.99"})
		if rec.Code != http.StatusAccepted {
			t.Fatalf("register %d: %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	rec := postJSON(t, srv, "/api/v1/auth/register", "", map[string]string{"email": "sixth@example.com"},
		map[string]string{"X-Forwarded-For": "203.0.113.99"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th status: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "rate_limited" {
		t.Fatalf("error: %v", body["error"])
	}
	sec, ok := body["retry_after_sec"].(float64)
	if !ok || sec < 1 {
		t.Fatalf("retry_after_sec: %v", body["retry_after_sec"])
	}
}

// Open registration answers "code sent" for known and unknown addresses on
// both endpoints; invite mode keeps the explicit 404 on /auth/code.
func TestOpenModeDoesNotEnumerate(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	registerSession(t, srv, caps, "known@example.com")

	rec := postJSON(t, srv, "/api/v1/auth/register", "", map[string]string{"email": "known@example.com"},
		map[string]string{"X-Forwarded-For": "203.0.113.50"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("register known (open): %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, srv, "/api/v1/auth/code", "", map[string]string{"email": "unknown@example.com"},
		map[string]string{"X-Forwarded-For": "203.0.113.51"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("code unknown (open): %d %s", rec.Code, rec.Body.String())
	}
	// The unknown address got a register code: verifying it creates the account.
	rec = postJSON(t, srv, "/api/v1/auth/verify", "", map[string]string{
		"email": "unknown@example.com", "code": caps.Last("unknown@example.com"),
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify unknown (open): %d %s", rec.Code, rec.Body.String())
	}

	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeInvite); err != nil {
		t.Fatal(err)
	}
	rec = postJSON(t, srv, "/api/v1/auth/code", "", map[string]string{"email": "nobody@example.com"},
		map[string]string{"X-Forwarded-For": "203.0.113.52"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code unknown (invite) should stay 404: %d %s", rec.Code, rec.Body.String())
	}
}

// Wrong codes are capped at three attempts, then the pending code is dead.
func TestVerifyAttemptsCapped(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	if err := srv.Auth.SetRegistrationMode(t.Context(), auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	rec := postJSON(t, srv, "/api/v1/auth/register", "", map[string]string{"email": "cap@example.com"},
		map[string]string{"X-Forwarded-For": "203.0.113.60"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	real := caps.Last("cap@example.com")
	wrong := "000000"
	if wrong == real {
		wrong = "000001"
	}
	for i := 0; i < 3; i++ {
		rec = postJSON(t, srv, "/api/v1/auth/verify", "", map[string]string{"email": "cap@example.com", "code": wrong}, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("wrong attempt %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec = postJSON(t, srv, "/api/v1/auth/verify", "", map[string]string{"email": "cap@example.com", "code": real}, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("real code after cap should be refused: %d %s", rec.Code, rec.Body.String())
	}
}

// Five wrong admin passwords from one address lock it out; the sixth try is
// refused even with the right password.
func TestAdminLoginLockout(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	hdr := map[string]string{"X-Forwarded-For": "203.0.113.70"}
	for i := 0; i < 5; i++ {
		rec := postJSON(t, srv, "/api/v1/admin/login", "", map[string]string{"password": "wrong"}, hdr)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("wrong %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := postJSON(t, srv, "/api/v1/admin/login", "", map[string]string{"password": "admin-pass"}, hdr)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login: %d %s", rec.Code, rec.Body.String())
	}
	// Another address is unaffected.
	rec = postJSON(t, srv, "/api/v1/admin/login", "", map[string]string{"password": "admin-pass"},
		map[string]string{"X-Forwarded-For": "203.0.113.71"})
	if rec.Code != http.StatusOK {
		t.Fatalf("other address: %d %s", rec.Code, rec.Body.String())
	}
}

// JSON bodies over the cap are refused with 413; the upload chunk route is
// exempt (its own bound is the session size).
func TestJSONBodyCap(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "big@example.com")
	huge := []byte(`{"name":"` + strings.Repeat("x", 2<<20) + `"}`)
	rec := postJSON(t, srv, "/api/v1/circles", token, huge, nil)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized json: %d %s", rec.Code, rec.Body.String())
	}
}

// Text fields have caps independent of the attachment limit.
func TestTextLengthCaps(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "long@example.com")
	rec := postJSON(t, srv, "/api/v1/circles", token,
		map[string]string{"name": strings.Repeat("н", 101), "owner_name": "Я"}, nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too_long") {
		t.Fatalf("long circle name: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, srv, "/api/v1/circles", token, map[string]string{"name": "Круг", "owner_name": "Я"}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create circle: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&created)
	rec = postJSON(t, srv, "/api/v1/circles/"+created["id"]+"/posts", token,
		map[string]string{"body": strings.Repeat("т", 20001), "entry_date": "2026-09-03"}, nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too_long") {
		t.Fatalf("long post: %d %s", rec.Code, rec.Body.String())
	}
}

// Logout revokes the token server-side.
func TestLogoutRevokesSession(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, _ := registerSession(t, srv, caps, "bye@example.com")
	rec := postJSON(t, srv, "/api/v1/auth/logout", token, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/circles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("revoked token still works: %d", rec.Code)
	}

	admin := adminToken(t, srv)
	rec = postJSON(t, srv, "/api/v1/admin/logout", admin, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin logout: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("revoked admin token still works: %d", rec.Code)
	}
}

// A push endpoint registered by one account cannot be re-bound by another
// account that does not hold the subscription keys.
func TestPushEndpointNotHijackable(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	alice, _ := registerSession(t, srv, caps, "alice@example.com")
	bob, _ := registerSession(t, srv, caps, "bob@example.com")
	sub := map[string]string{"endpoint": "https://push.example/ep1", "p256dh": "k1", "auth": "a1"}
	if rec := postJSON(t, srv, "/api/v1/push/subscribe", alice, sub, nil); rec.Code != http.StatusOK {
		t.Fatalf("alice subscribe: %d %s", rec.Code, rec.Body.String())
	}
	hijack := map[string]string{"endpoint": "https://push.example/ep1", "p256dh": "k2", "auth": "a2"}
	if rec := postJSON(t, srv, "/api/v1/push/subscribe", bob, hijack, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("bob hijack should be forbidden: %d %s", rec.Code, rec.Body.String())
	}
	// Same browser, new account (same keys): allowed.
	if rec := postJSON(t, srv, "/api/v1/push/subscribe", bob, sub, nil); rec.Code != http.StatusOK {
		t.Fatalf("same keys rebind: %d %s", rec.Code, rec.Body.String())
	}
}

// Bootstrap refuses short passwords — но только после верного токена: на
// установленном инстансе тот же запрос обязан быть отбит как invalid, не
// сообщая ничего о пароле.
func TestBootstrapWeakPassword(t *testing.T) {
	fresh, _, _, _ := setupFreshAPI(t)
	rec := postJSON(t, fresh, "/api/v1/admin/bootstrap", "", map[string]string{
		"token": "bootstrap", "instance_name": "x", "password": "short",
	}, map[string]string{"X-Forwarded-For": "203.0.113.80"})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "weak_password") {
		t.Fatalf("weak password: %d %s", rec.Code, rec.Body.String())
	}

	done, _, _, _ := setupAPI(t)
	rec = postJSON(t, done, "/api/v1/admin/bootstrap", "", map[string]string{
		"token": "bootstrap", "instance_name": "x", "password": "short",
	}, map[string]string{"X-Forwarded-For": "203.0.113.81"})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"invalid"`) {
		t.Fatalf("bootstrapped instance: %d %s", rec.Code, rec.Body.String())
	}
}

// SPA responses carry the hardening headers.
func TestSecurityHeaders(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instance", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("api nosniff missing: %v", rec.Header())
	}
}
