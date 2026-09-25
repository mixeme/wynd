package auth_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// Инвариант (SEC-8, аудит 2026-09-22): попытки ввода кода считаются по адресу
// обратившегося. Знающий чужую почту сжигал три попытки её кода сколько
// угодно раз; теперь его адрес запирается, а хозяин со своего адреса входит.
func TestVerifyAttemptsLimitedPerClientIP(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	const email = "victim@example.com"
	const victimIP = "10.0.0.7"
	const attackerIP = "203.0.113.9"

	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: email, ClientIP: victimIP, Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}

	when := e.t0.Add(time.Minute)
	locked := false
	for i := range 20 {
		_, err := e.auth.Verify(e.ctx, auth.VerifyInput{
			Email: email, Code: "000000", ClientIP: attackerIP, Now: when,
		})
		if err == nil {
			t.Fatal("случайный код подошёл")
		}
		if errors.Is(err, auth.ErrRateLimited) {
			locked = true
			if i == 0 {
				t.Fatal("первая же попытка отвергнута лимитом")
			}
			break
		}
	}
	if !locked {
		t.Fatal("адрес, сжигающий чужие попытки, не заперт")
	}

	// Хозяин просит новый код (старый израсходован злоумышленником) и входит.
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: email, ClientIP: victimIP, Now: when,
	}); err != nil {
		t.Fatal(err)
	}
	code := e.caps.Last(email)
	if code == "" {
		t.Fatal("код не выдан")
	}
	if _, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: email, Code: code, ClientIP: victimIP, Now: when.Add(time.Minute),
	}); err != nil {
		t.Fatalf("хозяин не вошёл со своего адреса: %v", err)
	}
}

// Инвариант (SEC-8): потолок запросов кода не обходится параллельностью.
// Проверка и запись в журнал шли двумя операторами, и десять одновременных
// запросов проходили все.
func TestCodeRequestLimitHoldsUnderConcurrency(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	const clientIP = "198.51.100.5"
	const n = 16

	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// Разные адреса почты: проверяется потолок на IP, не на ящик.
			errs[i] = e.auth.Register(e.ctx, auth.RegisterInput{
				Email:    fmt.Sprintf("user%02d@example.com", i),
				ClientIP: clientIP,
				Now:      e.t0,
			})
		}()
	}
	close(start)
	wg.Wait()

	passed, limited := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			passed++
		case errors.Is(err, auth.ErrRateLimited):
			limited++
		}
		// Прочее — «database table is locked» от параллельной записи в
		// общую память теста: такой запрос кода не выдал, и потолка не
		// касается.
	}
	if passed > 10 {
		t.Fatalf("сквозь потолок прошло %d запросов, больше десяти", passed)
	}
	if passed == 0 || limited == 0 {
		t.Fatalf("прошло %d, отвергнуто лимитом %d — потолок не проверен", passed, limited)
	}
}
