package api

import (
	"net/http"
	"sync"
	"time"
)

// Тяжёлые публичные пробы: /probe/body анонимно принимает до 2 МБ, а
// /probe/stream держит соединение две секунды. Обе доступны без сессии,
// поэтому на них стоит общий счётчик по адресу (DEP-5). Проверку 9.8 это не
// стесняет: она зовёт каждую пробу по разу за прогон.
const (
	probeRateWindow = time.Minute
	probeRateLimit  = 12
)

type probeLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newProbeLimiter() *probeLimiter {
	return &probeLimiter{hits: make(map[string][]time.Time)}
}

func (l *probeLimiter) allow(key string, now time.Time) bool {
	if key == "" {
		key = "unknown"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	since := now.Add(-probeRateWindow)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(since) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= probeRateLimit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	// Чужие адреса не копятся: раз в окно выметаем остывшие записи.
	if len(l.hits) > 1024 {
		for k, times := range l.hits {
			if len(times) == 0 || times[len(times)-1].Before(since) {
				delete(l.hits, k)
			}
		}
	}
	return true
}

// limitProbe wraps a public probe handler with the per-IP counter.
func (s *Server) limitProbe(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.probes.allow(s.clientIP(r), time.Now()) {
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":           "rate_limited",
				"retry_after_sec": int(probeRateWindow.Seconds()),
			})
			return
		}
		next(w, r)
	}
}
