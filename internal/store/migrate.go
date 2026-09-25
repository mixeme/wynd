package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

const migrationsDir = "migrations"

type migration struct {
	version int
	name    string
	sql     string
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var maxVersion int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxVersion); err != nil {
		return fmt.Errorf("schema version: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	known := map[int]bool{0: true}
	for _, m := range migrations {
		known[m.version] = true
	}
	if !known[maxVersion] {
		return fmt.Errorf("устаревшая схема БД (версия %d): удалите wynd.db и запустите снова", maxVersion)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		// Пропущенная версия ниже уже применённых — отказ запуска, а не
		// применение поверх более новой схемы: середина цепочки не
		// восстанавливается задним числом, и «доливка» портит данные (MIG-1).
		if m.version < maxVersion {
			return fmt.Errorf(
				"миграция %s пропущена, а схема уже версии %d: восстановите базу из бэкапа",
				m.name, maxVersion)
		}
		if err := applyMigration(db, m); err != nil {
			return err
		}
	}
	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.Glob(migrationFS, migrationsDir+"/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}

	out := make([]migration, 0, len(entries))
	seen := make(map[int]string, len(entries))
	for _, name := range entries {
		version, err := parseMigrationVersion(path.Base(name))
		if err != nil {
			return nil, err
		}
		if prev, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", version, prev, name)
		}
		seen[version] = name

		body, err := fs.ReadFile(migrationFS, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		out = append(out, migration{
			version: version,
			name:    path.Base(name),
			sql:     string(body),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func parseMigrationVersion(filename string) (int, error) {
	base, ok := strings.CutSuffix(filename, ".sql")
	if !ok {
		return 0, fmt.Errorf("migration %q: expected .sql suffix", filename)
	}
	num, _, ok := strings.Cut(base, "_")
	if !ok || num == "" {
		return 0, fmt.Errorf("migration %q: expected NNNN_name.sql", filename)
	}
	version, err := strconv.Atoi(num)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("migration %q: invalid version", filename)
	}
	return version, nil
}

func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	return applied, nil
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin %s: %w", m.name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(m.sql); err != nil {
		return fmt.Errorf("apply %s: %w", m.name, err)
	}

	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		m.version,
		time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"),
	); err != nil {
		return fmt.Errorf("record %s: %w", m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", m.name, err)
	}
	return nil
}
