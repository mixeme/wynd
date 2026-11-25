package auth_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// Класс плана 72 «сессии устройств»: метка устройства при входе, скользящий
// срок по активности, список своих сессий, отзыв по открытому id.
func openAccount(t *testing.T, e *env, email string) {
	t.Helper()
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatalf("registration mode: %v", err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{Email: email, ClientIP: "127.0.0.1", Now: e.t0}); err != nil {
		t.Fatalf("register: %v", err)
	}
}

func loginSession(t *testing.T, e *env, email, device string, at time.Time) auth.Session {
	t.Helper()
	if err := e.auth.RequestCode(e.ctx, auth.RequestCodeInput{Email: email, ClientIP: "127.0.0.1", Now: at}); err != nil {
		t.Fatalf("request code: %v", err)
	}
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: email, Code: e.caps.Last(email), ClientIP: "127.0.0.1", Device: device, Now: at,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return res.Session
}

func TestSessionDeviceStoredAndListed(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	const email = "ana@example.com"
	openAccount(t, e, email)

	first := loginSession(t, e, email, "", e.t0.Add(time.Minute))
	second := loginSession(t, e, email, "Firefox на Windows", e.t0.Add(2*time.Minute))
	if second.Device != "Firefox на Windows" {
		t.Fatalf("device = %q, want Firefox на Windows", second.Device)
	}

	list, err := e.auth.ListSessions(e.ctx, second.AccountID, auth.SessionParticipant)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d sessions, want 2", len(list))
	}
	// Свежая активность сверху: вторая сессия первая.
	if list[0].Token[:16] != auth.SessionID(second.Token) {
		t.Fatalf("order: первая в списке не самая свежая")
	}
	if list[1].Device != "" {
		t.Fatalf("device у старой сессии = %q, want пусто (до плана 72)", list[1].Device)
	}
	_ = first
}

func TestTouchSessionSlidesExpiry(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	const email = "ana@example.com"
	openAccount(t, e, email)
	res := loginSession(t, e, email, "", e.t0)
	before := res.ExpiresAt

	if err := e.auth.TouchSession(e.ctx, res.Token, "Wynd Android"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	sess, err := e.auth.SessionByToken(e.ctx, res.Token, auth.SessionParticipant)
	if err != nil {
		t.Fatalf("by token: %v", err)
	}
	if !sess.LastSeen.After(res.CreatedAt) {
		t.Fatalf("last_seen не сдвинулся: %v", sess.LastSeen)
	}
	if !sess.ExpiresAt.After(before) {
		t.Fatalf("expires не продлился: %v <= %v", sess.ExpiresAt, before)
	}
	if sess.Device != "Wynd Android" {
		t.Fatalf("device = %q, want Wynd Android", sess.Device)
	}

	// Пустая метка устройство не затирает.
	if err := e.auth.TouchSession(e.ctx, res.Token, ""); err != nil {
		t.Fatalf("touch empty: %v", err)
	}
	sess, err = e.auth.SessionByToken(e.ctx, res.Token, auth.SessionParticipant)
	if err != nil {
		t.Fatalf("by token: %v", err)
	}
	if sess.Device != "Wynd Android" {
		t.Fatalf("device затёрся пустой меткой: %q", sess.Device)
	}
}

func TestRevokeSessionByIDOnlyOwn(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	const email = "ana@example.com"
	openAccount(t, e, email)
	first := loginSession(t, e, email, "", e.t0.Add(time.Minute))
	second := loginSession(t, e, email, "Chrome на Android", e.t0.Add(2*time.Minute))

	if err := e.auth.RevokeSessionByID(e.ctx, first.AccountID, auth.SessionParticipant, auth.SessionID(first.Token)); err != nil {
		t.Fatalf("revoke by id: %v", err)
	}
	if _, err := e.auth.SessionByToken(e.ctx, first.Token, auth.SessionParticipant); err == nil {
		t.Fatal("первая сессия жива после отзыва")
	}
	if _, err := e.auth.SessionByToken(e.ctx, second.Token, auth.SessionParticipant); err != nil {
		t.Fatalf("вторая сессия задета: %v", err)
	}

	// Чужая учётная запись не отзывает по тому же id.
	e2 := newEnv(t)
	e2.bootstrap(t)
	const email2 = "bob@example.com"
	openAccount(t, e2, email2)
	other := loginSession(t, e2, email2, "", e2.t0)
	if err := e.auth.RevokeSessionByID(e.ctx, other.AccountID, auth.SessionParticipant, auth.SessionID(other.Token)); err == nil {
		t.Fatal("чужой отзыв прошёл")
	}
	if _, err := e2.auth.SessionByToken(e2.ctx, other.Token, auth.SessionParticipant); err != nil {
		t.Fatalf("сессия после чужого отзыва: %v", err)
	}
}

func TestDeviceLabel(t *testing.T) {
	cases := []struct{ ua, want string }{
		{"", ""},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36", "Chrome на Windows"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36 Edg/125.0", "Edge на Windows"},
		{"Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0", "Firefox на Linux"},
		{"Mozilla/5.0 (Linux; Android 14; CPH1931) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Mobile Safari/537.36", "Chrome на Android"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1", "Safari на iPhone"},
		{"Mozilla/5.0 (Linux; Android 14; CPH1931; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0 Mobile Safari/537.36", "Wynd Android"},
		{"WyndDesktop/0.5.2 (Windows NT 10.0)", "Wynd Настольный"},
		{"unknown-agent", "Браузер"},
	}
	for _, c := range cases {
		if got := auth.DeviceLabel(c.ua); got != c.want {
			t.Errorf("DeviceLabel(%q) = %q, want %q", c.ua, got, c.want)
		}
	}
}
