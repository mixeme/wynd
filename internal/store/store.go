// Package store is the persistence boundary.
//
// SQLite is the only database driver. One Wynd process owns one database file
// (no shared pool across instances). Store is a narrow boundary
// (Open/Close/Ping/Version); domain code uses *SQLite via New().
package store

import (
	"context"
	"fmt"
)

// Store is a migrated database.
type Store interface {
	Close() error
	Ping(ctx context.Context) error
	Version(ctx context.Context) (int, error)
}

// Open opens (or creates) a SQLite database at path, applies migrations, and
// returns it as a Store.
func Open(path string) (Store, error) {
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		return nil, err
	}
	if err := migrate(s.db); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// OpenMemory opens an isolated in-memory SQLite database, applies migrations,
// and returns it as a Store. Intended for tests.
func OpenMemory() (Store, error) {
	s, err := openSQLite(memoryDSN(), false)
	if err != nil {
		return nil, err
	}
	if err := migrate(s.db); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}
