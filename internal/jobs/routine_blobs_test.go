package jobs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/jobs"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func seedBlobFile(t *testing.T, st *store.SQLite, blobsDir, id, rel, createdAt string) string {
	t.Helper()
	path := filepath.Join(blobsDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(id), 0o640); err != nil {
		t.Fatal(err)
	}
	_, err := st.DB().ExecContext(t.Context(), `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES (?, 'member', 'deadbeef', 6, 'image/jpeg', ?, 'complete', ?)
	`, id, rel, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// Инвариант (BLB-1): сборщик сирот знает обо всех, кто ссылается на блоб.
// Аватар участника и обложка дня живут в своих таблицах, а не в blob_refs, и
// раньше исчезали с диска через сутки после загрузки.
func TestRunDailyRoutineKeepsAvatarsAndDayCovers(t *testing.T) {
	st := openDB(t)
	ctx := context.Background()
	blobsDir := t.TempDir()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)

	insertAccount(t, st, "member", "member@test.local", old)
	mustExec(t, st, `INSERT INTO circles (id, name, owner_account_id, created_at, updated_at)
		VALUES ('c1', 'Test', 'member', ?, ?)`, old, old)
	mustExec(t, st, `INSERT INTO identities (id, circle_id, account_id, created_at)
		VALUES ('id1', 'c1', 'member', ?)`, old)

	avatarPath := seedBlobFile(t, st, blobsDir, "blob-avatar", "av/atar", old)
	dayCoverPath := seedBlobFile(t, st, blobsDir, "blob-daycover", "dc/over", old)
	erasedPath := seedBlobFile(t, st, blobsDir, "blob-erased", "er/ased", old)

	mustExec(t, st, `INSERT INTO identity_names (id, identity_id, name, avatar_blob_id, effective_at)
		VALUES ('n1', 'id1', 'Аня', 'blob-avatar', ?)`, old)
	mustExec(t, st, `INSERT INTO identity_names (id, identity_id, name, avatar_blob_id, effective_at, erased_at)
		VALUES ('n0', 'id1', 'Старое', 'blob-erased', ?, ?)`, old, old)
	mustExec(t, st, `INSERT INTO days (circle_id, entry_date, cover_blob_id)
		VALUES ('c1', '2026-08-01', 'blob-daycover')`)

	counts, err := jobs.RunDailyRoutine(ctx, st.DB(), blobsDir, now)
	if err != nil {
		t.Fatalf("RunDailyRoutine: %v", err)
	}
	if counts.OrphanedBlobs != 1 {
		t.Fatalf("orphaned blobs = %d, want 1 (только стёртое имя)", counts.OrphanedBlobs)
	}
	for _, keep := range []struct{ name, path string }{
		{"аватар", avatarPath},
		{"обложка дня", dayCoverPath},
	} {
		if _, err := os.Stat(keep.path); err != nil {
			t.Fatalf("%s удалён рутиной: %v", keep.name, err)
		}
		var n int
		if err := st.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM blobs WHERE storage_path = ?`,
			filepath.ToSlash(keep.path[len(blobsDir)+1:])).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("%s: строка blobs пропала", keep.name)
		}
	}
	if _, err := os.Stat(erasedPath); !os.IsNotExist(err) {
		t.Fatalf("аватар стёртого имени должен был уйти: %v", err)
	}
}

// Инвариант (BLB-2): скриншот оплаты держит строку blobs внешним ключом.
// Раньше рутина сначала удаляла файл, потом падала на DELETE и обрывала все
// оставшиеся шаги — навсегда.
func TestRunDailyRoutineSurvivesPayScreenshotAndRunsEveryStep(t *testing.T) {
	st := openDB(t)
	ctx := context.Background()
	blobsDir := t.TempDir()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	expired := now.Add(-time.Hour).Format(time.RFC3339Nano)

	insertAccount(t, st, "member", "member@test.local", old)
	payPath := seedBlobFile(t, st, blobsDir, "blob-pay", "pa/y", old)
	mustExec(t, st, `INSERT INTO pay_requests (id, account_id, blob_id, comment, status, created_at)
		VALUES ('pr1', 'member', 'blob-pay', '', 'pending', ?)`, old)

	// Шаги после сборщика сирот: их отмена и была главным следствием.
	mustExec(t, st, `INSERT INTO invites (id, token, kind, max_uses, uses, expires_at, created_at)
		VALUES ('inv-old', 'tok-old', 'single', 1, 0, ?, ?)`, expired, old)
	mustExec(t, st, `INSERT INTO sessions (token_hash, account_id, kind, expires_at, created_at)
		VALUES ('tok', 'member', 'participant', ?, ?)`, expired, old)

	counts, err := jobs.RunDailyRoutine(ctx, st.DB(), blobsDir, now)
	if err != nil {
		t.Fatalf("RunDailyRoutine: %v", err)
	}
	if _, err := os.Stat(payPath); err != nil {
		t.Fatalf("скриншот оплаты потерян: %v", err)
	}
	if counts.ExpiredInvites != 1 || counts.ExpiredSessions != 1 {
		t.Fatalf("шаги после сборщика сирот не выполнены: %+v", counts)
	}
	var lastRoutine string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT last_routine_at FROM instance_settings WHERE id = 1`).Scan(&lastRoutine); err != nil {
		t.Fatal(err)
	}
	if lastRoutine == "" {
		t.Fatal("last_routine_at не записан")
	}
}

// Инвариант (REF-3): сбой одного шага не отменяет остальные, ошибка при этом
// не проглатывается, а last_routine_at пишется всё равно.
func TestRunDailyRoutineReportsFailureButFinishesRest(t *testing.T) {
	st := openDB(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	expired := now.Add(-time.Hour).Format(time.RFC3339Nano)

	insertAccount(t, st, "member", "member@test.local", old)
	mustExec(t, st, `INSERT INTO invites (id, token, kind, max_uses, uses, expires_at, created_at)
		VALUES ('inv-old', 'tok-old', 'single', 1, 0, ?, ?)`, expired, old)
	// Ломаем первый шаг: сборщик сирот не сможет прочитать свою таблицу.
	mustExec(t, st, `DROP TABLE blob_refs`)

	counts, err := jobs.RunDailyRoutine(ctx, st.DB(), t.TempDir(), now)
	if err == nil {
		t.Fatal("сбой шага должен быть возвращён")
	}
	if counts.ExpiredInvites != 1 {
		t.Fatalf("остальные шаги не выполнены: %+v (err=%v)", counts, err)
	}
	var lastRoutine string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT last_routine_at FROM instance_settings WHERE id = 1`).Scan(&lastRoutine); err != nil {
		t.Fatal(err)
	}
	if lastRoutine == "" {
		t.Fatal("last_routine_at не записан при частичном сбое")
	}
}

func mustExec(t *testing.T, st *store.SQLite, query string, args ...any) {
	t.Helper()
	if _, err := st.DB().ExecContext(t.Context(), query, args...); err != nil {
		t.Fatalf("exec %.40s: %v", query, err)
	}
}
