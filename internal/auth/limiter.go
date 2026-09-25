package auth

import (
	"context"
	"sync"
	"time"
)

// failLimiter throttles password guessing on the admin login and bootstrap
// endpoints. It is in-memory: a restart clears it, which is fine because the
// point is to slow an online attacker, not to persist a ban.
type failLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	maxFails int
	lockout  time.Duration
	byKey    map[string]*failState
}

type failState struct {
	fails       int
	windowStart time.Time
	lockedUntil time.Time
}

const (
	loginFailWindow = 15 * time.Minute
	loginMaxFails   = 5
	loginLockout    = 15 * time.Minute
	// Ввод кода: потолок выше, чем у пароля админа, — за одним адресом
	// может сидеть семья, и три попытки на код расходуются быстро.
	verifyFailWindow = 15 * time.Minute
	verifyMaxFails   = 10
	verifyLockout    = 15 * time.Minute
	globalFailKey    = "*"
	globalMaxFails   = 50
	// Переполнение общего ведра замедляет, а не запирает: иначе 50 неудач с
	// любых адресов закрывали вход самому хозяину сервера на всё окно —
	// отказ в обслуживании чужими руками (аудит 2026-09-22, SEC-8).
	globalThrottle    = 2 * time.Second
	limiterGCEvery    = 256
	limiterStaleAfter = time.Hour
)

func newFailLimiter() *failLimiter {
	return newFailLimiterWith(loginFailWindow, loginMaxFails, loginLockout)
}

func newFailLimiterWith(window time.Duration, maxFails int, lockout time.Duration) *failLimiter {
	return &failLimiter{
		window:   window,
		maxFails: maxFails,
		lockout:  lockout,
		byKey:    map[string]*failState{},
	}
}

// allow reports whether key may attempt now, and how long the caller must
// wait first. Свой ключ запирается наглухо; общее ведро только замедляет.
func (l *failLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locked(key, now) {
		return false, 0
	}
	if l.locked(globalFailKey, now) {
		return true, globalThrottle
	}
	return true, 0
}

func (l *failLimiter) locked(key string, now time.Time) bool {
	st, ok := l.byKey[key]
	if !ok {
		return false
	}
	return now.Before(st.lockedUntil)
}

// fail records a failed attempt and locks the key when the window overflows.
func (l *failLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bump(key, l.maxFails, now)
	l.bump(globalFailKey, globalMaxFails, now)
	if len(l.byKey) > limiterGCEvery {
		l.gc(now)
	}
}

func (l *failLimiter) bump(key string, max int, now time.Time) {
	st, ok := l.byKey[key]
	if !ok || now.Sub(st.windowStart) > l.window {
		st = &failState{windowStart: now}
		l.byKey[key] = st
	}
	st.fails++
	if st.fails >= max {
		st.lockedUntil = now.Add(l.lockout)
		st.fails = 0
		st.windowStart = now
	}
}

// reset clears the key after a successful attempt.
func (l *failLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byKey, key)
}

// throttle выдерживает паузу, не игнорируя отмену запроса.
func throttle(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *failLimiter) gc(now time.Time) {
	for k, st := range l.byKey {
		if now.Sub(st.windowStart) > limiterStaleAfter && now.After(st.lockedUntil) {
			delete(l.byKey, k)
		}
	}
}
