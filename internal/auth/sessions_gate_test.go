package auth_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант (AUTH-4): в БД лежит только хеш токена. Бэкап не должен содержать
// живых Bearer на 30 дней вперёд.
func TestSessionTokenStoredHashedOnly(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	// Время входа — настоящее: SessionByToken сверяет срок с time.Now().
	now := time.Now().UTC()
	sess, err := e.auth.AdminLogin(e.ctx, auth.AdminLoginInput{
		Password: "secret-admin", ClientIP: "10.0.0.1", Now: now,
	})
	if err != nil {
		t.Fatalf("AdminLogin: %v", err)
	}

	var stored string
	if err := e.auth.DB().QueryRowContext(e.ctx,
		`SELECT token_hash FROM sessions WHERE kind = 'admin'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == sess.Token {
		t.Fatal("токен лежит в БД открытым текстом")
	}
	sum := sha256.Sum256([]byte(sess.Token))
	if stored != hex.EncodeToString(sum[:]) {
		t.Fatalf("в БД не sha256(token): %q", stored)
	}
	// Сам токен по-прежнему работает.
	if _, err := e.auth.IsAdminSession(e.ctx, sess.Token); err != nil {
		t.Fatalf("IsAdminSession: %v", err)
	}
}

// Инвариант (AUTH-3): смена пароля админа отзывает все админские сессии.
func TestChangeAdminPasswordRevokesAdminSessions(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	// Время входа — настоящее: SessionByToken сверяет срок с time.Now().
	now := time.Now().UTC()
	sess, err := e.auth.AdminLogin(e.ctx, auth.AdminLoginInput{
		Password: "secret-admin", ClientIP: "10.0.0.1", Now: now,
	})
	if err != nil {
		t.Fatalf("AdminLogin: %v", err)
	}
	if err := e.auth.ChangeAdminPassword(e.ctx, "secret-admin", "новый-пароль-8", now.Add(time.Minute)); err != nil {
		t.Fatalf("ChangeAdminPassword: %v", err)
	}
	if _, err := e.auth.IsAdminSession(e.ctx, sess.Token); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("старая админская сессия жива: %v", err)
	}
}

// Инвариант (AUTH-4): личная ссылка работает только своему адресу, и отказ
// приходит до выдачи кода — чужой адрес не получает письма.
func TestAcceptPersonalInviteRejectsOtherAddress(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	const owner = "owner-account"
	circle, _, _, err := e.ch.CreateCircle(e.ctx, chronicle.CreateCircleInput{
		Name: "Семья", OwnerAccountID: owner, OwnerName: "Аня", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Учётка адресата: личная ссылка привязана к её id.
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: "target@example.com", ClientIP: "10.0.0.5", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: "target@example.com", Code: e.caps.Last("target@example.com"),
		ClientIP: "10.0.0.5", Now: e.t0,
	})
	if err != nil {
		t.Fatal(err)
	}
	target := res.Account.ID
	inv, err := e.auth.CreateInvite(e.ctx, auth.CreateInviteInput{
		CircleID:           circle.ID,
		Kind:               auth.InviteSingle,
		MaxUses:            1,
		TTL:                72 * time.Hour,
		CreatedByAccountID: target,
		TargetAccountID:    target,
		Now:                e.t0,
	})
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	err = e.auth.AcceptInvite(e.ctx, auth.AcceptInviteInput{
		Token: inv.Token, Email: "stranger@example.com", Name: "Чужой",
		ClientIP: "10.0.0.9", Now: e.t0.Add(time.Minute),
	})
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("чужой адрес по личной ссылке: err = %v, want forbidden", err)
	}
	if e.caps.Last("stranger@example.com") != "" {
		t.Fatal("чужому адресу ушёл код")
	}

	if err := e.auth.AcceptInvite(e.ctx, auth.AcceptInviteInput{
		Token: inv.Token, Email: "target@example.com", Name: "Свой",
		ClientIP: "10.0.0.9", Now: e.t0.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("свой адрес по личной ссылке: %v", err)
	}
}
