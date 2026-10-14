package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// База, прошедшая прежнюю цепочку 0001–0030 по шагам, несёт тридцать отметок.
// Свёрнутая схема стоит под номером 0030 и поверх такой базы не применяется.
func TestMigrateKeepsDatabaseOfFormerChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for v := 1; v < 30; v++ {
		if _, err := s.db.Exec(
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, '2026-10-08T00:00:00.000000000Z')`, v,
		); err != nil {
			t.Fatalf("mark %d: %v", v, err)
		}
	}
	if _, err := s.db.Exec(
		`INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '2026-08-30T09:00:00.000000000Z')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate over former chain: %v", err)
	}
	var accounts int
	if err := s.db.QueryRow(`SELECT count(*) FROM accounts`).Scan(&accounts); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accounts != 1 {
		t.Fatalf("данные не пережили запуск: accounts = %d", accounts)
	}
}

// Инвариант (TST-4): миграция применяется к заполненной базе прежней версии.
// 0031 пересоздаёт pending_codes: невведённые коды гаснут, остальное цело, а
// новая таблица принимает код смены почты, привязанный к учётке.
func TestMigrateEmailChangeOverPopulatedBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()
	all, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if _, err := s.db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if all[0].version != 30 {
		t.Fatalf("первая миграция: %d", all[0].version)
	}
	if err := applyMigration(s.db, all[0]); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '2026-08-30T09:00:00.000000000Z');
		INSERT INTO pending_codes (id, email, code_hash, client_ip, flow, expires_at, created_at)
		VALUES ('p1', 'a@test.local', 'h', '127.0.0.1', 'login', '2026-08-30T09:15:00.000000000Z', '2026-08-30T09:00:00.000000000Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate over populated baseline: %v", err)
	}
	var accounts, codes int
	if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM accounts), (SELECT count(*) FROM pending_codes)`).Scan(&accounts, &codes); err != nil {
		t.Fatalf("count: %v", err)
	}
	if accounts != 1 || codes != 0 {
		t.Fatalf("accounts = %d, pending_codes = %d", accounts, codes)
	}
	if _, err := s.db.Exec(`
		INSERT INTO pending_codes (id, email, code_hash, client_ip, flow, account_id, expires_at, created_at)
		VALUES ('p2', 'b@test.local', 'h', '127.0.0.1', 'email_change', 'acc', '2026-08-30T09:15:00.000000000Z', '2026-08-30T09:00:00.000000000Z')
	`); err != nil {
		t.Fatalf("код смены почты не принят: %v", err)
	}
}

// База, остановившаяся в середине прежней цепочки, не доводится: шагов до
// свёрнутой схемы в бинарнике нет. Отказ называет версию, которая их несёт.
func TestMigrateRefusesDatabaseBelowBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()
	if _, err := s.db.Exec(`
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);
		INSERT INTO schema_migrations (version, applied_at) VALUES (16, '2026-09-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err = migrate(s.db)
	if err == nil {
		t.Fatal("база версии 16 должна получать отказ")
	}
	if !strings.Contains(err.Error(), "0.25.10") {
		t.Fatalf("err = %v", err)
	}
}

// Инвариант (TST-4): 0032 добавляет переключатель стирания имён к уже
// заполненной базе — выключенным, остальные настройки сервера целы.
func TestMigrateNameErasureOverPopulatedBase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()
	all, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if _, err := s.db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	for _, m := range all {
		if m.version >= 32 {
			break
		}
		if err := applyMigration(s.db, m); err != nil {
			t.Fatalf("migration %d: %v", m.version, err)
		}
	}
	if _, err := s.db.Exec(`UPDATE instance_settings SET name = 'Дом Ани', registration_mode = 'open' WHERE id = 1`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate over populated base: %v", err)
	}
	var name, mode string
	var allow int
	if err := s.db.QueryRow(`SELECT name, registration_mode, allow_name_erasure FROM instance_settings WHERE id = 1`).Scan(&name, &mode, &allow); err != nil {
		t.Fatalf("read: %v", err)
	}
	if name != "Дом Ани" || mode != "open" || allow != 0 {
		t.Fatalf("name = %q, mode = %q, allow_name_erasure = %d", name, mode, allow)
	}
}
