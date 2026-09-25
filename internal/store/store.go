// Package store is the persistence boundary.
//
// SQLite is the only driver in this plan. Store exists so a PostgreSQL
// driver can be added later without rewriting callers.
package store

import (
	"context"
	"fmt"
)

// Store is a migrated database. Domain methods are added in later stages.
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
