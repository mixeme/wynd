package auth_test

import (
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

func TestPayAccountsGatedWhenSubscriptionOff(t *testing.T) {
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
	if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{
		Required: false, RemindDays: 7,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := e.auth.ListPayAccounts(e.ctx); !errors.Is(err, auth.ErrInvalid) {
		t.Fatalf("list: %v", err)
	}
	if _, err := e.auth.PayAccountByID(e.ctx, res.Account.ID); !errors.Is(err, auth.ErrInvalid) {
		t.Fatalf("by id: %v", err)
	}
	if err := e.auth.GrantPayAccount(e.ctx, res.Account.ID, 30, false); !errors.Is(err, auth.ErrInvalid) {
		t.Fatalf("grant: %v", err)
	}
}

// Инвариант (план 42, PAY-1): узкий шлюз PaymentRequired решает так же, как
// полное правило PayStatus (`required && expired && has_requisites`), во всех
// сочетаниях: шлюз выключен, нет реквизитов, срока нет, срок прошёл, срок
// впереди, бессрочно.
func TestPaymentRequiredMatchesPayStatus(t *testing.T) {
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
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: e.caps.Last("ana@example.com"), ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	setExpiry := func(v any) {
		t.Helper()
		if _, err := e.auth.DB().ExecContext(e.ctx, `
			UPDATE accounts SET subscription_expires_at = ? WHERE id = ?
		`, v, res.Account.ID); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name       string
		required   bool
		requisites string
		expiry     any
		want       bool
	}{
		{"gate off", false, "карта 1234", nil, false},
		{"no requisites", true, "", nil, false},
		{"no term", true, "карта 1234", nil, true},
		{"expired", true, "карта 1234", now.Add(-time.Hour).Format(time.RFC3339Nano), true},
		{"active", true, "карта 1234", now.Add(time.Hour).Format(time.RFC3339Nano), false},
		{"unlimited", true, "карта 1234", auth.SubscriptionUnlimitedAt.Format(time.RFC3339Nano), false},
	}
	for _, tc := range cases {
		if err := e.auth.SetPayRequisites(e.ctx, tc.requisites); err != nil {
			t.Fatal(err)
		}
		if err := e.auth.SetPaySubscriptionSettings(e.ctx, auth.PaySubscriptionSettings{
			Required: tc.required, RemindDays: 7,
		}); err != nil {
			t.Fatal(err)
		}
		setExpiry(tc.expiry)
		got, err := e.auth.PaymentRequired(e.ctx, res.Account.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		status, err := e.auth.PayStatus(e.ctx, res.Account.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		full := status.Required && status.Expired && status.HasRequisites
		if got != tc.want || got != full {
			t.Fatalf("%s: PaymentRequired=%v, PayStatus=%v, want %v", tc.name, got, full, tc.want)
		}
	}
}
