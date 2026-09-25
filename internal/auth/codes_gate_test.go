package auth_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// Инвариант (AUTH-1): код входа одноразовый. Два параллельных Verify с одним
// кодом обязаны дать ровно одну сессию, иначе перехваченный код работает
// дважды.
func TestVerifyConsumesCodeOnce(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	if err := e.auth.SetRegistrationMode(e.ctx, auth.ModeOpen); err != nil {
		t.Fatal(err)
	}
	const email = "once@example.com"
	if err := e.auth.Register(e.ctx, auth.RegisterInput{
		Email: email, ClientIP: "10.0.0.7", Now: e.t0,
	}); err != nil {
		t.Fatal(err)
	}
	code := e.caps.Last(email)
	if code == "" {
		t.Fatal("код не выдан")
	}

	const n = 2
	results := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := e.auth.Verify(e.ctx, auth.VerifyInput{
				Email: email, Code: code, ClientIP: "10.0.0.7",
				Now: e.t0.Add(time.Minute),
			})
			results[i] = err
		}()
	}
	close(start)
	wg.Wait()

	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("код сработал %d раз(а): %v", ok, results)
	}

	// Повторный вход тем же кодом после успешного — отказ.
	if _, err := e.auth.Verify(e.ctx, auth.VerifyInput{
		Email: email, Code: code, ClientIP: "10.0.0.7", Now: e.t0.Add(2 * time.Minute),
	}); err == nil {
		t.Fatal("израсходованный код принят повторно")
	}
}

// Инвариант (AUTH-2): лимит запросов кода считается до поиска учётки, поэтому
// перебор чужих адресов упирается в лимит, а не остаётся бесплатным.
func TestUnknownAddressProbesHitRateLimit(t *testing.T) {
	e := newEnv(t)
	e.bootstrap(t)
	// Режим invite: неизвестный адрес отвечает not_found — это принятый
	// оракул, но он обязан стоить попытки.
	var lastErr error
	for i := range 8 {
		lastErr = e.auth.RequestCode(e.ctx, auth.RequestCodeInput{
			Email: "probe@example.com", ClientIP: "10.0.0.8",
			Now: e.t0.Add(time.Duration(i) * time.Minute),
		})
		var rl *auth.RateLimitError
		if errors.As(lastErr, &rl) {
			return
		}
		if !errors.Is(lastErr, auth.ErrNotFound) {
			t.Fatalf("попытка %d: err = %v", i+1, lastErr)
		}
	}
	t.Fatalf("перебор неизвестных адресов не упёрся в лимит: %v", lastErr)
}
