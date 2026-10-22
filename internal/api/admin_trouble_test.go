package api_test

import (
	"testing"
	"time"
)

// Письмо о поломках (A10, ч. 2): без почты администратора не пишется; с ней —
// не чаще раза в сутки. Беда здесь — резервной копии ещё не было.
func TestServerTroubleMailOncePerDay(t *testing.T) {
	srv, _, _, _ := setupAPI(t)
	ctx := t.Context()
	sentAt := func() string {
		t.Helper()
		var at *string
		if err := srv.Auth.DB().QueryRowContext(ctx, `SELECT admin_trouble_sent_at FROM instance_settings WHERE id = 1`).Scan(&at); err != nil {
			t.Fatal(err)
		}
		if at == nil {
			return ""
		}
		return *at
	}
	day := time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)

	srv.MaybeSendServerTrouble(ctx, day)
	if got := sentAt(); got != "" {
		t.Fatalf("без почты администратора отмечено письмо: %q", got)
	}

	if err := srv.Mail.SetAdminEmail(ctx, "admin@example.com"); err != nil {
		t.Fatal(err)
	}
	srv.MaybeSendServerTrouble(ctx, day)
	first := sentAt()
	if first == "" {
		t.Fatal("письмо о поломках не ушло")
	}
	srv.MaybeSendServerTrouble(ctx, day.Add(5*time.Hour))
	if got := sentAt(); got != first {
		t.Fatalf("второе письмо за сутки: %q после %q", got, first)
	}
	srv.MaybeSendServerTrouble(ctx, day.Add(21*time.Hour))
	if got := sentAt(); got == first {
		t.Fatal("назавтра письмо не ушло, хотя беда не прошла")
	}
}
