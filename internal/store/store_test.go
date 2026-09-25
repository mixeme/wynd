package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenMigrateClose(t *testing.T) {
	st, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}

	ctx := context.Background()
	if err := st.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	version, err := st.Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if version != 13 {
		t.Fatalf("schema version: got %d, want 13", version)
	}

	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := st.Ping(ctx); err == nil {
		t.Fatal("Ping after Close: want error")
	}
}

func TestReopenAppliesMigrationsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	ctx := context.Background()

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	st, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st.Close()

	version, err := st.Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if version != 13 {
		t.Fatalf("schema version: got %d, want 13", version)
	}

	s := st.(*SQLite)
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if n != 13 {
		t.Fatalf("schema_migrations rows: got %d, want 13", n)
	}
}

func TestSQLitePragmas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	s := st.(*SQLite)
	ctx := context.Background()

	var timeout int
	if err := s.db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if timeout != busyTimeoutMS {
		t.Fatalf("busy_timeout: got %d, want %d", timeout, busyTimeoutMS)
	}

	var fk int
	if err := s.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys: got %d, want 1", fk)
	}

	var mode string
	if err := s.db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode: got %q, want wal", mode)
	}
}

func TestParseMigrationVersion(t *testing.T) {
	v, err := parseMigrationVersion("0001_init.sql")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v != 1 {
		t.Fatalf("version: got %d, want 1", v)
	}
	if _, err := parseMigrationVersion("init.sql"); err == nil {
		t.Fatal("want error for missing version")
	}
}

func TestMigration0013WithMembershipReferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer s.Close()

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	for _, m := range migrations {
		if m.version >= 13 {
			break
		}
		if err := applyMigration(s.db, m); err != nil {
			t.Fatalf("apply %s: %v", m.name, err)
		}
	}

	if _, err := s.db.Exec(`
		INSERT INTO accounts (id, email, created_at)
		VALUES ('acct-1', 'alice@example.com', '2026-01-01T00:00:00Z');
		INSERT INTO circles (id, name, owner_account_id, created_at, updated_at)
		VALUES ('circle-1', 'Test', 'acct-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
		INSERT INTO identities (id, circle_id, account_id, created_at)
		VALUES ('id-1', 'circle-1', 'acct-1', '2026-01-01T00:00:00Z');
		INSERT INTO identity_names (id, identity_id, name, effective_at)
		VALUES ('name-1', 'id-1', 'Alice', '2026-01-01T00:00:00Z');
		INSERT INTO memberships (id, circle_id, account_id, identity_id, can_settings, status, created_at, updated_at)
		VALUES ('mem-1', 'circle-1', 'acct-1', 'id-1', 0, 'active', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed data: %v", err)
	}

	var m13 migration
	for _, m := range migrations {
		if m.version == 13 {
			m13 = m
			break
		}
	}
	if m13.version != 13 {
		t.Fatal("migration 13 not found")
	}

	if err := applyMigration(s.db, m13); err != nil {
		t.Fatalf("apply 0013: %v", err)
	}
}
