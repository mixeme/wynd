package auth

import (
	"testing"
	"time"
)

// Инвариант (SEC-8, аудит 2026-09-22): общее ведро неудач замедляет, но не
// запирает. Раньше 50 неудачных попыток с любых адресов закрывали вход в
// панель самому хозяину на всё окно — отказ в обслуживании чужими руками.
func TestGlobalFailBucketThrottlesWithoutLocking(t *testing.T) {
	l := newFailLimiter()
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	for i := range globalMaxFails + 5 {
		l.fail(fmtKey(i), now)
	}

	ok, delay := l.allow("fresh-address", now)
	if !ok {
		t.Fatal("чужие неудачи заперли вход с нового адреса")
	}
	if delay != globalThrottle {
		t.Fatalf("задержка %v, ожидалась %v", delay, globalThrottle)
	}
}

// Инвариант: свой ключ после maxFails запирается наглухо на lockout.
func TestOwnKeyLocksAfterMaxFails(t *testing.T) {
	l := newFailLimiter()
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	for range loginMaxFails {
		l.fail("198.51.100.7", now)
	}
	if ok, _ := l.allow("198.51.100.7", now); ok {
		t.Fatal("ключ не заперт после потолка неудач")
	}
	if ok, _ := l.allow("198.51.100.8", now); !ok {
		t.Fatal("заперт чужой ключ")
	}
	if ok, _ := l.allow("198.51.100.7", now.Add(loginLockout+time.Second)); !ok {
		t.Fatal("ключ не отпустило после lockout")
	}
}

// Успешная попытка снимает счётчик своего ключа.
func TestResetClearsKey(t *testing.T) {
	l := newFailLimiter()
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	for range loginMaxFails {
		l.fail("198.51.100.7", now)
	}
	l.reset("198.51.100.7")
	if ok, _ := l.allow("198.51.100.7", now); !ok {
		t.Fatal("reset не снял замок")
	}
}

func fmtKey(i int) string {
	return "key-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
}
