package chronicle

import (
	"context"
	"database/sql"
)

const syncBatchSize = 200

// SyncBatchSize is the maximum events per sync response.
func SyncBatchSize() int { return syncBatchSize }

// MaxEventSeq returns the highest event sequence in the instance.
func (c *Chronicle) MaxEventSeq(ctx context.Context) (int64, error) {
	var seq sql.NullInt64
	err := c.db.QueryRowContext(ctx, `SELECT MAX(seq) FROM events`).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return seq.Int64, nil
}

// SyncEvents returns events with seq > cursor visible to account, ordered by seq.
func (c *Chronicle) SyncEvents(ctx context.Context, accountID string, cursor int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > syncBatchSize {
		limit = syncBatchSize
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT e.seq, e.id, e.circle_id, e.event_type, e.is_service,
			e.actor_identity_id, e.actor_name, e.target_id, e.payload, e.summary, e.created_at
		FROM events e
		WHERE e.seq > ?
		  AND EXISTS (
		    SELECT 1 FROM memberships m
		    JOIN membership_spans ms ON ms.membership_id = m.id
		    WHERE m.circle_id = e.circle_id AND m.account_id = ?
		      AND ms.can_read = 1
		      AND e.created_at >= ms.started_at
		      AND (ms.ended_at IS NULL OR e.created_at < ms.ended_at)
		  )
		ORDER BY e.seq
		LIMIT ?
	`, cursor, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var out []Event
	for rows.Next() {
		var ev Event
		var actorID sql.NullString
		var targetID sql.NullString
		var isService int
		var created string
		if err := rows.Scan(&ev.Seq, &ev.ID, &ev.CircleID, &ev.Type, &isService,
			&actorID, &ev.ActorName, &targetID, &ev.Payload, &ev.Summary, &created); err != nil {
			return nil, err
		}
		ev.IsService = isService == 1
		if actorID.Valid {
			ev.ActorIdentityID = actorID.String
		}
		if targetID.Valid {
			ev.TargetID = targetID.String
		}
		ev.CreatedAt, _ = parseTime(created)
		out = append(out, ev)
	}
	return out, rows.Err()
}
