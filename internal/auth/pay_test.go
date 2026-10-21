package auth_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func TestDonateUntilDateExpires(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "ana@example.com", ClientIP: "127.0.0.1", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	code := e.caps.Last("ana@example.com")
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: code, ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPayRequisites(e.ctx, "карта 1234"); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPayDonateSettings(e.ctx, auth.PayDonateSettings{
		Text: "Сбор на диск", Show: true, Dismissible: true, Until: "2026-09-10",
	}); err != nil {
		t.Fatal(err)
	}

	during, err := e.auth.PayStatus(e.ctx, res.Account.ID, time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if during.Banner == nil {
		t.Fatal("banner should be active through the until date")
	}

	after, err := e.auth.PayStatus(e.ctx, res.Account.ID, time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if after.Banner != nil {
		t.Fatal("banner should expire after the until date")
	}
}

func TestDonateVersionBumpsOnlyWhenShown(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetPayDonateSettings(e.ctx, auth.PayDonateSettings{
		Text: "черновик", Show: false, Dismissible: true,
	}); err != nil {
		t.Fatal(err)
	}
	hub, err := e.auth.PayHubSettings(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if hub.DonateVersion != 0 {
		t.Fatalf("hidden draft should not bump version, got %d", hub.DonateVersion)
	}
	if err := e.auth.SetPayDonateSettings(e.ctx, auth.PayDonateSettings{
		Text: "Сбор на диск", Show: true, Dismissible: true,
	}); err != nil {
		t.Fatal(err)
	}
	hub, err = e.auth.PayHubSettings(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if hub.DonateVersion != 1 {
		t.Fatalf("turning on should bump version, got %d", hub.DonateVersion)
	}
}

func TestPayReminderLastDayIsOne(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "ana@example.com", ClientIP: "127.0.0.1", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	code := e.caps.Last("ana@example.com")
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: code, ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPayRequisites(e.ctx, "карта 1234"); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{
		Required: true, RemindDays: 7,
	}); err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC)
	if _, err := e.auth.DB().ExecContext(e.ctx, `
		UPDATE accounts SET subscription_expires_at = ? WHERE id = ?
	`, expires.Format(time.RFC3339Nano), res.Account.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	status, err := e.auth.PayStatus(e.ctx, res.Account.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Reminder || status.ReminderDaysLeft == nil || *status.ReminderDaysLeft != 1 {
		t.Fatalf("last-day reminder: %+v", status)
	}
}

func TestGrantPayAccountUnlimitedNotExpired(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "ana@example.com", ClientIP: "127.0.0.1", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	code := e.caps.Last("ana@example.com")
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: code, ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPayRequisites(e.ctx, "карта 1234"); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{
		Required: true, RemindDays: 7,
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.GrantPayAccount(e.ctx, res.Account.ID, 0, true); err != nil {
		t.Fatal(err)
	}
	status, err := e.auth.PayStatus(e.ctx, res.Account.ID, e.t0)
	if err != nil {
		t.Fatal(err)
	}
	if status.Expired {
		t.Fatal("unlimited subscription should not be expired")
	}
	if status.Reminder {
		t.Fatal("unlimited subscription should not remind")
	}
	var stored string
	if err := e.auth.DB().QueryRowContext(e.ctx, `
		SELECT subscription_expires_at FROM accounts WHERE id = ?
	`, res.Account.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "9999-12-31") {
		t.Fatalf("unlimited stored %q", stored)
	}
	var n int
	if err := e.auth.DB().QueryRowContext(e.ctx, `
		SELECT COUNT(*) FROM pay_requests WHERE account_id = ?
	`, res.Account.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("grant wrote pay_requests: %d", n)
	}
}

func TestGrantPayAccountDaysFromEmptyFuturePast(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "ana@example.com", ClientIP: "127.0.0.1", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	code := e.caps.Last("ana@example.com")
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: code, ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPayRequisites(e.ctx, "карта 1234"); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{
		Required: true, RemindDays: 7,
	}); err != nil {
		t.Fatal(err)
	}

	emptyStatus, err := e.auth.PayStatus(e.ctx, res.Account.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !emptyStatus.Expired {
		t.Fatal("empty expiry should be expired when required")
	}

	if err := e.auth.GrantPayAccount(e.ctx, res.Account.ID, 30, false); err != nil {
		t.Fatal(err)
	}
	got := readExpires(t, e, res.Account.ID)
	want := time.Now().UTC().AddDate(0, 0, 30)
	if got.Sub(want).Abs() > 5*time.Second {
		t.Fatalf("empty+30: got %s want ~%s", got, want)
	}

	future := time.Now().UTC().AddDate(0, 0, 10)
	if _, err := e.auth.DB().ExecContext(e.ctx, `
		UPDATE accounts SET subscription_expires_at = ? WHERE id = ?
	`, future.Format(time.RFC3339Nano), res.Account.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.GrantPayAccount(e.ctx, res.Account.ID, 5, false); err != nil {
		t.Fatal(err)
	}
	got = readExpires(t, e, res.Account.ID)
	want = future.AddDate(0, 0, 5)
	if got.Sub(want).Abs() > 5*time.Second {
		t.Fatalf("future+5: got %s want ~%s", got, want)
	}

	past := time.Now().UTC().AddDate(0, 0, -10)
	if _, err := e.auth.DB().ExecContext(e.ctx, `
		UPDATE accounts SET subscription_expires_at = ? WHERE id = ?
	`, past.Format(time.RFC3339Nano), res.Account.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.GrantPayAccount(e.ctx, res.Account.ID, 5, false); err != nil {
		t.Fatal(err)
	}
	got = readExpires(t, e, res.Account.ID)
	want = time.Now().UTC().AddDate(0, 0, 5)
	if got.Sub(want).Abs() > 5*time.Second {
		t.Fatalf("past+5: got %s want ~%s", got, want)
	}

	if err := e.auth.GrantPayAccount(e.ctx, res.Account.ID, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.GrantPayAccount(e.ctx, res.Account.ID, 10, false); err != nil {
		t.Fatal(err)
	}
	got = readExpires(t, e, res.Account.ID)
	if got.Year() == 9999 {
		t.Fatal("days after unlimited should count from today")
	}
	want = time.Now().UTC().AddDate(0, 0, 10)
	if got.Sub(want).Abs() > 5*time.Second {
		t.Fatalf("unlimited then 10: got %s want ~%s", got, want)
	}

	if _, err := e.auth.AdminLogin(e.ctx, auth.AdminLoginInput{
		Password: "secret-admin", ClientIP: "127.0.0.1", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	var sentinel string
	if err := e.auth.DB().QueryRowContext(e.ctx, `
		SELECT id FROM accounts WHERE email = ?
	`, auth.AdminSentinelEmail).Scan(&sentinel); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.GrantPayAccount(e.ctx, sentinel, 30, false); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("sentinel grant: %v", err)
	}
}

func readExpires(t *testing.T, e *env, accountID string) time.Time {
	t.Helper()
	var raw sql.NullString
	if err := e.auth.DB().QueryRowContext(e.ctx, `
		SELECT subscription_expires_at FROM accounts WHERE id = ?
	`, accountID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !raw.Valid || raw.String == "" {
		t.Fatal("empty expiry")
	}
	got, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// Переходный срок при включении подписки (2026-10-21): дни получают те, у
// кого подписки нет или она прошла; идущая и бессрочная не трогаются; повторное
// сохранение настроек дни не раздаёт.
func TestEnableSubscriptionGrantsTransitionTerm(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, email := range []string{"none@example.com", "past@example.com", "live@example.com", "forever@example.com"} {
		if err := e.auth.Register(e.ctx, auth.RegisterInput{Email: email, ClientIP: "127.0.0.1", Now: e.t0}); err != nil {
			t.Fatal(err)
		}
		res, err := e.auth.Verify(e.ctx, auth.VerifyInput{Email: email, Code: e.caps.Last(email), ClientIP: "127.0.0.1", Now: e.t0})
		if err != nil {
			t.Fatal(err)
		}
		ids[email] = res.Account.ID
	}
	now := time.Now().UTC()
	live := now.AddDate(0, 0, 10)
	for email, at := range map[string]string{
		"past@example.com":    now.AddDate(0, 0, -3).Format(time.RFC3339Nano),
		"live@example.com":    live.Format(time.RFC3339Nano),
		"forever@example.com": auth.SubscriptionUnlimitedAt.Format(time.RFC3339Nano),
	} {
		if _, err := e.auth.DB().ExecContext(e.ctx, `UPDATE accounts SET subscription_expires_at = ? WHERE id = ?`, at, ids[email]); err != nil {
			t.Fatal(err)
		}
	}
	got, err := e.auth.PaySubscriptionSettings(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.WithoutSubscription < 2 {
		t.Fatalf("без подписки: %d, ждали хотя бы 2 (none, past)", got.WithoutSubscription)
	}
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{Required: true, RemindDays: 7, GrantDays: 30}); err != nil {
		t.Fatal(err)
	}
	want := time.Now().UTC().AddDate(0, 0, 30)
	for _, email := range []string{"none@example.com", "past@example.com"} {
		if got := readExpires(t, e, ids[email]); got.Sub(want).Abs() > 5*time.Second {
			t.Fatalf("%s: %s, ждали ~%s", email, got, want)
		}
	}
	if got := readExpires(t, e, ids["live@example.com"]); got.Sub(live).Abs() > time.Second {
		t.Fatalf("идущая подписка сдвинулась: %s", got)
	}
	if got := readExpires(t, e, ids["forever@example.com"]); !got.Equal(auth.SubscriptionUnlimitedAt) {
		t.Fatalf("бессрочная сдвинулась: %s", got)
	}
	// Бессрочный переход: выключить, снять срок, включить с бессрочной.
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{Required: false, RemindDays: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.DB().ExecContext(e.ctx, `UPDATE accounts SET subscription_expires_at = NULL WHERE id = ?`, ids["past@example.com"]); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{Required: true, RemindDays: 7, GrantUnlimited: true}); err != nil {
		t.Fatal(err)
	}
	if got := readExpires(t, e, ids["past@example.com"]); !got.Equal(auth.SubscriptionUnlimitedAt) {
		t.Fatalf("бессрочный переход: %s", got)
	}
	// Подписка уже включена: новый срок не раздаётся.
	if _, err := e.auth.DB().ExecContext(e.ctx, `UPDATE accounts SET subscription_expires_at = NULL WHERE id = ?`, ids["none@example.com"]); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{Required: true, RemindDays: 3, GrantDays: 30}); err != nil {
		t.Fatal(err)
	}
	var expires sql.NullString
	if err := e.auth.DB().QueryRowContext(e.ctx, `SELECT subscription_expires_at FROM accounts WHERE id = ?`, ids["none@example.com"]).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if expires.Valid {
		t.Fatalf("повторное сохранение раздало срок: %s", expires.String)
	}
}
