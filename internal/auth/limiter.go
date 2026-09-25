package auth

import (
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
	loginFailWindow   = 15 * time.Minute
	loginMaxFails     = 5
	loginLockout      = 15 * time.Minute
	globalFailKey     = "*"
	globalMaxFails    = 50
	limiterGCEvery    = 256
	limiterStaleAfter = time.Hour
)

func newFailLimiter() *failLimiter {
	return &failLimiter{
		window:   loginFailWindow,
		maxFails: loginMaxFails,
		lockout:  loginLockout,
		byKey:    map[string]*failState{},
	}
}

// allow reports whether key (and the global bucket) may attempt now.
func (l *failLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.locked(key, now) && !l.locked(globalFailKey, now)
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

func (l *failLimiter) gc(now time.Time) {
	for k, st := range l.byKey {
		if now.Sub(st.windowStart) > limiterStaleAfter && now.After(st.lockedUntil) {
			delete(l.byKey, k)
		}
	}
}
