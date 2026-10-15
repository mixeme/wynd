package chronicle

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Встреча приехавшего с кругом (план 53, 1.9): «Круг переехал» или «Круг
// оживили из архива» вместо вступления, пока человек не вошёл в круг.

// Arrival — что показать на 1.9.
type Arrival struct {
	Kind string // moved | revived
	// Returned — кто уже вернулся: входил на этот сервер (имена, до трёх);
	// ReturnedCount и Total — «Уже вернулись · 3 из 22».
	Returned      []string
	ReturnedCount int
	Total         int
	// YourPosts — «118 записей на месте»: записи его лица в круге.
	YourPosts int
}

// CircleArrival — встреча для этой учётки в этом круге; nil — встречать не
// нужно (не приехал с кругом или уже вошёл).
func (c *Chronicle) CircleArrival(ctx context.Context, circleID, accountID string) (*Arrival, error) {
	var a Arrival
	err := c.db.QueryRowContext(ctx, `
		SELECT kind FROM circle_arrivals WHERE circle_id = ? AND account_id = ? AND seen_at IS NULL
	`, circleID, accountID).Scan(&a.Kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT m.identity_id, a.last_login_at IS NOT NULL
		FROM memberships m
		JOIN accounts a ON a.id = m.account_id
		WHERE m.circle_id = ? AND m.status = 'active'
		ORDER BY a.last_login_at DESC
	`, circleID)
	if err != nil {
		return nil, err
	}
	var identities []string
	for rows.Next() {
		var identity string
		var returned bool
		if err := rows.Scan(&identity, &returned); err != nil {
			rows.Close()
			return nil, err
		}
		a.Total++
		if returned {
			a.ReturnedCount++
			identities = append(identities, identity)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range identities {
		if len(a.Returned) == 3 {
			break
		}
		if name, err := c.identityName(ctx, c.db, id); err == nil && name != "" {
			a.Returned = append(a.Returned, name)
		}
	}
	if err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts p JOIN memberships m ON m.identity_id = p.identity_id
		WHERE p.circle_id = ? AND m.circle_id = ? AND m.account_id = ? AND p.deleted = 0
	`, circleID, circleID, accountID).Scan(&a.YourPosts); err != nil {
		return nil, err
	}
	return &a, nil
}

// SeeArrival — «Войти в круг» на 1.9: встреча больше не показывается.
func (c *Chronicle) SeeArrival(ctx context.Context, circleID, accountID string, now time.Time) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE circle_arrivals SET seen_at = ? WHERE circle_id = ? AND account_id = ? AND seen_at IS NULL
	`, formatTime(utcOrNow(now)), circleID, accountID)
	return err
}
