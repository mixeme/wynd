package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"

	_ "modernc.org/sqlite"
)

const (
	driverName    = "sqlite"
	busyTimeoutMS = 5000
	pragmaTimeout = "busy_timeout(5000)"
	pragmaFK      = "foreign_keys(1)"
	pragmaJournal = "journal_mode(WAL)"
)

var memSeq atomic.Uint64

// SQLite is the only Store implementation.
type SQLite struct {
	db *sql.DB
}

var _ Store = (*SQLite)(nil)

func fileDSN(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return "file:" + filepath.ToSlash(abs) + "?" + sqliteQuery()
}

func memoryDSN() string {
	n := memSeq.Add(1)
	return fmt.Sprintf("file:wynd-mem-%d?mode=memory&cache=shared&%s", n, sqliteQuery())
}

func sqliteQuery() string {
	q := url.Values{}
	q.Add("_pragma", pragmaTimeout)
	q.Add("_pragma", pragmaFK)
	q.Add("_pragma", pragmaJournal)
	// Все транзакции — BEGIN IMMEDIATE: при повышении read→write SQLite
	// отдаёт SQLITE_BUSY сразу, и busy_timeout не помогает (QLT-2).
	q.Set("_txlock", "immediate")
	return q.Encode()
}

func openSQLite(dsn string, requireWAL bool) (*SQLite, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := checkPragmas(db, requireWAL); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLite{db: db}, nil
}

func checkPragmas(db *sql.DB, requireWAL bool) error {
	var timeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
		return fmt.Errorf("pragma busy_timeout: %w", err)
	}
	if timeout != busyTimeoutMS {
		return fmt.Errorf("busy_timeout: got %d, want %d", timeout, busyTimeoutMS)
	}

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if fk != 1 {
		return fmt.Errorf("foreign_keys: got %d, want 1", fk)
	}

	if !requireWAL {
		return nil
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return fmt.Errorf("pragma journal_mode: %w", err)
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("journal_mode: got %q, want wal", mode)
	}
	return nil
}

func (s *SQLite) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *SQLite) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return sql.ErrConnDone
	}
	return s.db.PingContext(ctx)
}

// DB exposes the underlying connection for domain packages (chronicle, etc.).
func (s *SQLite) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *SQLite) Version(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, sql.ErrConnDone
	}
	var version int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	return version, nil
}
