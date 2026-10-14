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

// Инвариант (C42): счёт текста круга, накопленный триггерами 0033, равен
// пересчёту той же миграцией с нуля — запись, правка, удаление, реакции,
// события и названия дня. Иначе база, пересчитанная при обновлении, и база,
// дожившая до того же состояния записями, расходились бы.
func TestTextQuotaTriggersMatchBackfill(t *testing.T) {
	s, err := openSQLite(fileDSN(filepath.Join(t.TempDir(), "wynd.db")), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	const ts = "2026-08-30T09:00:00.000000000Z"
	steps := []string{
		`INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '` + ts + `')`,
		`INSERT INTO circles (id, name, owner_account_id, created_at, updated_at) VALUES ('c1', 'Семья', 'acc', '` + ts + `', '` + ts + `')`,
		`INSERT INTO identities (id, circle_id, account_id, created_at) VALUES ('i1', 'c1', 'acc', '` + ts + `')`,
		`INSERT INTO events (id, circle_id, event_type, is_service, actor_identity_id, actor_name, payload, summary, created_at)
		 VALUES ('e1', 'c1', 'post.created', 0, 'i1', 'Аня', '{"body":"были на даче"}', 'Аня опубликовала запись', '` + ts + `')`,
		`INSERT INTO days (circle_id, entry_date) VALUES ('c1', '2026-08-30')`,
		`INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at)
		 VALUES ('p1', 'c1', 1, 'i1', 'Аня', 'были на даче', '2026-08-30', '` + ts + `')`,
		`INSERT INTO events (id, circle_id, event_type, is_service, actor_identity_id, actor_name, payload, summary, created_at)
		 VALUES ('e2', 'c1', 'comment.created', 0, 'i1', 'Аня', '{}', 'Аня оставила комментарий', '` + ts + `')`,
		`INSERT INTO comments (id, circle_id, post_id, event_seq, identity_id, author_name, body, created_at)
		 VALUES ('m1', 'c1', 'p1', 2, 'i1', 'Аня', 'и мы', '` + ts + `')`,
		`INSERT INTO events (id, circle_id, event_type, is_service, actor_identity_id, actor_name, payload, summary, created_at)
		 VALUES ('e3', 'c1', 'reaction.set', 0, 'i1', 'Аня', '{"emoji":"heart"}', 'Аня поставила реакцию', '` + ts + `')`,
		`INSERT INTO reactions (id, circle_id, post_id, event_seq, identity_id, author_name, emoji, created_at)
		 VALUES ('r1', 'c1', 'p1', 3, 'i1', 'Аня', 'heart', '` + ts + `')`,
		`INSERT INTO events (id, circle_id, event_type, is_service, actor_identity_id, actor_name, payload, summary, created_at)
		 VALUES ('e4', 'c1', 'day.titled', 0, 'i1', 'Аня', '{"title":"Дача"}', 'Аня назвала 30 августа «Дача»', '` + ts + `')`,
		`INSERT INTO day_titles (circle_id, entry_date, event_seq, identity_id, title, created_at)
		 VALUES ('c1', '2026-08-30', 4, 'i1', 'Дача', '` + ts + `')`,
		`UPDATE posts SET body = 'были на даче, все живы' WHERE id = 'p1'`,
		`UPDATE reactions SET emoji = 'laugh' WHERE id = 'r1'`,
		`UPDATE comments SET body = NULL, deleted = 1 WHERE id = 'm1'`,
		`UPDATE events SET payload = '{}', summary = '' WHERE id = 'e2'`,
		`DELETE FROM day_titles WHERE event_seq = 4`,
	}
	for _, q := range steps {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	var counted int64
	if err := s.db.QueryRow(`SELECT text_bytes FROM circles WHERE id = 'c1'`).Scan(&counted); err != nil {
		t.Fatal(err)
	}
	all, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var backfill string
	for _, m := range all {
		if m.version == 33 {
			start := strings.Index(m.sql, "UPDATE circles SET text_bytes =")
			end := strings.Index(m.sql[start:], ";")
			backfill = m.sql[start : start+end]
		}
	}
	if backfill == "" {
		t.Fatal("в 0033 нет пересчёта")
	}
	if _, err := s.db.Exec(backfill); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	var recomputed int64
	if err := s.db.QueryRow(`SELECT text_bytes FROM circles WHERE id = 'c1'`).Scan(&recomputed); err != nil {
		t.Fatal(err)
	}
	if counted <= 0 || counted != recomputed {
		t.Fatalf("триггеры насчитали %d, пересчёт — %d", counted, recomputed)
	}
}

// Инвариант (TST-4): 0033 на заполненной базе пересчитывает текст уже
// написанного — круг с записью после обновления не «пустой».
func TestMigrateTextQuotaOverPopulatedBase(t *testing.T) {
	s, err := openSQLite(fileDSN(filepath.Join(t.TempDir(), "wynd.db")), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()
	all, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.version >= 33 {
			break
		}
		if err := applyMigration(s.db, m); err != nil {
			t.Fatalf("migration %d: %v", m.version, err)
		}
	}
	const ts = "2026-08-30T09:00:00.000000000Z"
	if _, err := s.db.Exec(`
		INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '` + ts + `');
		INSERT INTO circles (id, name, owner_account_id, created_at, updated_at) VALUES ('c1', 'Семья', 'acc', '` + ts + `', '` + ts + `');
		INSERT INTO identities (id, circle_id, account_id, created_at) VALUES ('i1', 'c1', 'acc', '` + ts + `');
		INSERT INTO events (id, circle_id, event_type, is_service, actor_identity_id, actor_name, payload, summary, created_at)
		VALUES ('e1', 'c1', 'post.created', 0, 'i1', 'Аня', '{}', 'Аня опубликовала запись', '` + ts + `');
		INSERT INTO days (circle_id, entry_date) VALUES ('c1', '2026-08-30');
		INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at)
		VALUES ('p1', 'c1', 1, 'i1', 'Аня', 'были на даче', '2026-08-30', '` + ts + `');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate over populated base: %v", err)
	}
	var text int64
	if err := s.db.QueryRow(`SELECT text_bytes FROM circles WHERE id = 'c1'`).Scan(&text); err != nil {
		t.Fatal(err)
	}
	// Запись 550 + событие 400 + текст: тело трижды, имена и строка журнала.
	if text < 950+3*int64(len("были на даче")) {
		t.Fatalf("text_bytes = %d", text)
	}
}
