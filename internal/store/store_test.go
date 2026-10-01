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
	if version != 24 {
		t.Fatalf("schema version: got %d, want 24", version)
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
	if version != 24 {
		t.Fatalf("schema version: got %d, want 24", version)
	}

	s := st.(*SQLite)
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if n != 24 {
		t.Fatalf("schema_migrations rows: got %d, want 24", n)
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
	v, err := parseMigrationVersion("0001_schema.sql")
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

func TestRejectsStaleSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	if _, err := s.db.Exec(`
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);
		INSERT INTO schema_migrations (version, applied_at) VALUES (99, '2026-01-01T00:00:00Z');
	`); err != nil {
		_ = s.Close()
		t.Fatalf("seed stale version: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = Open(path)
	if err == nil {
		t.Fatal("Open: want error for unknown schema version 99")
	}
	if !strings.Contains(err.Error(), "удалите wynd.db") {
		t.Fatalf("error: %v", err)
	}
}

func TestSchemaAllowsNullableIdentityAccountID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	s := st.(*SQLite)
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

	if _, err := s.db.Exec(`UPDATE identities SET account_id = NULL WHERE id = 'id-1'`); err != nil {
		t.Fatalf("nullable account_id: %v", err)
	}

	var accountID *string
	if err := s.db.QueryRow(`SELECT account_id FROM identities WHERE id = 'id-1'`).Scan(&accountID); err != nil {
		t.Fatalf("read identity: %v", err)
	}
	if accountID != nil {
		t.Fatalf("account_id: got %q, want NULL", *accountID)
	}

	var memCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM memberships WHERE identity_id = 'id-1'`).Scan(&memCount); err != nil {
		t.Fatalf("membership count: %v", err)
	}
	if memCount != 1 {
		t.Fatalf("membership rows: got %d, want 1", memCount)
	}
}
