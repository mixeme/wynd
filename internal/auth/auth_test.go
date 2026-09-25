package auth_test

import (
	"context"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

type env struct {
	auth *auth.Service
	ch   *chronicle.Chronicle
	caps *auth.CaptureCodes
	ctx  context.Context
	t0   time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatalf("chronicle.New: %v", err)
	}
	caps := auth.NewCaptureCodes()
	svc, err := auth.New(st, ch, caps, true)
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	return &env{
		auth: svc,
		ch:   ch,
		caps: caps,
		ctx:  context.Background(),
		t0:   time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	}
}

func (e *env) bootstrap(t *testing.T) {
	t.Helper()
	if err := e.auth.Bootstrap(e.ctx, auth.BootstrapInput{
		Token: "tok", InstanceName: "Дом Ани", Password: "secret-admin", Now: e.t0,
	}, "tok"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
}

func TestOpenRegistrationCreatesAccount(t *testing.T) {
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
	if res.Account.Email != "ana@example.com" {
		t.Fatalf("account email: %q", res.Account.Email)
	}
	if res.Session.Kind != auth.SessionParticipant {
		t.Fatalf("session kind: %q", res.Session.Kind)
	}
}

func TestClosedRegistrationRejectsOpenSignup(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeClosed); err != nil {
		t.Fatal(err)
	}
	err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "ana@example.com", ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != auth.ErrClosed {
		t.Fatalf("want ErrClosed, got %v", err)
	}
}

func TestCircleInviteCreatesMembership(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)

	ownerID := "owner-account"
	circle, _, _, err := e.ch.CreateCircle(e.ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: ownerID, OwnerName: "Аня", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := e.auth.CreateInvite(e.ctx, auth.CreateInviteInput{
		CircleID: circle.ID, Kind: auth.InviteSingle, MaxUses: 1, TTL: time.Hour, Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.AcceptInvite(e.ctx, auth.AcceptInviteInput{
		Token: inv.Token, Email: "bob@example.com", Name: "Боб", ClientIP: "10.0.0.2", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	code := e.caps.Last("bob@example.com")
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "bob@example.com", Code: code, ClientIP: "10.0.0.2", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Account.Email != "bob@example.com" {
		t.Fatalf("email: %q", res.Account.Email)
	}
	ok, err := e.ch.CanWrite(e.ctx, circle.ID, res.Account.ID, e.t0)
	if err != nil || !ok {
		t.Fatal("invite must create an active membership")
	}
}

func TestCodeExpiresAfterFifteenMinutes(t *testing.T) {
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
	_, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: code, ClientIP: "127.0.0.1",
		Now: e.t0.Add(16 * time.Minute),
	})
	if err != auth.ErrExpired {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

func TestAdminSessionSeparateFromParticipant(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	sess, err := e.auth.AdminLogin(e.ctx, auth.AdminLoginInput{Password: "secret-admin", Now: e.t0})
	if err != nil {
		t.Fatal(err)
	}
	if sess.Kind != auth.SessionAdmin {
		t.Fatalf("kind: %q", sess.Kind)
	}
	if err := auth.RejectAdminJournal(sess.Kind); err != auth.ErrForbidden {
		t.Fatalf("admin must not access journal: %v", err)
	}
}

func TestClosedRejectsLiveInvite(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeClosed); err != nil {
		t.Fatal(err)
	}
	inv, err := e.auth.CreateServerInvite(e.ctx, auth.CreateServerInviteInput{
		Kind: auth.InviteSingle, MaxUses: 1, TTL: time.Hour, Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = e.auth.AcceptInvite(e.ctx, auth.AcceptInviteInput{
		Token: inv.Token, Email: "bob@example.com", ClientIP: "10.0.0.2", Now: e.t0,
	})
	if err != auth.ErrClosed {
		t.Fatalf("want ErrClosed, got %v", err)
	}
}

func TestCodeThreeAttempts(t *testing.T) {
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
	for i := 0; i < 3; i++ {
		_, err := e.auth.Verify(e.ctx, auth.VerifyInput{
			Email: "ana@example.com", Code: "000000", ClientIP: "127.0.0.1", Now: e.t0,
		})
		if err != auth.ErrInvalid {
			t.Fatalf("attempt %d: want ErrInvalid, got %v", i+1, err)
		}
	}
	_, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "ana@example.com", Code: e.caps.Last("ana@example.com"), ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != auth.ErrTooManyAttempts {
		t.Fatalf("want ErrTooManyAttempts, got %v", err)
	}
}

func TestAdminEmailCannotRegister(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: auth.AdminSentinelEmail, ClientIP: "127.0.0.1", Now: e.t0,
	})
	if err != auth.ErrInvalid {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestBlockClosesLogin(t *testing.T) {
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
	if err := e.auth.SetAccountBlocked(e.ctx, res.Account.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.IsParticipantSession(e.ctx, res.Session.Token); err != auth.ErrNotFound {
		t.Fatalf("blocked session: %v", err)
	}
	if err := e.auth.RequestCode(e.ctx, auth.RequestCodeInput{
		Email: "ana@example.com", ClientIP: "10.0.0.9", Now: e.t0.Add(time.Minute),
	}); err != auth.ErrForbidden {
		t.Fatalf("blocked code: %v", err)
	}
	if err := e.auth.SetAccountBlocked(e.ctx, res.Account.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.RequestCode(e.ctx, auth.RequestCodeInput{
		Email: "ana@example.com", ClientIP: "10.0.0.9", Now: e.t0.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("unblocked code: %v", err)
	}
}
