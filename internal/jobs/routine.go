package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

const (
	routineGrace      = 24 * time.Hour
	emptyAccountGrace = 30 * 24 * time.Hour
)

// DailyRoutineCounts reports how many rows each cleanup step removed or updated.
type DailyRoutineCounts struct {
	OrphanedBlobs    int
	AbandonedUploads int
	EmptyAccounts    int
	ExpiredInvites   int
	ExpiredSessions  int
	ExpiredCodes     int
	OldCodeRequests  int
}

// RunDailyRoutine performs the daily maintenance pass: orphaned blobs, abandoned
// uploads, empty accounts, expired invites, and records last_routine_at.
func RunDailyRoutine(ctx context.Context, db *sql.DB, blobsDir string, now time.Time) (DailyRoutineCounts, error) {
	if db == nil {
		return DailyRoutineCounts{}, fmt.Errorf("jobs: nil db")
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := xtime.Format(now.Add(-routineGrace))
	emptyCutoff := xtime.Format(now.Add(-emptyAccountGrace))
	nowRaw := xtime.Format(now)
	expireBound := expireBefore(now)

	// Шаги независимы: сбой одного не отменяет остальные, иначе одна
	// застрявшая ошибка навсегда оставляет инстанс без уборки сессий, кодов,
	// инвайтов и загрузок (план 42, REF-3).
	var counts DailyRoutineCounts
	var errs []error
	step := func(n *int, name string, run func() (int, error)) {
		got, err := run()
		*n = got
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	step(&counts.OrphanedBlobs, "orphaned blobs", func() (int, error) {
		return cleanOrphanedBlobs(ctx, db, blobsDir, cutoff)
	})
	step(&counts.AbandonedUploads, "abandoned uploads", func() (int, error) {
		return cleanAbandonedUploads(ctx, db, blobsDir, expireBound)
	})
	step(&counts.EmptyAccounts, "empty accounts", func() (int, error) {
		return cleanEmptyAccounts(ctx, db, emptyCutoff, now)
	})
	step(&counts.ExpiredInvites, "expired invites", func() (int, error) {
		return revokeExpiredInvites(ctx, db, nowRaw)
	})
	step(&counts.ExpiredSessions, "expired sessions", func() (int, error) {
		return deleteExpired(ctx, db, "sessions", "expires_at", expireBound)
	})
	step(&counts.ExpiredCodes, "expired codes", func() (int, error) {
		return deleteExpired(ctx, db, "pending_codes", "expires_at", expireBound)
	})
	step(&counts.OldCodeRequests, "old code requests", func() (int, error) {
		return deleteExpired(ctx, db, "code_request_log", "requested_at", cutoff)
	})

	// last_routine_at пишется всегда: иначе панель показывает «рутина не
	// выполнялась» при том, что шесть шагов из семи прошли.
	if _, err := db.ExecContext(ctx, `
		UPDATE instance_settings SET last_routine_at = ? WHERE id = 1
	`, nowRaw); err != nil {
		errs = append(errs, fmt.Errorf("update last_routine_at: %w", err))
	}
	return counts, errors.Join(errs...)
}

func cleanOrphanedBlobs(ctx context.Context, db *sql.DB, blobsDir, cutoff string) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT b.id, b.storage_path
		FROM blobs b
		WHERE b.status = 'complete'
		  AND b.created_at < ?
		  AND NOT `+blob.ReferencedPredicate("b.id"), cutoff)
	if err != nil {
		return 0, fmt.Errorf("list orphaned blobs: %w", err)
	}
	defer rows.Close()

	type blobRow struct {
		id, rel string
	}
	var pending []blobRow
	for rows.Next() {
		var row blobRow
		if err := rows.Scan(&row.id, &row.rel); err != nil {
			return 0, err
		}
		pending = append(pending, row)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	// Сначала строка, потом файл: обратный порядок при отказе DELETE
	// (внешний ключ) терял файл безвозвратно (BLB-2).
	n := 0
	var errs []error
	for _, row := range pending {
		res, err := db.ExecContext(ctx, `DELETE FROM blobs WHERE id = ?`, row.id)
		if err != nil {
			errs = append(errs, fmt.Errorf("delete orphaned blob %s: %w", row.id, err))
			continue
		}
		aff, _ := res.RowsAffected()
		n += int(aff)
		if blobsDir != "" && aff > 0 {
			path := filepath.Join(blobsDir, filepath.FromSlash(row.rel))
			_ = os.Remove(path)
		}
	}
	return n, errors.Join(errs...)
}

func cleanAbandonedUploads(ctx context.Context, db *sql.DB, blobsDir, nowRaw string) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id FROM upload_sessions WHERE expires_at < ?
	`, nowRaw)
	if err != nil {
		return 0, fmt.Errorf("list abandoned uploads: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	for _, id := range ids {
		if blobsDir != "" {
			_ = os.Remove(filepath.Join(blobsDir, ".uploads", id+".part"))
		}
	}
	res, err := db.ExecContext(ctx, `DELETE FROM upload_sessions WHERE expires_at < ?`, nowRaw)
	if err != nil {
		return 0, fmt.Errorf("delete abandoned uploads: %w", err)
	}
	aff, _ := res.RowsAffected()
	return int(aff), nil
}

func cleanEmptyAccounts(ctx context.Context, db *sql.DB, cutoff string, now time.Time) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id FROM accounts
		WHERE email != ?
		  AND deleted_at IS NULL
		  AND created_at < ?
		  AND NOT EXISTS (SELECT 1 FROM memberships m WHERE m.account_id = accounts.id)
	`, auth.AdminSentinelEmail, cutoff)
	if err != nil {
		return 0, fmt.Errorf("list empty accounts: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	deletedAt := xtime.Format(now)
	n := 0
	for _, id := range ids {
		deletedEmail := fmt.Sprintf("deleted+%s@wynd.local", id)
		if _, err := db.ExecContext(ctx, `
			UPDATE accounts SET deleted_at = ?, blocked = 1, email = ? WHERE id = ?
		`, deletedAt, deletedEmail, id); err != nil {
			return n, fmt.Errorf("soft delete empty account %s: %w", id, err)
		}
		if _, err := db.ExecContext(ctx, `
			DELETE FROM sessions WHERE account_id = ? AND kind = ?
		`, id, auth.SessionParticipant); err != nil {
			return n, fmt.Errorf("revoke empty account sessions %s: %w", id, err)
		}
		n++
	}
	return n, nil
}

func revokeExpiredInvites(ctx context.Context, db *sql.DB, nowRaw string) (int, error) {
	res, err := db.ExecContext(ctx, `
		UPDATE invites
		SET revoked_at = ?
		WHERE expires_at < ? AND revoked_at IS NULL
	`, nowRaw, nowRaw)
	if err != nil {
		return 0, fmt.Errorf("revoke expired invites: %w", err)
	}
	aff, _ := res.RowsAffected()
	return int(aff), nil
}

// expireBefore returns a string bound so legacy RFC3339 timestamps (no fractional
// seconds) compare as expired within the same UTC second as now.
func expireBefore(now time.Time) string {
	return now.UTC().Truncate(time.Second).Add(time.Second).Format(time.RFC3339Nano)
}

// deleteExpired removes rows whose timestamp column is before the boundary.
// Expired sessions and codes are already rejected on read; purging them keeps
// the tables (and the rate-limit log) from growing without bound.
func deleteExpired(ctx context.Context, db *sql.DB, table, column, before string) (int, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+column+` < ?`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired %s: %w", table, err)
	}
	aff, _ := res.RowsAffected()
	return int(aff), nil
}
