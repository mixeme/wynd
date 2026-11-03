package chronicle

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"
)

// ArchiveWindow — отдельное право сохранения. Не используется для чтения.
type ArchiveWindow struct {
	StartedAt time.Time
	EndedAt   *time.Time
}

// ErrArchivePartial требует подтверждения потери несохранённого журнала.
var ErrArchivePartial = errors.New("chronicle: partial archive removal needs acknowledgement")

// ArchiveRemoval привязывает удаление к скачанной копии текущей ревизии.
type ArchiveRemoval struct {
	ID            string
	AcceptPartial bool
}

func checkArchiveRemoval(ctx context.Context, tx *sql.Tx, circleID string, condition ArchiveRemoval) error {
	var full bool
	err := tx.QueryRowContext(ctx, `SELECT a.full_journal FROM circle_archives a JOIN circles c ON c.id = a.circle_id
		WHERE a.id = ? AND c.id = ? AND c.frozen_at IS NOT NULL AND a.delete_circle = 1
		AND a.downloaded_at IS NOT NULL AND a.scope_revision = c.archive_revision`, condition.ID, circleID).Scan(&full)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalid
	}
	if err != nil {
		return err
	}
	if !full && !condition.AcceptPartial {
		return ErrArchivePartial
	}
	return nil
}

// ArchiveScope — право нынешнего владельца сохранять прошлое и будущее.
type ArchiveScope struct {
	All      bool
	Revision int64
	Windows  []ArchiveWindow
}

type archiveReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func archiveScope(ctx context.Context, q archiveReader, circleID string) (ArchiveScope, error) {
	var scope ArchiveScope
	if err := q.QueryRowContext(ctx, `SELECT archive_all, archive_revision FROM circles WHERE id = ?`, circleID).Scan(&scope.All, &scope.Revision); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return scope, ErrNotFound
		}
		return scope, err
	}
	rows, err := q.QueryContext(ctx, `SELECT started_at, ended_at FROM archive_spans WHERE circle_id = ? ORDER BY started_at, id`, circleID)
	if err != nil {
		return scope, err
	}
	defer rows.Close()
	for rows.Next() {
		var start string
		var end sql.NullString
		if err := rows.Scan(&start, &end); err != nil {
			return scope, err
		}
		at, err := parseTime(start)
		if err != nil {
			return scope, err
		}
		scope.Windows = append(scope.Windows, ArchiveWindow{at, optTime(end)})
	}
	return scope, rows.Err()
}

// ArchiveScope возвращает отдельное право сохранения, не право чтения.
func (c *Chronicle) ArchiveScope(ctx context.Context, circleID string) (ArchiveScope, error) {
	return archiveScope(ctx, c.db, circleID)
}

func normalizeArchiveWindows(windows []ArchiveWindow) []ArchiveWindow {
	sort.Slice(windows, func(i, j int) bool { return windows[i].StartedAt.Before(windows[j].StartedAt) })
	var out []ArchiveWindow
	for _, w := range windows {
		if w.EndedAt != nil && w.EndedAt.Before(w.StartedAt) {
			continue
		}
		if len(out) == 0 {
			out = append(out, w)
			continue
		}
		last := &out[len(out)-1]
		if last.EndedAt == nil {
			continue
		}
		if w.StartedAt.After(*last.EndedAt) {
			out = append(out, w)
			continue
		}
		if w.EndedAt == nil || w.EndedAt.After(*last.EndedAt) {
			last.EndedAt = w.EndedAt
		}
	}
	return out
}

func (scope ArchiveScope) contains(at time.Time, service bool) bool {
	if scope.All {
		return true
	}
	for _, w := range scope.Windows {
		if at.Before(w.StartedAt) {
			continue
		}
		if w.EndedAt == nil || at.Before(*w.EndedAt) || service && at.Equal(*w.EndedAt) {
			return true
		}
	}
	return false
}

// transferArchiveScope вызывается в транзакции смены владельца.
func transferArchiveScope(ctx context.Context, tx *sql.Tx, circleID, membershipID string, preserve bool, now time.Time) error {
	scope, err := archiveScope(ctx, tx, circleID)
	if err != nil {
		return err
	}
	all := preserve && scope.All
	var windows []ArchiveWindow
	if preserve && !all {
		windows = append(windows, scope.Windows...)
	}
	if !all {
		rows, err := tx.QueryContext(ctx, `SELECT started_at, ended_at FROM membership_spans WHERE membership_id = ? AND can_read = 1`, membershipID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var start string
			var end sql.NullString
			if err := rows.Scan(&start, &end); err != nil {
				rows.Close()
				return err
			}
			at, err := parseTime(start)
			if err != nil {
				rows.Close()
				return err
			}
			windows = append(windows, ArchiveWindow{at, optTime(end)})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		// Будущий журнал владелец сохраняет независимо от унаследованного прошлого.
		windows = append(windows, ArchiveWindow{StartedAt: now})
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM archive_spans WHERE circle_id = ?`, circleID); err != nil {
		return err
	}
	for _, w := range normalizeArchiveWindows(windows) {
		id, err := newID()
		if err != nil {
			return err
		}
		var end any
		if w.EndedAt != nil {
			end = formatTime(*w.EndedAt)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO archive_spans (id,circle_id,started_at,ended_at) VALUES (?,?,?,?)`, id, circleID, formatTime(w.StartedAt), end); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE circles SET archive_all = ?, archive_revision = archive_revision + 1 WHERE id = ?`, all, circleID); err != nil {
		return err
	}
	// Старые готовые и идущие сборки не дают обойти новый выбор.
	_, err = tx.ExecContext(ctx, `UPDATE circle_archives SET status = 'removed', removed_reason = 'owner', removed_at = ?
		WHERE circle_id = ? AND status IN ('building','ready','failed')`, formatTime(now), circleID)
	return err
}

// OwnershipRevokedArchiveIDs перечисляет прежние копии для отмены работника и очистки диска.
func (c *Chronicle) OwnershipRevokedArchiveIDs(ctx context.Context, circleID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id FROM circle_archives WHERE circle_id = ? AND scope_revision < (SELECT archive_revision FROM circles WHERE id = ?)`, circleID, circleID)
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
