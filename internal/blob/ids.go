package blob

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func utcOrNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

func storageRelPath(id string) string {
	if len(id) < 2 {
		return id
	}
	return fmt.Sprintf("%s/%s", id[:2], id)
}
