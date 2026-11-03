package blob

import (
	"context"
	"database/sql"
	"fmt"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

const uploadSessionTTL = 24 * 60 * 60 // seconds

// Store manages blob files on disk and metadata in SQLite.
type Store struct {
	db  *sql.DB
	dir string
}

// New wraps a migrated SQLite store and blobs directory.
func New(st store.Store, blobsDir string) (*Store, error) {
	s, ok := st.(*store.SQLite)
	if !ok {
		return nil, fmt.Errorf("blob: store must be *store.SQLite")
	}
	db := s.DB()
	if db == nil {
		return nil, fmt.Errorf("blob: closed store")
	}
	if blobsDir == "" {
		return nil, fmt.Errorf("blob: empty blobs dir")
	}
	b := &Store{db: db, dir: blobsDir}
	if err := b.migrateContents(context.Background()); err != nil {
		return nil, fmt.Errorf("blob content migration: %w", err)
	}
	return b, nil
}

// DB exposes the underlying connection for tests.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Dir returns the blobs root directory.
func (s *Store) Dir() string {
	return s.dir
}
