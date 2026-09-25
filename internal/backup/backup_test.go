package backup_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

// Инвариант (аудит 2026-09-22, условие DEC-1): копия wynd.db и keys/* в
// бэкапе доступны только владельцу. VACUUM INTO создавал файл с правами
// SQLite по умолчанию, а keys/bootstrap копировался 0600 → 0640.
// На Windows права POSIX не выражаются — проверка только на Unix.
func TestBackupKeepsSecretsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-права недоступны на Windows")
	}
	dataDir := seedDataDir(t)
	if err := os.WriteFile(filepath.Join(dataDir, "keys", "bootstrap"), []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	destDir := filepath.Join(t.TempDir(), "out")
	if err := backup.Backup(dataDir, destDir, false); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"wynd.db", "keys/bootstrap"} {
		info, err := os.Stat(filepath.Join(destDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Fatalf("%s: права %o, ожидались только владельцу", rel, perm)
		}
	}
	info, err := os.Stat(destDir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("каталог бэкапа: права %o", perm)
	}
}

// Инвариант: инкрементальный прогон чинит копию, усечённую обрывом прошлого
// прогона. Раньше он сверял источник только с манифестом и пропускал файл,
// который манифест помнил целым (план 42, BKP-2).
func TestBackupIncrementalRepairsTruncatedCopy(t *testing.T) {
	dataDir := seedDataDir(t)
	destDir := t.TempDir()
	const payload = "hello world payload"
	writeBlob(t, dataDir, "ab/one", payload)
	if err := backup.Backup(dataDir, destDir, false); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(destDir, "blobs", "ab", "one")
	if err := os.WriteFile(copyPath, []byte("hel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := backup.Backup(dataDir, destDir, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != payload {
		t.Fatalf("copy still truncated after incremental run: %q", got)
	}
}

// Инвариант: блоб неизменяем, поэтому инкрементальный прогон верит манифесту
// по имени и размеру и не перечитывает хранилище (BKP-1): подменённое
// содержимое той же длины копию не трогает.
func TestBackupIncrementalTrustsManifestBySize(t *testing.T) {
	dataDir := seedDataDir(t)
	destDir := t.TempDir()
	writeBlob(t, dataDir, "ab/one", "one")
	if err := backup.Backup(dataDir, destDir, false); err != nil {
		t.Fatal(err)
	}
	writeBlob(t, dataDir, "ab/one", "uno")
	if err := backup.Backup(dataDir, destDir, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "blobs", "ab", "one"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one" {
		t.Fatalf("same-size blob was re-read and re-copied: %q", got)
	}
	if _, err := os.Stat(filepath.Join(destDir, "blobs", "ab", "one.part")); err == nil {
		t.Fatal("temporary .part file left behind")
	}
}

// Инвариант (план 42, BKP-8): копия, снятая wynd backup, — рабочий каталог
// данных: база открывается мигратором как есть и содержит записанное, блобы
// совпадают побайтно. README обещает это восстановлением «скопировать и
// запустить».
func TestBackupRestoresIntoWorkingDataDir(t *testing.T) {
	dataDir := seedDataDir(t)
	st, err := store.Open(filepath.Join(dataDir, "wynd.db"))
	if err != nil {
		t.Fatal(err)
	}
	db := st.(*store.SQLite).DB()
	if _, err := db.ExecContext(t.Context(), `
		INSERT INTO accounts (id, email, created_at) VALUES ('acc-restore', 'restore@example.com', '2026-09-24T00:00:00.000000000Z')
	`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	writeBlob(t, dataDir, "ab/photo", "photo-bytes")

	dest := t.TempDir()
	if err := backup.Backup(dataDir, dest, false); err != nil {
		t.Fatal(err)
	}
	// README: manifest.json в каталоге данных не нужен.
	if err := os.Remove(filepath.Join(dest, "blobs", "manifest.json")); err != nil {
		t.Fatal(err)
	}

	restored, err := store.Open(filepath.Join(dest, "wynd.db"))
	if err != nil {
		t.Fatalf("restored db does not open: %v", err)
	}
	defer restored.Close()
	var email string
	if err := restored.(*store.SQLite).DB().QueryRowContext(t.Context(),
		`SELECT email FROM accounts WHERE id = 'acc-restore'`).Scan(&email); err != nil {
		t.Fatalf("restored row: %v", err)
	}
	if email != "restore@example.com" {
		t.Fatalf("email = %q", email)
	}
	got, err := os.ReadFile(filepath.Join(dest, "blobs", "ab", "photo"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "photo-bytes" {
		t.Fatalf("blob = %q", got)
	}
}

// Инвариант (BKP-7): бэкап не пишется в сам каталог данных и внутрь blobs/
// или keys/ — там он перезаписал бы живую базу или копировал бы сам себя;
// отдельный подкаталог рядом (как `<data>/backups/…` у install.sh) можно.
func TestBackupRefusesDestinationInsideCopiedData(t *testing.T) {
	dataDir := seedDataDir(t)
	writeBlob(t, dataDir, "ab/one", "one")
	for _, dest := range []string{
		dataDir,
		filepath.Join(dataDir, "blobs", "copy"),
		filepath.Join(dataDir, "keys", "copy"),
	} {
		if err := backup.Backup(dataDir, dest, false); !errors.Is(err, backup.ErrBadDestination) {
			t.Fatalf("%s: %v, want ErrBadDestination", dest, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "blobs", "copy")); !os.IsNotExist(err) {
		t.Fatalf("каталог назначения создан: %v", err)
	}
	if err := backup.Backup(dataDir, filepath.Join(dataDir, "backups", "pre-update"), false); err != nil {
		t.Fatalf("соседний подкаталог: %v", err)
	}
}
