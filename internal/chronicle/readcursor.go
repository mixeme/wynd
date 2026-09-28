package chronicle

import (
	"context"
	"database/sql"
	"time"
)

// ReadCursor is the per-circle sync read position for an account.
type ReadCursor struct {
	CircleID    string
	LastReadSeq int64
	UpdatedAt   time.Time
}

// GetReadCursor returns stored cursor or zero if unset.
func (c *Chronicle) GetReadCursor(ctx context.Context, accountID, circleID string) (ReadCursor, error) {
	var seq int64
	var updated string
	err := c.db.QueryRowContext(ctx, `
		SELECT last_read_seq, updated_at FROM read_cursors
		WHERE account_id = ? AND circle_id = ?
	`, accountID, circleID).Scan(&seq, &updated)
	if err == sql.ErrNoRows {
		return ReadCursor{CircleID: circleID, LastReadSeq: 0}, nil
	}
	if err != nil {
		return ReadCursor{}, err
	}
	t, _ := parseTime(updated)
	return ReadCursor{CircleID: circleID, LastReadSeq: seq, UpdatedAt: t}, nil
}

// SetReadCursor advances the read cursor if seq is greater.
func (c *Chronicle) SetReadCursor(ctx context.Context, accountID, circleID string, seq int64, now time.Time) error {
	if seq < 0 {
		return ErrInvalid
	}
	now = utcOrNow(now)
	if _, err := c.membership(ctx, c.db, circleID, accountID); err != nil {
		return err
	}
	_, err := c.db.ExecContext(ctx, `
		INSERT INTO read_cursors (account_id, circle_id, last_read_seq, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, circle_id) DO UPDATE SET
			last_read_seq = CASE WHEN excluded.last_read_seq > read_cursors.last_read_seq
				THEN excluded.last_read_seq ELSE read_cursors.last_read_seq END,
			updated_at = excluded.updated_at
	`, accountID, circleID, seq, formatTime(now))
	return err
}

