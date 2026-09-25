// Package xtime is the single place that formats and parses timestamps
// stored in SQLite.
package xtime

import "time"

// Layout — фиксированная ширина, всегда UTC и всегда девять знаков дроби.
// RFC3339Nano обрезает хвостовые нули, и строки получались разной длины:
// SQLite сравнивает их лексикографически, поэтому «10:00:00.25Z» оказывалось
// позже «10:00:00.2501Z». Тридцать сравнений и тридцать ORDER BY по меткам,
// включая отрезки видимости, зависели от этого (TIME-1).
const Layout = "2006-01-02T15:04:05.000000000Z"

// Format stores timestamps in SQLite with fixed-width nanosecond precision.
func Format(t time.Time) string {
	return t.UTC().Format(Layout)
}

// Parse reads stored timestamps. RFC3339Nano покрывает и новый формат
// фиксированной ширины, и значения с любым числом знаков дроби.
func Parse(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

// UTCOrNow returns UTC now when t is zero.
func UTCOrNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

// FormatDate returns an ISO calendar date (YYYY-MM-DD).
func FormatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
