package api

import (
	"testing"
	"time"
)

// Приветствие после полуночи по Москве — сегодняшним днём устройства, а не
// вчерашним по UTC; день далеко от сейчас или не день — по UTC.
func TestGreetingDay(t *testing.T) {
	now := time.Date(2026, 10, 23, 21, 48, 0, 0, time.UTC) // 00:48 24-го по Москве
	for client, want := range map[string]string{
		"2026-10-24": "2026-10-24",
		"2026-10-23": "2026-10-23",
		"2026-09-01": "2026-10-23",
		"":           "2026-10-23",
		"вчера":      "2026-10-23",
	} {
		if got := greetingDay(client, now); got != want {
			t.Errorf("greetingDay(%q) = %q, want %q", client, got, want)
		}
	}
}
