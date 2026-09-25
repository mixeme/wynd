package auth_test

import (
	"errors"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// Инвариант (план 42, BKP-4): `wynd admin-password` меняет пароль панели без
// старого — только на установленном инстансе и не короче минимума; открытые
// сессии панели закрываются, входит новый пароль, старый — нет.
func TestSetAdminPasswordRevokesSessions(t *testing.T) {
	e := newEnv(t)
	if err := e.auth.SetAdminPassword(e.ctx, "new-secret-1", e.t0); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("before bootstrap: %v, want ErrNotFound", err)
	}
	e.bootstrap(t)
	old, err := e.auth.AdminLogin(e.ctx, auth.AdminLoginInput{Password: "secret-admin", Now: e.t0})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.auth.SetAdminPassword(e.ctx, "short", e.t0); !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("short password: %v, want ErrWeakPassword", err)
	}
	if err := e.auth.SetAdminPassword(e.ctx, "new-secret-1", e.t0); err != nil {
		t.Fatal(err)
	}
	if _, err := e.auth.IsAdminSession(e.ctx, old.Token); err == nil {
		t.Fatal("старая сессия панели жива после смены пароля")
	}
	if _, err := e.auth.AdminLogin(e.ctx, auth.AdminLoginInput{Password: "secret-admin", Now: e.t0}); err == nil {
		t.Fatal("старый пароль всё ещё входит")
	}
	if _, err := e.auth.AdminLogin(e.ctx, auth.AdminLoginInput{Password: "new-secret-1", Now: e.t0}); err != nil {
		t.Fatalf("новый пароль: %v", err)
	}
}
