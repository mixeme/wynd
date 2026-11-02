package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// Обновление существующей базы не отзывает просьбы и не теряет решения администратора.
func TestPlaceWithdrawalMigrationFrom11(t *testing.T) {
	file := filepath.Join(t.TempDir(), "wynd.db")
	old, err := openSQLite(fileDSN(file), true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range migrations {
		if m.version > 11 {
			break
		}
		if err := applyMigration(old.db, m); err != nil {
			t.Fatal(err)
		}
	}
	_, err = old.db.Exec(`INSERT INTO accounts (id, email, created_at) VALUES ('owner', 'owner@example.com', '2026-11-02T12:00:00Z');
		INSERT INTO quota_requests (id, circle_name, requested_by_account_id, requested_bytes, status, created_at)
		VALUES ('existing', 'Семья', 'owner', 1000, 'approved', '2026-11-02T12:00:00Z')`)
	if err != nil {
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
	var status string
	var withdrawn, operation sql.NullString
	var size int64
	err = current.(*SQLite).db.QueryRow(`SELECT status, requested_bytes, withdrawn_at, operation_key FROM quota_requests WHERE id = 'existing'`).Scan(&status, &size, &withdrawn, &operation)
	if err != nil {
		t.Fatal(err)
	}
	if status != "approved" || size != 1000 || withdrawn.Valid || operation.Valid {
		t.Fatalf("changed existing request: %s %d %v %v", status, size, withdrawn, operation)
	}
}
