package blob

import (
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/uid"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

func newID() (string, error) {
	return uid.NewID()
}

func formatTime(t time.Time) string {
	return xtime.Format(t)
}

func parseTime(s string) (time.Time, error) {
	return xtime.Parse(s)
}

func utcOrNow(t time.Time) time.Time {
	return xtime.UTCOrNow(t)
}
