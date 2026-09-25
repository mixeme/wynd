package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

const routineGrace = 24 * time.Hour

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
	nowRaw := xtime.Format(now)
	expireBound := expireBefore(now)

	var counts DailyRoutineCounts
	var err error

	counts.OrphanedBlobs, err = cleanOrphanedBlobs(ctx, db, blobsDir, cutoff)
	if err != nil {
		return counts, err
	}
	counts.AbandonedUploads, err = cleanAbandonedUploads(ctx, db, blobsDir, expireBound)
	if err != nil {
		return counts, err
	}
	counts.EmptyAccounts, err = cleanEmptyAccounts(ctx, db, cutoff, now)
	if err != nil {
		return counts, err
	}
	counts.ExpiredInvites, err = revokeExpiredInvites(ctx, db, nowRaw)
	if err != nil {
		return counts, err
	}
	counts.ExpiredSessions, err = deleteExpired(ctx, db, "sessions", "expires_at", expireBound)
	if err != nil {
		return counts, err
	}
	counts.ExpiredCodes, err = deleteExpired(ctx, db, "pending_codes", "expires_at", expireBound)
	if err != nil {
		return counts, err
	}
	counts.OldCodeRequests, err = deleteExpired(ctx, db, "code_request_log", "requested_at", cutoff)
	if err != nil {
		return counts, err
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE instance_settings SET last_routine_at = ? WHERE id = 1
	`, nowRaw); err != nil {
		return counts, fmt.Errorf("update last_routine_at: %w", err)
	}
	return counts, nil
}

func cleanOrphanedBlobs(ctx context.Context, db *sql.DB, blobsDir, cutoff string) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT b.id, b.storage_path
		FROM blobs b
		WHERE b.status = 'complete'
		  AND b.created_at < ?
		  AND NOT EXISTS (SELECT 1 FROM blob_refs r WHERE r.blob_id = b.id)
		  AND NOT EXISTS (SELECT 1 FROM post_media pm WHERE pm.blob_id = b.id)
	`, cutoff)
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

	n := 0
	for _, row := range pending {
		if blobsDir != "" {
			path := filepath.Join(blobsDir, filepath.FromSlash(row.rel))
			_ = os.Remove(path)
		}
		res, err := db.ExecContext(ctx, `DELETE FROM blobs WHERE id = ?`, row.id)
		if err != nil {
			return n, fmt.Errorf("delete orphaned blob %s: %w", row.id, err)
		}
		aff, _ := res.RowsAffected()
		n += int(aff)
	}
	return n, nil
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
