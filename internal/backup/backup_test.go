package backup_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/backup"
	"gitea.mixdep.ru/mix/wynd/internal/store"

	_ "modernc.org/sqlite"
)

func seedDataDir(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	for _, dir := range []string{dataDir, filepath.Join(dataDir, "blobs"), filepath.Join(dataDir, "keys")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	cfg := []byte(`{"listen":":7676","public_url":"http://127.0.0.1:7676"}` + "\n")
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), cfg, 0o640); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "wynd.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	return dataDir
}

func writeBlob(t *testing.T, dataDir, rel, payload string) {
	t.Helper()
	path := filepath.Join(dataDir, "blobs", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(payload), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestBackupFullAndIncremental(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	dataDir := seedDataDir(t)
	destDir := t.TempDir()
	writeBlob(t, dataDir, "aa/first.bin", "one")
	writeBlob(t, dataDir, "bb/second.bin", "two")

	if err := backup.Backup(dataDir, destDir, false); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"config.json", "wynd.db", "blobs/aa/first.bin", "blobs/bb/second.bin", "blobs/manifest.json"} {
		if _, err := os.Stat(filepath.Join(destDir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}

	writeBlob(t, dataDir, "cc/third.bin", "three")
	firstInfo, err := os.Stat(filepath.Join(destDir, "blobs/aa/first.bin"))
	if err != nil {
		t.Fatal(err)
	}

	if err := backup.Backup(dataDir, destDir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "blobs/cc/third.bin")); err != nil {
		t.Fatalf("incremental blob missing: %v", err)
	}
	secondInfo, err := os.Stat(filepath.Join(destDir, "blobs/aa/first.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !firstInfo.ModTime().Equal(secondInfo.ModTime()) {
		t.Fatal("incremental backup rewrote unchanged blob")
	}

	var manifest struct {
		Files map[string]any `json:"files"`
	}
	data, err := os.ReadFile(filepath.Join(destDir, "blobs/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 3 {
		t.Fatalf("manifest files: %d", len(manifest.Files))
	}
}

func TestBackupUpdatesLastBackupAt(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	dataDir := seedDataDir(t)
	destDir := t.TempDir()
	before := time.Now().UTC().Add(-time.Minute)

	if err := backup.Backup(dataDir, destDir, false); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dataDir, "wynd.db")
	stFile, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stFile.Close()
	var raw string
	if err := stFile.(*store.SQLite).DB().QueryRow(`SELECT last_backup_at FROM instance_settings WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		t.Fatal("last_backup_at empty")
	}
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Before(before) {
		t.Fatalf("last_backup_at too old: %s", raw)
	}
}

func TestBackupCapturesWALData(t *testing.T) {
	dataDir := seedDataDir(t)
	destDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "wynd.db")

	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	const marker = "backup-wal-marker"
	if _, err := db.Exec(`UPDATE instance_settings SET name = ? WHERE id = 1`, marker); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); err != nil {
		t.Skip("WAL file not created in this environment")
	}

	if err := backup.Backup(dataDir, destDir, false); err != nil {
		t.Fatal(err)
	}

	backupDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(destDir, "wynd.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	var name string
	if err := backupDB.QueryRow(`SELECT name FROM instance_settings WHERE id = 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != marker {
		t.Fatalf("backup db missing WAL-only write: got %q", name)
	}
	if _, err := os.Stat(filepath.Join(destDir, "wynd.db-wal")); !os.IsNotExist(err) {
		t.Fatal("backup should not copy WAL sidecar")
	}
}
