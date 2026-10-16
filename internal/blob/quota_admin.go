package blob

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
			`+circleUsedSQL("c.id")+`
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
	ID string
	// CircleID пуст у просьбы для круга, которого пока нет (оживление, 7.30):
	// у неё — CircleName.
	CircleID       string
	CircleName     string
	RequesterID    string
	RequesterEmail string
	RequestedBytes int64
	Status         string
	AdminNote      *string
	CreatedAt      string
	ResolvedAt     *string
}

// CreateQuotaRequest opens a pending quota request for a circle owner.
// Просьба одна, пока на неё не ответили (кадр 6.19): вторая — ErrInvalid.
func (s *Store) CreateQuotaRequest(ctx context.Context, circleID, accountID string, requestedBytes int64) (string, error) {
	if requestedBytes < 1 {
		return "", ErrInvalid
	}
	var pending int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM quota_requests WHERE circle_id = ? AND status = 'pending'
	`, circleID).Scan(&pending); err != nil {
		return "", err
	}
	if pending > 0 {
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
		SELECT qr.id, COALESCE(qr.circle_id, ''), COALESCE(qr.circle_name, ''), qr.requested_by_account_id, a.email,
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
		if err := rows.Scan(&item.ID, &item.CircleID, &item.CircleName, &item.RequesterID, &item.RequesterEmail,
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

// LastQuotaRequest — последняя просьба круга о месте, если есть: экран
// «Место» показывает ждущую (6.19) и отказанную (6.20). Отказанные стираются
// отсечкой — после неё отказ уже не показывается.
func (s *Store) LastQuotaRequest(ctx context.Context, circleID string) (*QuotaRequest, error) {
	return s.quotaRequest(ctx, `qr.circle_id = ? ORDER BY qr.created_at DESC, qr.rowid DESC LIMIT 1`, circleID)
}

// QuotaRequestByID — просьба с почтой просившего: ей уходит письмо об ответе.
func (s *Store) QuotaRequestByID(ctx context.Context, id string) (*QuotaRequest, error) {
	return s.quotaRequest(ctx, `qr.id = ?`, id)
}

func (s *Store) quotaRequest(ctx context.Context, where string, arg string) (*QuotaRequest, error) {
	var item QuotaRequest
	var note, resolved sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT qr.id, COALESCE(qr.circle_id, ''), COALESCE(qr.circle_name, ''), qr.requested_by_account_id, a.email,
			qr.requested_bytes, qr.status, qr.admin_note, qr.created_at, qr.resolved_at
		FROM quota_requests qr
		JOIN accounts a ON a.id = qr.requested_by_account_id
		WHERE `+where, arg).Scan(&item.ID, &item.CircleID, &item.CircleName, &item.RequesterID, &item.RequesterEmail,
		&item.RequestedBytes, &item.Status, &note, &item.CreatedAt, &resolved)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if note.Valid {
		item.AdminNote = &note.String
	}
	if resolved.Valid {
		item.ResolvedAt = &resolved.String
	}
	return &item, nil
}

// QuotaRequestLowers — просьба опускает квоту круга: меньше действующей (или
// круг был без ограничения) и не меньше занятого. Такую просьбу одобрять
// некому и незачем — она одобряется сама.
func (s *Store) QuotaRequestLowers(ctx context.Context, circleID string, requested int64) (bool, error) {
	used, err := s.circleUsedBytes(ctx, circleID)
	if err != nil {
		return false, err
	}
	if requested <= used {
		return false, nil
	}
	cq, err := s.effectiveCircleQuotaBytes(ctx, circleID)
	if err != nil {
		return false, err
	}
	return !cq.Valid || requested < cq.Int64, nil
}

// ResolveQuotaRequest approves (sets absolute quota) or rejects a pending request.
func (s *Store) ResolveQuotaRequest(ctx context.Context, id string, approve bool, adminNote string) (string, error) {
	var circleID sql.NullString
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
		// Просьба для круга, которого пока нет, квоту ставит при оживлении.
		if circleID.Valid {
			if _, err := s.db.ExecContext(ctx, `
				UPDATE circles SET quota_custom = 1, quota_bytes = ? WHERE id = ?
			`, requested, circleID.String); err != nil {
				return "", err
			}
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

// CreatePlaceRequest — просьба о месте для круга, которого пока нет (7.30):
// оживляют круг из архива, а он больше, чем сервер даёт новому кругу. Одна
// на учётку, пока на неё не ответили.
func (s *Store) CreatePlaceRequest(ctx context.Context, accountID, circleName string, requested int64) (string, error) {
	if requested < 1 || strings.TrimSpace(circleName) == "" {
		return "", ErrInvalid
	}
	var pending int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM quota_requests
		WHERE circle_id IS NULL AND requested_by_account_id = ? AND status = 'pending'
	`, accountID).Scan(&pending); err != nil {
		return "", err
	}
	if pending > 0 {
		return "", ErrInvalid
	}
	id, err := uid.NewID()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO quota_requests (id, circle_id, circle_name, requested_by_account_id, requested_bytes, created_at)
		VALUES (?, NULL, ?, ?, ?, ?)
	`, id, circleName, accountID, requested, xtime.Format(time.Now().UTC()))
	return id, err
}

// LastPlaceRequest — последняя неиспользованная просьба учётки о месте для
// будущего круга (7.31, 7.32); nil — не просили.
func (s *Store) LastPlaceRequest(ctx context.Context, accountID string) (*QuotaRequest, error) {
	return s.quotaRequest(ctx, `qr.circle_id IS NULL AND qr.used_at IS NULL AND qr.requested_by_account_id = ?
		ORDER BY qr.created_at DESC, qr.rowid DESC LIMIT 1`, accountID)
}

// UsePlaceRequest — круг ожил по одобренной просьбе: она больше не действует.
func (s *Store) UsePlaceRequest(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE quota_requests SET used_at = ? WHERE id = ?`, xtime.Format(time.Now().UTC()), id)
	return err
}

// BlobDigest — сумма, длина и хозяин готового файла: оживление сверяет
// загруженное с описанием в архиве (план 52).
func (s *Store) BlobDigest(ctx context.Context, id string) (sha string, size int64, owner string, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT sha256, size_bytes, account_id FROM blobs WHERE id = ? AND status = 'complete'
	`, id).Scan(&sha, &size, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, "", ErrNotFound
	}
	return sha, size, owner, err
}

// OwnBlobsBySHA — готовые файлы учётки с такими суммами: сумма → id. Только
// свои: поиск по чужим выдавал бы, лежит ли на сервере такой файл.
func (s *Store) OwnBlobsBySHA(ctx context.Context, accountID string, shas []string) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(shas); start += 200 {
		part := shas[start:min(start+200, len(shas))]
		args := []any{accountID}
		marks := make([]string, len(part))
		for i, sh := range part {
			marks[i] = "?"
			args = append(args, sh)
		}
		rows, err := s.db.QueryContext(ctx, `
			SELECT sha256, id FROM blobs
			WHERE account_id = ? AND status = 'complete' AND sha256 IN (`+strings.Join(marks, ",")+`)
		`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var sh, id string
			if err := rows.Scan(&sh, &id); err != nil {
				rows.Close()
				return nil, err
			}
			out[sh] = id
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
