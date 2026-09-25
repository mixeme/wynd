package blob

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/uid"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

// StorageCircle is an admin storage overview row.
type StorageCircle struct {
	ID          string
	Name        string
	Color       string
	Posts       int
	MediaBytes  int64
	QuotaBytes  *int64
	QuotaCustom bool
	OwnerEmail  string
}

// ListStorageCircles returns circles ordered by media usage.
func (s *Store) ListStorageCircles(ctx context.Context) ([]StorageCircle, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.color, c.quota_bytes, c.quota_custom, a.email,
			(SELECT COUNT(*) FROM posts p WHERE p.circle_id = c.id AND p.deleted = 0),
			(SELECT COALESCE(SUM(b.size_bytes), 0)
			 FROM post_media pm
			 JOIN posts p ON p.id = pm.post_id AND p.circle_id = c.id AND p.deleted = 0
			 JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete')
		FROM circles c
		JOIN accounts a ON a.id = c.owner_account_id
		ORDER BY 7 DESC, c.name COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]StorageCircle, 0)
	for rows.Next() {
		var row StorageCircle
		var quota sql.NullInt64
		var custom int
		if err := rows.Scan(&row.ID, &row.Name, &row.Color, &quota, &custom, &row.OwnerEmail,
			&row.Posts, &row.MediaBytes); err != nil {
			return nil, err
		}
		row.QuotaCustom = custom != 0
		if quota.Valid {
			q := quota.Int64
			row.QuotaBytes = &q
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SetCircleQuotaAdmin updates per-circle quota mode and resolves pending requests on change.
func (s *Store) SetCircleQuotaAdmin(ctx context.Context, circleID string, custom bool, quotaBytes *int64) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM circles WHERE id = ?`, circleID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !custom {
		if _, err := s.db.ExecContext(ctx, `UPDATE circles SET quota_custom = 0 WHERE id = ?`, circleID); err != nil {
			return err
		}
	} else if quotaBytes != nil {
		if *quotaBytes < 1 {
			return ErrInvalid
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE circles SET quota_custom = 1, quota_bytes = ? WHERE id = ?
		`, *quotaBytes, circleID); err != nil {
			return err
		}
	} else {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE circles SET quota_custom = 1, quota_bytes = NULL WHERE id = ?
		`, circleID); err != nil {
			return err
		}
	}
	now := xtime.Format(time.Now().UTC())
	_, _ = s.db.ExecContext(ctx, `
		UPDATE quota_requests SET status = 'approved', resolved_at = ?
		WHERE circle_id = ? AND status = 'pending'
	`, now, circleID)
	return nil
}

// QuotaRequest is a pending or resolved storage increase request.
type QuotaRequest struct {
	ID             string
	CircleID       string
	RequesterID    string
	RequesterEmail string
	RequestedBytes int64
	Status         string
	AdminNote      *string
	CreatedAt      string
	ResolvedAt     *string
}

// CreateQuotaRequest opens a pending quota request for a circle owner.
func (s *Store) CreateQuotaRequest(ctx context.Context, circleID, accountID string, requestedBytes int64) (string, error) {
	if requestedBytes < 1 {
		return "", ErrInvalid
	}
	id, err := uid.NewID()
	if err != nil {
		return "", err
	}
	now := xtime.Format(time.Now().UTC())
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO quota_requests (id, circle_id, requested_by_account_id, requested_bytes, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, circleID, accountID, requestedBytes, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ListPendingQuotaRequests returns open quota requests for admin review.
func (s *Store) ListPendingQuotaRequests(ctx context.Context) ([]QuotaRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT qr.id, qr.circle_id, qr.requested_by_account_id, a.email,
			qr.requested_bytes, qr.status, qr.admin_note, qr.created_at, qr.resolved_at
		FROM quota_requests qr
		JOIN accounts a ON a.id = qr.requested_by_account_id
		WHERE qr.status = 'pending'
		ORDER BY qr.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QuotaRequest
	for rows.Next() {
		var item QuotaRequest
		var note, resolved sql.NullString
		if err := rows.Scan(&item.ID, &item.CircleID, &item.RequesterID, &item.RequesterEmail,
			&item.RequestedBytes, &item.Status, &note, &item.CreatedAt, &resolved); err != nil {
			return nil, err
		}
		if note.Valid {
			item.AdminNote = &note.String
		}
		if resolved.Valid {
			item.ResolvedAt = &resolved.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ResolveQuotaRequest approves (sets absolute quota) or rejects a pending request.
func (s *Store) ResolveQuotaRequest(ctx context.Context, id string, approve bool, adminNote string) (string, error) {
	var circleID string
	var requested int64
	err := s.db.QueryRowContext(ctx, `
		SELECT circle_id, requested_bytes FROM quota_requests WHERE id = ? AND status = 'pending'
	`, id).Scan(&circleID, &requested)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	status := "rejected"
	if approve {
		status = "approved"
		if _, err := s.db.ExecContext(ctx, `
			UPDATE circles SET quota_custom = 1, quota_bytes = ? WHERE id = ?
		`, requested, circleID); err != nil {
			return "", err
		}
	}
	now := xtime.Format(time.Now().UTC())
	_, err = s.db.ExecContext(ctx, `
		UPDATE quota_requests SET status = ?, admin_note = NULLIF(?, ''), resolved_at = ? WHERE id = ?
	`, status, adminNote, now, id)
	if err != nil {
		return "", err
	}
	return status, nil
}

// CircleExists reports whether a circle id is present.
func (s *Store) CircleExists(ctx context.Context, circleID string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM circles WHERE id = ?`, circleID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
