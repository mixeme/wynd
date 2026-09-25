package chronicle

import (
	"database/sql"
	"fmt"

	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// Chronicle is the event-sourced circle core.
type Chronicle struct {
	db *sql.DB
}

// New wraps a migrated SQLite store.
func New(st store.Store) (*Chronicle, error) {
	s, ok := st.(*store.SQLite)
	if !ok {
		return nil, fmt.Errorf("chronicle: store must be *store.SQLite")
	}
	db := s.DB()
	if db == nil {
		return nil, fmt.Errorf("chronicle: closed store")
	}
	return &Chronicle{db: db}, nil
}

// DB exposes the underlying connection for tests.
func (c *Chronicle) DB() *sql.DB {
	return c.db
}
