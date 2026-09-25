package auth_test

import (
	"errors"
	"testing"

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
