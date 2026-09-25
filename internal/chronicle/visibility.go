package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SnapshotPostLimit caps feed/day snapshots; circles beyond this are out of product scope.
const SnapshotPostLimit = 2000

// readScope caches membership visibility spans for one circle/account per snapshot request.
type readScope struct {
	spans []Span
}

func (c *Chronicle) newReadScope(ctx context.Context, circleID, accountID string) (*readScope, error) {
	spans, err := c.visibilitySpans(ctx, circleID, accountID)
	if err != nil {
		return nil, err
	}
	return &readScope{spans: spans}, nil
}

func (rs *readScope) canRead(createdAt time.Time) bool {
	for _, sp := range rs.spans {
		if !sp.CanRead {
			continue
		}
		if createdAt.Before(sp.StartedAt) {
			continue
		}
		if sp.EndedAt != nil && !createdAt.Before(*sp.EndedAt) {
			continue
		}
		return true
	}
	return false
}

// sqlVisibleAt returns an EXISTS clause: event at atColumn is readable for accountID in circleID.
func sqlVisibleAt(atColumn string) string {
	return fmt.Sprintf(`
EXISTS (
  SELECT 1 FROM memberships m
  JOIN membership_spans ms ON ms.membership_id = m.id
  WHERE m.circle_id = ? AND m.account_id = ?
    AND ms.can_read = 1
    AND %s >= ms.started_at
    AND (ms.ended_at IS NULL OR %s < ms.ended_at)
)`, atColumn, atColumn)
}

// sqlVisibleAtMembership returns an EXISTS clause for a joined memberships alias m.
func sqlVisibleAtMembership(atColumn string) string {
	return fmt.Sprintf(`
EXISTS (
  SELECT 1 FROM membership_spans ms
  WHERE ms.membership_id = m.id
    AND ms.can_read = 1
    AND %s >= ms.started_at
    AND (ms.ended_at IS NULL OR %s < ms.ended_at)
)`, atColumn, atColumn)
}

// CanReadEvent reports whether account can see an event at createdAt.
func (c *Chronicle) CanReadEvent(ctx context.Context, circleID, accountID string, createdAt time.Time) (bool, error) {
	spans, err := c.visibilitySpans(ctx, circleID, accountID)
	if err != nil {
		return false, err
	}
	for _, sp := range spans {
		if !sp.CanRead {
			continue
		}
		if createdAt.Before(sp.StartedAt) {
			continue
		}
		if sp.EndedAt != nil && !createdAt.Before(*sp.EndedAt) {
			continue
		}
		return true, nil
	}
	return false, nil
}

// CanWrite reports whether account can publish content now.
func (c *Chronicle) CanWrite(ctx context.Context, circleID, accountID string, now time.Time) (bool, error) {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return false, err
	}
	if mem.Status != StatusActive {
		return false, nil
	}
	spans, err := c.openSpans(ctx, mem.ID)
	if err != nil {
		return false, err
	}
	for _, sp := range spans {
		if sp.CanWrite && sp.EndedAt == nil && !now.Before(sp.StartedAt) {
			return true, nil
		}
	}
	return false, nil
}

func (c *Chronicle) visibilitySpans(ctx context.Context, circleID, accountID string) ([]Span, error) {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, membership_id, started_at, ended_at, can_read, can_write
		FROM membership_spans WHERE membership_id = ? ORDER BY started_at
	`, mem.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Span
	for rows.Next() {
		var sp Span
		var started string
		var ended sql.NullString
		var canRead, canWrite int
		if err := rows.Scan(&sp.ID, &sp.MembershipID, &started, &ended, &canRead, &canWrite); err != nil {
			return nil, err
		}
		sp.StartedAt, err = parseTime(started)
		if err != nil {
			return nil, fmt.Errorf("membership span started_at: %w", err)
		}
		if ended.Valid {
			t, err := parseTime(ended.String)
			if err != nil {
				return nil, fmt.Errorf("membership span ended_at: %w", err)
			}
			sp.EndedAt = &t
		}
		sp.CanRead = canRead == 1
		sp.CanWrite = canWrite == 1
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (c *Chronicle) openSpans(ctx context.Context, membershipID string) ([]Span, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, membership_id, started_at, ended_at, can_read, can_write
		FROM membership_spans WHERE membership_id = ? AND ended_at IS NULL
	`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Span
	for rows.Next() {
		var sp Span
		var started string
		var ended sql.NullString
		var canRead, canWrite int
		if err := rows.Scan(&sp.ID, &sp.MembershipID, &started, &ended, &canRead, &canWrite); err != nil {
			return nil, err
		}
		sp.StartedAt, err = parseTime(started)
		if err != nil {
			return nil, fmt.Errorf("membership span started_at: %w", err)
		}
		sp.CanRead = canRead == 1
		sp.CanWrite = canWrite == 1
		out = append(out, sp)
	}
	return out, rows.Err()
}
