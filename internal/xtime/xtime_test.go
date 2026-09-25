package xtime_test

import (
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

// Инвариант (TIME-1): порядок строк = порядок времени. SQLite сравнивает
// метки лексикографически, поэтому формат обязан быть фиксированной ширины —
// иначе «10:00:00.25Z» оказывается позже «10:00:00.2501Z».
func TestFormatOrdersLikeTime(t *testing.T) {
	base := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	times := make([]time.Time, 1000)
	r := rand.New(rand.NewPCG(1, 2))
	for i := range times {
		// Сутки вокруг базы, с наносекундами — многие из них с нулями в хвосте.
		times[i] = base.Add(time.Duration(r.Int64N(int64(24*time.Hour)))).
			Add(time.Duration(r.Int64N(int64(time.Second))))
	}
	sorted := make([]time.Time, len(times))
	copy(sorted, times)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	strs := make([]string, len(times))
	for i, tm := range times {
		strs[i] = xtime.Format(tm)
	}
	sort.Strings(strs)

	for i := range sorted {
		if strs[i] != xtime.Format(sorted[i]) {
			t.Fatalf("позиция %d: строка %q, время %q", i, strs[i], xtime.Format(sorted[i]))
		}
	}
}

// Ширина одна для любого времени, включая целые секунды и полночь.
func TestFormatFixedWidth(t *testing.T) {
	cases := []time.Time{
		time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 30, 10, 0, 0, 250000000, time.UTC),
		time.Date(2026, 8, 30, 10, 0, 0, 250100000, time.UTC),
		time.Date(2026, 8, 30, 10, 0, 0, 999999999, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	width := len(xtime.Format(cases[0]))
	if width != 30 {
		t.Fatalf("ширина = %d, want 30", width)
	}
	for _, tm := range cases {
		got := xtime.Format(tm)
		if len(got) != width {
			t.Fatalf("%q: ширина %d, want %d", got, len(got), width)
		}
		back, err := xtime.Parse(got)
		if err != nil {
			t.Fatalf("Parse(%q): %v", got, err)
		}
		if !back.Equal(tm) {
			t.Fatalf("round-trip %q: got %v want %v", got, back, tm)
		}
	}
}

// Метка из другого часового пояса приводится к UTC, а не пишется со сдвигом.
func TestFormatAlwaysUTC(t *testing.T) {
	msk := time.FixedZone("MSK", 3*3600)
	got := xtime.Format(time.Date(2026, 8, 30, 13, 0, 0, 0, msk))
	if got != "2026-08-30T10:00:00.000000000Z" {
		t.Fatalf("got %q", got)
	}
}

// Значения прежнего формата (любое число знаков дроби) читаются по-прежнему:
// иначе миграция на заполненной БД оставила бы нечитаемые строки.
func TestParseAcceptsLegacyWidths(t *testing.T) {
	cases := map[string]time.Time{
		"2026-08-30T10:00:00Z":           time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
		"2026-08-30T10:00:00.25Z":        time.Date(2026, 8, 30, 10, 0, 0, 250000000, time.UTC),
		"2026-08-30T10:00:00.2501Z":      time.Date(2026, 8, 30, 10, 0, 0, 250100000, time.UTC),
		"2026-08-30T10:00:00.000000000Z": time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
	}
	for raw, want := range cases {
		got, err := xtime.Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		if !got.Equal(want) {
			t.Fatalf("Parse(%q) = %v, want %v", raw, got, want)
		}
	}
	if got, err := xtime.Parse(""); err != nil || !got.IsZero() {
		t.Fatalf(`Parse("") = %v, %v`, got, err)
	}
}
