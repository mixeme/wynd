package auth_test

import (
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
