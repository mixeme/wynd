package jobs_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/jobs"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func openDB(t *testing.T) *store.SQLite {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st.(*store.SQLite)
}

func insertAccount(t *testing.T, db *store.SQLite, id, email, created string) {
	t.Helper()
	_, err := db.DB().ExecContext(t.Context(), `
		INSERT INTO accounts (id, email, created_at) VALUES (?, ?, ?)
	`, id, email, created)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunDailyRoutineCleansTargets(t *testing.T) {
	st := openDB(t)
	ctx := context.Background()
	blobsDir := t.TempDir()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	soon := now.Add(-time.Hour).Format(time.RFC3339Nano)

	insertAccount(t, st, "admin", auth.AdminSentinelEmail, old)
	insertAccount(t, st, "empty", "empty@test.local", old)
	insertAccount(t, st, "member", "member@test.local", old)

	_, err := st.DB().ExecContext(ctx, `
		INSERT INTO circles (id, name, owner_account_id, created_at, updated_at)
		VALUES ('c1', 'Test', 'member', ?, ?)
	`, old, old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO identities (id, circle_id, account_id, created_at)
		VALUES ('id1', 'c1', 'member', ?)
	`, old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO memberships (id, circle_id, account_id, identity_id, status, created_at, updated_at)
		VALUES ('m1', 'c1', 'member', 'id1', 'active', ?, ?)
	`, old, old)
	if err != nil {
		t.Fatal(err)
	}

	orphRel := "aa/orphan"
	orphPath := filepath.Join(blobsDir, filepath.FromSlash(orphRel))
	if err := os.MkdirAll(filepath.Dir(orphPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphPath, []byte("orphan"), 0o640); err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES ('blob-orphan', 'member', 'deadbeef', 6, 'text/plain', ?, 'complete', ?)
	`, orphRel, old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES ('blob-fresh', 'member', 'deadbeef', 6, 'text/plain', 'bb/fresh', 'complete', ?)
	`, soon)
	if err != nil {
		t.Fatal(err)
	}

	uploadsDir := filepath.Join(blobsDir, ".uploads")
	if err := os.MkdirAll(uploadsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadsDir, "sess-old.part"), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadsDir, "sess-new.part"), []byte("y"), 0o640); err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO upload_sessions (id, account_id, expected_size, mime_type, received_bytes, expires_at, created_at)
		VALUES
			('sess-old', 'member', 1, 'text/plain', 0, ?, ?),
			('sess-new', 'member', 1, 'text/plain', 0, ?, ?)
	`, now.Add(-time.Hour).Format(time.RFC3339Nano), old,
		now.Add(time.Hour).Format(time.RFC3339Nano), soon)
	if err != nil {
		t.Fatal(err)
	}

	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO invites (id, token, kind, max_uses, uses, expires_at, created_at)
		VALUES
			('inv-old', 'tok-old', 'single', 1, 0, ?, ?),
			('inv-new', 'tok-new', 'single', 1, 0, ?, ?)
	`, now.Add(-time.Hour).Format(time.RFC3339Nano), old,
		now.Add(time.Hour).Format(time.RFC3339Nano), soon)
	if err != nil {
		t.Fatal(err)
	}

	counts, err := jobs.RunDailyRoutine(ctx, st.DB(), blobsDir, now)
	if err != nil {
		t.Fatal(err)
	}
	if counts.OrphanedBlobs != 1 || counts.AbandonedUploads != 1 || counts.EmptyAccounts != 1 || counts.ExpiredInvites != 1 {
		t.Fatalf("counts: %+v", counts)
	}
	if _, err := os.Stat(orphPath); !os.IsNotExist(err) {
		t.Fatalf("orphan file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(uploadsDir, "sess-old.part")); !os.IsNotExist(err) {
		t.Fatalf("old upload part still exists: %v", err)
	}

	var lastRoutine string
	if err := st.DB().QueryRowContext(ctx, `SELECT last_routine_at FROM instance_settings WHERE id = 1`).Scan(&lastRoutine); err != nil {
		t.Fatal(err)
	}
	if lastRoutine == "" {
		t.Fatal("last_routine_at not updated")
	}

	var emptyDeleted sql.NullString
	if err := st.DB().QueryRowContext(ctx, `SELECT deleted_at FROM accounts WHERE id = 'empty'`).Scan(&emptyDeleted); err != nil {
		t.Fatal(err)
	}
	if !emptyDeleted.Valid || emptyDeleted.String == "" {
		t.Fatal("empty account should be soft-deleted, not removed")
	}
}

func TestRunDailyRoutineExpiresUploadInSameSecond(t *testing.T) {
	st := openDB(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 30, 12, 0, 0, 500000000, time.UTC)
	expires := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	_, err := st.DB().ExecContext(ctx, `
		INSERT INTO accounts (id, email, created_at) VALUES ('member', 'member@test.local', ?)
	`, expires)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO upload_sessions (id, account_id, expected_size, mime_type, received_bytes, expires_at, created_at)
		VALUES ('sess-exp', 'member', 1, 'text/plain', 0, ?, ?)
	`, expires, expires)
	if err != nil {
		t.Fatal(err)
	}

	counts, err := jobs.RunDailyRoutine(ctx, st.DB(), t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	if counts.AbandonedUploads != 1 {
		t.Fatalf("abandoned uploads: %+v", counts)
	}
}
