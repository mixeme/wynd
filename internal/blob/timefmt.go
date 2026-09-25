package blob

import (
	"fmt"
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

func storageRelPath(id string) string {
	if len(id) < 2 {
		return id
	}
	return fmt.Sprintf("%s/%s", id[:2], id)
}
