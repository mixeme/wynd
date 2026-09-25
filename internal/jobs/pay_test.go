package jobs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

func TestPayScreenshotCleanupKeepsBlobRow(t *testing.T) {
	st := openDB(t)
	ctx := t.Context()
	ch, err := chronicle.New(st)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(st, ch, auth.NewCaptureCodes(), true)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	past := xtime.Format(now.Add(-24 * time.Hour))
	created := xtime.Format(now.Add(-48 * time.Hour))
	blobsDir := t.TempDir()
	rel := "pay/shot.jpg"
	path := filepath.Join(blobsDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("shot"), 0o640); err != nil {
		t.Fatal(err)
	}

	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO accounts (id, email, created_at, subscription_expires_at)
		VALUES ('payer', 'payer@test.local', ?, ?)
	`, created, past)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		VALUES ('blob-pay', 'payer', 'abc', 4, 'image/jpeg', ?, 'complete', ?)
	`, rel, created)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO pay_requests (id, account_id, blob_id, status, created_at, resolved_at)
		VALUES ('req-1', 'payer', 'blob-pay', 'approved', ?, ?)
	`, created, created)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB().ExecContext(ctx, `
		INSERT INTO blob_refs (blob_id, ref_type, ref_id)
		VALUES ('blob-pay', 'pay_request', 'req-1')
	`)
	if err != nil {
		t.Fatal(err)
	}

	n, err := svc.CleanupExpiredPayScreenshots(ctx, blobsDir, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted %d, want 1", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("screenshot file still exists: %v", err)
	}
	var deleted int
	if err := st.DB().QueryRowContext(ctx, `
		SELECT blob_deleted FROM pay_requests WHERE id = 'req-1'
	`).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("blob_deleted: %d", deleted)
	}
	var blobID string
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM blobs WHERE id = 'blob-pay'`).Scan(&blobID); err != nil {
		t.Fatalf("blob row should remain: %v", err)
	}
}
