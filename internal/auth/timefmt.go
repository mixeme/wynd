package auth

import (
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/uid"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

func formatTime(t time.Time) string {
	return xtime.Format(t)
}

func parseTime(raw string) (time.Time, error) {
	return xtime.Parse(raw)
}

func newID() (string, error) {
	return uid.NewID()
}
