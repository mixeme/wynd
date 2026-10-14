package chronicle

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Состояния полного архива (план 51): собирается → готов → убран (скачан,
// вышел срок, стёр администратор или владелец, отменён); сбой сборки — failed.
const (
	FullArchiveBuilding = "building"
	FullArchiveReady    = "ready"
	FullArchiveFailed   = "failed"
	FullArchiveRemoved  = "removed"

	RemovedDownloaded = "downloaded"
	RemovedExpired    = "expired"
	RemovedAdmin      = "admin"
	RemovedOwner      = "owner"
	RemovedCanceled   = "canceled"
)

// DefaultArchiveRetentionHours — сколько готовый архив ждёт скачивания, если
// администратор не задал иного (9.12; решение владельца 2026-10-13).
const DefaultArchiveRetentionHours = 72

// ErrArchiveBusy — у круга уже собирается или ждёт скачивания архив.
var ErrArchiveBusy = errors.New("chronicle: full archive already in progress")

// FullArchive — запрос полного архива круга и что с ним стало.
type FullArchive struct {
	ID            string
	CircleID      string
	RequestedBy   string
	RequestedAt   time.Time
	SnapshotAt    *time.Time
	DeleteCircle  bool
	Encrypted     bool
	Status        string
	SizeBytes     int64
	ReadyAt       *time.Time
	ExpiresAt     *time.Time
	DownloadedAt  *time.Time
	RemovedReason string
	RemovedAt     *time.Time
}

const fullArchiveCols = `id, circle_id, requested_by_account_id, requested_at, snapshot_at,
	delete_circle, encrypted, status, size_bytes, ready_at, expires_at, downloaded_at,
	COALESCE(removed_reason, ''), removed_at`

func scanFullArchive(row interface{ Scan(...any) error }) (*FullArchive, error) {
	var a FullArchive
	var requested string
	var snap, ready, expires, downloaded, removed sql.NullString
	if err := row.Scan(&a.ID, &a.CircleID, &a.RequestedBy, &requested, &snap,
		&a.DeleteCircle, &a.Encrypted, &a.Status, &a.SizeBytes, &ready, &expires, &downloaded,
		&a.RemovedReason, &removed); err != nil {
		return nil, err
	}
	a.RequestedAt = parseTimeOrZero(requested)
	a.SnapshotAt = optTime(snap)
	a.ReadyAt = optTime(ready)
	a.ExpiresAt = optTime(expires)
	a.DownloadedAt = optTime(downloaded)
	a.RemovedAt = optTime(removed)
	return &a, nil
}

func parseTimeOrZero(s string) time.Time {
	t, _ := parseTime(s)
	return t
}

func optTime(v sql.NullString) *time.Time {
	if !v.Valid || v.String == "" {
		return nil
	}
	t := parseTimeOrZero(v.String)
	return &t
}

// LatestFullArchive — последний запрос архива круга; nil — архив не просили.
func (c *Chronicle) LatestFullArchive(ctx context.Context, circleID string) (*FullArchive, error) {
	a, err := scanFullArchive(c.db.QueryRowContext(ctx, `
		SELECT `+fullArchiveCols+` FROM circle_archives
		WHERE circle_id = ? ORDER BY requested_at DESC, rowid DESC LIMIT 1
	`, circleID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// GetFullArchive — архив по id.
func (c *Chronicle) GetFullArchive(ctx context.Context, id string) (*FullArchive, error) {
	a, err := scanFullArchive(c.db.QueryRowContext(ctx, `
		SELECT `+fullArchiveCols+` FROM circle_archives WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// StartFullArchive записывает запрос: архив собирается, оценка его длины уже
// занимает место на сервере. Второй, пока первый собирается или ждёт
// скачивания, — ErrArchiveBusy. С удалением круга (6.32) круг замирает той же
// транзакцией, и в ленте встаёт «Кот остановил круг» (6.36); froze — круг
// замер сейчас, а не раньше: тогда участникам уходит письмо.
func (c *Chronicle) StartFullArchive(ctx context.Context, circleID, accountID string, deleteCircle, encrypted bool, estimate int64, now time.Time) (a *FullArchive, froze bool, err error) {
	id, err := newID()
	if err != nil {
		return nil, false, err
	}
	now = utcOrNow(now)
	var actor Membership
	var actorName string
	if deleteCircle {
		if actor, err = c.membership(ctx, c.db, circleID, accountID); err != nil {
			return nil, false, err
		}
		if actorName, err = c.identityName(ctx, c.db, actor.IdentityID); err != nil {
			return nil, false, err
		}
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var busy int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM circle_archives WHERE circle_id = ? AND status IN ('building', 'ready')
	`, circleID).Scan(&busy); err != nil {
		return nil, false, err
	}
	if busy > 0 {
		return nil, false, ErrArchiveBusy
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO circle_archives (id, circle_id, requested_by_account_id, requested_at,
			delete_circle, encrypted, status, size_bytes)
		VALUES (?, ?, ?, ?, ?, ?, 'building', ?)
	`, id, circleID, accountID, formatTime(now), deleteCircle, encrypted, estimate); err != nil {
		return nil, false, err
	}
	if deleteCircle {
		res, err := tx.ExecContext(ctx, `
			UPDATE circles SET frozen_at = ?, updated_at = ? WHERE id = ? AND frozen_at IS NULL
		`, formatTime(now), formatTime(now), circleID)
		if err != nil {
			return nil, false, err
		}
		n, _ := res.RowsAffected()
		froze = n > 0
		if froze {
			if _, err := c.appendEvent(ctx, tx, appendEventInput{
				circleID: circleID, eventType: "circle_frozen", isService: true,
				actorIdentityID: actor.IdentityID, actorName: actorName,
				summary: summaryCircleFrozen(c.identityGender(ctx, tx, actor.IdentityID), actorName),
				now:     now,
			}); err != nil {
				return nil, false, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	a, err = c.GetFullArchive(ctx, id)
	return a, froze, err
}

// CircleFrozenAt — когда круг замер под архив с удалением; nil — не замер.
func (c *Chronicle) CircleFrozenAt(ctx context.Context, circleID string) (*time.Time, error) {
	var v sql.NullString
	err := c.db.QueryRowContext(ctx, `SELECT frozen_at FROM circles WHERE id = ?`, circleID).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return optTime(v), nil
}

// UnfreezeCircle — «Отменить — оживить круг» (6.34, 6.30, 6.41): круг снова
// пишет, в ленте — «Кот оживил круг».
func (c *Chronicle) UnfreezeCircle(ctx context.Context, circleID, accountID string, now time.Time) error {
	now = utcOrNow(now)
	actor, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return err
	}
	name, err := c.identityName(ctx, c.db, actor.IdentityID)
	if err != nil {
		return err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE circles SET frozen_at = NULL, updated_at = ? WHERE id = ? AND frozen_at IS NOT NULL
	`, formatTime(now), circleID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return tx.Commit()
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: circleID, eventType: "circle_unfrozen", isService: true,
		actorIdentityID: actor.IdentityID, actorName: name,
		summary: summaryCircleUnfrozen(c.identityGender(ctx, tx, actor.IdentityID), name),
		now:     now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// SetFullArchiveSnapshot — момент, на который снят круг (6.40).
func (c *Chronicle) SetFullArchiveSnapshot(ctx context.Context, id string, at time.Time) error {
	_, err := c.db.ExecContext(ctx, `UPDATE circle_archives SET snapshot_at = ? WHERE id = ?`, formatTime(at), id)
	return err
}

// FinishFullArchive — файл собран: точная длина и срок хранения. false —
// архив за время сборки отменили, файл надо стереть.
func (c *Chronicle) FinishFullArchive(ctx context.Context, id string, size int64, now time.Time, retention time.Duration) (bool, error) {
	now = utcOrNow(now)
	res, err := c.db.ExecContext(ctx, `
		UPDATE circle_archives SET status = 'ready', size_bytes = ?, ready_at = ?, expires_at = ?
		WHERE id = ? AND status = 'building'
	`, size, formatTime(now), formatTime(now.Add(retention)), id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// FailFullArchive — сборка не удалась: место освобождается.
func (c *Chronicle) FailFullArchive(ctx context.Context, id string) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE circle_archives SET status = 'failed', size_bytes = 0 WHERE id = ? AND status = 'building'
	`, id)
	return err
}

// RemoveFullArchive убирает собираемый или готовый архив с причиной. false —
// он уже убран (другим путём): стирать файл повторно не нужно.
func (c *Chronicle) RemoveFullArchive(ctx context.Context, id, reason string, now time.Time) (bool, error) {
	now = utcOrNow(now)
	downloaded := any(nil)
	if reason == RemovedDownloaded {
		downloaded = formatTime(now)
	}
	res, err := c.db.ExecContext(ctx, `
		UPDATE circle_archives
		SET status = 'removed', removed_reason = ?, removed_at = ?, downloaded_at = COALESCE(?, downloaded_at)
		WHERE id = ? AND status IN ('building', 'ready')
	`, reason, formatTime(now), downloaded, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// FullArchivesWithFiles — архивы, у которых сейчас есть файл на диске или он
// пишется: по кругу (circleID != "") или все. Для удаления круга, очистки
// администратором и проверки при старте.
func (c *Chronicle) FullArchivesWithFiles(ctx context.Context, circleID string) ([]FullArchive, error) {
	q := `SELECT ` + fullArchiveCols + ` FROM circle_archives WHERE status IN ('building', 'ready')`
	args := []any{}
	if circleID != "" {
		q += ` AND circle_id = ?`
		args = append(args, circleID)
	}
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FullArchive
	for rows.Next() {
		a, err := scanFullArchive(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// ExpiredFullArchives — готовые архивы, чей срок вышел к now.
func (c *Chronicle) ExpiredFullArchives(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id FROM circle_archives WHERE status = 'ready' AND expires_at <= ?
	`, formatTime(utcOrNow(now)))
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

// FailInterruptedFullArchives — сборки, прерванные остановкой сервера: ключ
// шифрования жил только в памяти, доделать их нельзя. Зовётся при старте.
func (c *Chronicle) FailInterruptedFullArchives(ctx context.Context) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id FROM circle_archives WHERE status = 'building'`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := c.FailFullArchive(ctx, id); err != nil {
			return nil, err
		}
	}
	return ids, rows.Err()
}

// AccountEmail — почта учётки: письмо о готовом архиве уходит просившему.
func (c *Chronicle) AccountEmail(ctx context.Context, accountID string) (string, error) {
	var email string
	err := c.db.QueryRowContext(ctx, `SELECT email FROM accounts WHERE id = ?`, accountID).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return email, err
}

// ArchiveRetention — срок хранения готового архива (9.12).
func (c *Chronicle) ArchiveRetention(ctx context.Context) (time.Duration, error) {
	var hours int
	if err := c.db.QueryRowContext(ctx, `SELECT archive_retention_hours FROM instance_settings WHERE id = 1`).Scan(&hours); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DefaultArchiveRetentionHours * time.Hour, nil
		}
		return 0, err
	}
	if hours < 1 {
		hours = DefaultArchiveRetentionHours
	}
	return time.Duration(hours) * time.Hour, nil
}

// CircleArchiveEstimate — сколько примерно займёт полный архив (6.29): файлы
// круга и его текст. Точную длину даёт сборка; этой оценки хватает для
// «8,4 ГБ» на экране и проверки, есть ли на сервере место.
func (c *Chronicle) CircleArchiveEstimate(ctx context.Context, circleID string) (bytes int64, posts, members int, err error) {
	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback()
	ids, err := c.circleBlobIDsTx(ctx, tx, circleID)
	if err != nil {
		return 0, 0, 0, err
	}
	for start := 0; start < len(ids); start += avatarBatch {
		marks, args := inClause(ids[start:min(start+avatarBatch, len(ids))])
		var part sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT SUM(size_bytes) FROM blobs WHERE id IN (`+marks+`)`, args...).Scan(&part); err != nil {
			return 0, 0, 0, err
		}
		bytes += part.Int64
	}
	var text int64
	if err := tx.QueryRowContext(ctx, `
		SELECT text_bytes,
			(SELECT COUNT(*) FROM posts WHERE circle_id = ? AND deleted = 0),
			(SELECT COUNT(*) FROM memberships WHERE circle_id = ? AND status != 'gone')
		FROM circles WHERE id = ?
	`, circleID, circleID, circleID).Scan(&text, &posts, &members); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, 0, ErrNotFound
		}
		return 0, 0, 0, err
	}
	return bytes + text, posts, members, nil
}
