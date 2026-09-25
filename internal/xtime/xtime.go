package xtime

import "time"

// Format stores timestamps in SQLite with nanosecond precision.
func Format(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// Parse reads RFC3339Nano timestamps; legacy RFC3339 (seconds) is accepted.
func Parse(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
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
