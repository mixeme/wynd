package store

import (
	"path/filepath"
	"testing"
)

// Миграция не угадывает согласие за прежние передачи: существующие круги
// и готовые копии сохраняют прежнее право, данные и состояние не меняются.
func TestArchivePreservationMigrationFrom12(t *testing.T) {
	file := filepath.Join(t.TempDir(), "wynd.db")
	old, err := openSQLite(fileDSN(file), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range migrations {
		if m.version > 12 {
			break
		}
		if err := applyMigration(old.db, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := old.db.Exec(`INSERT INTO accounts(id,email,created_at) VALUES ('owner','owner@example.com','2026-11-02T12:00:00Z');
		INSERT INTO circles(id,name,owner_account_id,created_at,updated_at) VALUES ('circle','Семья','owner','2026-11-02T12:00:00Z','2026-11-02T12:00:00Z');
		INSERT INTO circle_archives(id,circle_id,requested_by_account_id,requested_at,snapshot_at,status,size_bytes)
		VALUES ('file','circle','owner','2026-11-02T12:00:00Z','2026-11-02T12:01:00Z','ready',1234)`); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	current, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	db := current.(*SQLite).db
	var all, revision, full, scope, posts, size int
	var status, snapshot string
	if err := db.QueryRow(`SELECT c.archive_all,c.archive_revision,a.full_journal,a.scope_revision,a.posts,a.status,a.size_bytes,a.snapshot_at FROM circles c JOIN circle_archives a ON a.circle_id=c.id`).Scan(&all, &revision, &full, &scope, &posts, &status, &size, &snapshot); err != nil {
		t.Fatal(err)
	}
	if all != 1 || revision != 0 || full != 1 || scope != 0 || posts != -1 || status != "ready" || size != 1234 || snapshot != "2026-11-02T12:01:00Z" {
		t.Fatalf("migration changed history: %d %d %d %d %d %s %d %s", all, revision, full, scope, posts, status, size, snapshot)
	}
}
