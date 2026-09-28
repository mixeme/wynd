package chronicle

import (
	"context"
	"time"
)

// SetSharePlace — «Место со снимков» участника в круге: уходят ли по
// умолчанию координаты из EXIF вместе с фото (wynd.html, «Геолокация»).
// Настройка личная и живёт в строке участия, поэтому одна на все устройства.
// Менять её может тот, кто в круге пишет: вышедшему с доступом она ни к чему.
// Событие хроники не пишется — это не сказанное и не структура круга.
func (c *Chronicle) SetSharePlace(ctx context.Context, circleID, accountID string, on bool, now time.Time) error {
	mem, err := c.requireWriter(ctx, circleID, accountID, now)
	if err != nil {
		return err
	}
	v := 0
	if on {
		v = 1
	}
	_, err = c.db.ExecContext(ctx, `
		UPDATE memberships SET share_place = ?, updated_at = ? WHERE id = ?
	`, v, formatTime(now), mem.ID)
	return err
}
