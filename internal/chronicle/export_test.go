package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Хелперы для тестов пакета. Лежат в _test-файле, чтобы не выдавать себя
// за API пакета: в боевом коде их не вызывает никто (STB-1).

// VisiblePostSeqs returns post event sequences visible to account ordered by created_at.
func (c *Chronicle) VisiblePostSeqs(ctx context.Context, circleID, accountID string) ([]int64, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT event_seq, created_at FROM posts
		WHERE circle_id = ? AND deleted = 0
		ORDER BY created_at
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var seq int64
		var created string
		if err := rows.Scan(&seq, &created); err != nil {
			return nil, err
		}
		t, _ := parseTime(created)
		ok, err := c.CanReadEvent(ctx, circleID, accountID, t)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, seq)
		}
	}
	return out, rows.Err()
}

// DayExists reports whether a day projection exists.
func (c *Chronicle) DayExists(ctx context.Context, circleID, entryDate string) (bool, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM days WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate).Scan(&n)
	return n > 0, err
}

// FeedOrder returns post IDs ordered by created_at (not entry_date).
func (c *Chronicle) FeedOrder(ctx context.Context, circleID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM posts WHERE circle_id = ? AND deleted = 0 ORDER BY created_at
	`, circleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// PostBody returns stored body text (empty if deleted).
func (c *Chronicle) PostBody(ctx context.Context, postID string) (string, error) {
	var body sql.NullString
	var deleted int
	err := c.db.QueryRowContext(ctx, `SELECT body, deleted FROM posts WHERE id = ?`, postID).Scan(&body, &deleted)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if deleted == 1 {
		return "", nil
	}
	if body.Valid {
		return body.String, nil
	}
	return "", nil
}

// CommentBodies returns comment bodies for a post.
func (c *Chronicle) CommentBodies(ctx context.Context, postID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT body FROM comments WHERE post_id = ?
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var body sql.NullString
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if body.Valid {
			out = append(out, body.String)
		}
	}
	return out, rows.Err()
}

// EventCount returns number of events (for GDPR test).
func (c *Chronicle) EventCount(ctx context.Context, circleID string) (int, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE circle_id = ?`, circleID).Scan(&n)
	return n, err
}

// ErasePersonalData removes identity names without touching events.
func (c *Chronicle) ErasePersonalData(ctx context.Context, identityID string, now time.Time) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE identity_names SET name = '', erased_at = ? WHERE identity_id = ? AND erased_at IS NULL
	`, formatTime(now), identityID)
	return err
}

// LoadPostEditWindow returns the snapshot edit window stored on a post.
func (c *Chronicle) LoadPostEditWindow(ctx context.Context, postID string) (EditWindow, time.Time, error) {
	post, err := c.loadPost(ctx, c.db, postID)
	if err != nil {
		return EditWindow{}, time.Time{}, err
	}
	return post.EditWindow, post.CreatedAt, nil
}

// CommentBody returns stored comment text (empty if deleted).
func (c *Chronicle) CommentBody(ctx context.Context, commentID string) (string, error) {
	cm, err := c.loadComment(ctx, c.db, commentID)
	if err != nil {
		return "", err
	}
	if cm.Deleted {
		return "", nil
	}
	return cm.Body, nil
}

// ReactionEmoji returns stored emoji (empty if deleted).
func (c *Chronicle) ReactionEmoji(ctx context.Context, reactionID string) (string, error) {
	r, err := c.loadReaction(ctx, c.db, reactionID)
	if err != nil {
		return "", err
	}
	if r.Deleted {
		return "", nil
	}
	return r.Emoji, nil
}

// AssertDayCascadeRemoved is a test helper verifying day metadata gone.
func (c *Chronicle) AssertDayCascadeRemoved(ctx context.Context, circleID, entryDate string) error {
	exists, err := c.DayExists(ctx, circleID, entryDate)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("day still exists")
	}
	var n int
	if err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM day_titles WHERE circle_id = ? AND entry_date = ?
	`, circleID, entryDate).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return fmt.Errorf("day_titles remain")
	}
	return nil
}
