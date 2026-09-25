package chronicle

import (
	"time"
)

const timeLayout = time.RFC3339Nano

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func utcOrNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(timeLayout, s)
}

func formatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
