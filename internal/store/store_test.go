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
	if version != 11 {
		t.Fatalf("schema version: got %d, want 11", version)
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
	if version != 11 {
		t.Fatalf("schema version: got %d, want 11", version)
	}

	s := st.(*SQLite)
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if n != 11 {
		t.Fatalf("schema_migrations rows: got %d, want 11", n)
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
