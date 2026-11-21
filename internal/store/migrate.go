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
	if maxVersion, err = renumberFormerNumberings(db, maxVersion); err != nil {
		return err
	}
	known := map[int]bool{0: true}
	for _, m := range migrations {
		known[m.version] = true
	}
	if !known[maxVersion] {
		return fmt.Errorf("схема БД версии %d новее этой программы", maxVersion)
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

// Прежняя нумерация (0.34.0–0.53.4): шаги 0001–0017, точка свёртки — 17.
// Хранится в migrations/legacy/ и поднимается только тестами.
const (
	// Маяк новой нумерации (C115): отметка 0 в schema_migrations, её ставит
	// только базовая схема свёртки. Прежняя нумерация 1–17 и разметка 1
	// (отметки 30–36) её не ставили.
	baselineMark = 0
	// Последняя схема прежней нумерации — точка свёртки.
	squashedTop = 17
	// Последний выпуск прежней нумерации: доводит любую базу до 17
	// (разметку 1 — через перевод 0036 в «версию 1» прежней нумерации).
	squashPrevRelease = "0.53.4"
)

// Разметка 1 (до 0.34.0): свёрнутая схема 0030 и шаги 0031–0036 — её узнавал
// renumberLegacy прежней нумерации.
const (
	legacyBaseline = 30
	legacyTop      = 36
)

// renumberFormerNumberings узнаёт базу по отметкам и переводит её в новую
// нумерацию (C115). Новая отличается маяком — отметкой 0, которую ставит
// только базовая схема свёртки; свежая база без отметок проходит насквозь.
// Прежняя нумерация (1–17), дошедшая до 17, переводится одной отметкой:
// таблицы и данные не трогаются. Остановившаяся раньше — и базы разметки 1 —
// отказом с выпуском, который доведёт их до точки свёртки: шагов прежних
// нумераций в этом бинарнике нет.
func renumberFormerNumberings(db *sql.DB, maxVersion int) (int, error) {
	if maxVersion == 0 {
		return 0, nil
	}
	var mark int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = ?`, baselineMark).Scan(&mark); err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	if mark > 0 {
		return maxVersion, nil
	}
	// Разметка 1: отметка 0030 — её ставила свёртка 0.26.0.
	var legacy int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = ?`, legacyBaseline).Scan(&legacy); err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	if legacy > 0 {
		if maxVersion > legacyTop {
			return 0, fmt.Errorf("схема БД версии %d новее этой программы", maxVersion)
		}
		return 0, fmt.Errorf(
			"схема БД версии %d по разметке 1: запустите на этой базе сначала Wynd %s — он переведёт её на прежнюю нумерацию и доведёт до схемы 17, затем эту версию",
			maxVersion, squashPrevRelease)
	}
	if maxVersion > squashedTop {
		return 0, fmt.Errorf("схема БД версии %d новее этой программы", maxVersion)
	}
	if maxVersion < squashedTop {
		return 0, fmt.Errorf(
			"схема БД версии %d: запустите на этой базе сначала Wynd %s — он доведёт её до схемы 17, затем эту версию",
			maxVersion, squashPrevRelease)
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("renumber schema: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM schema_migrations`); err != nil {
		return 0, fmt.Errorf("renumber schema: %w", err)
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?), (?, ?)`,
		baselineMark, "1970-01-01T00:00:00.000000000Z",
		1, now,
	); err != nil {
		return 0, fmt.Errorf("renumber schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("renumber schema: %w", err)
	}
	return 1, nil
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
