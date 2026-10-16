package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// textQuotaBackfill — пересчёт текста круга с нуля, как в миграции 0033
// прежней нумерации (0.30.0). Триггеры quota_text_* обязаны давать то же.
const textQuotaBackfill = `UPDATE circles SET text_bytes =
    COALESCE((SELECT SUM(550 + 3 * length(CAST(COALESCE(p.body, '') AS BLOB)) + length(CAST(p.author_name AS BLOB)))
        FROM posts p WHERE p.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(560 + 3 * length(CAST(COALESCE(c.body, '') AS BLOB)) + length(CAST(c.author_name AS BLOB)))
        FROM comments c WHERE c.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(450 + length(CAST(r.emoji AS BLOB)) + length(CAST(r.author_name AS BLOB)))
        FROM reactions r WHERE r.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(400 + length(CAST(e.payload AS BLOB)) + length(CAST(e.summary AS BLOB)) + length(CAST(e.actor_name AS BLOB)))
        FROM events e WHERE e.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(250 + 3 * length(CAST(t.title AS BLOB)))
        FROM day_titles t WHERE t.circle_id = circles.id), 0)`

// formerChainBase — база, дошедшая по прежней нумерации до top: схема та же,
// что 0001 (свёртка её не меняла), отметки — 0030…top.
func formerChainBase(t *testing.T, top int) *SQLite {
	t.Helper()
	s, err := openSQLite(fileDSN(filepath.Join(t.TempDir(), "wynd.db")), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM schema_migrations`); err != nil {
		t.Fatal(err)
	}
	for v := legacyBaseline; v <= top; v++ {
		if _, err := s.db.Exec(
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, '2026-10-15T00:00:00.000000000Z')`, v,
		); err != nil {
			t.Fatalf("mark %d: %v", v, err)
		}
	}
	return s
}

// База прежней нумерации, дошедшая до 0036 (рабочий сервер на 0.33.x), при
// запуске получает одну отметку «версия 1»; таблицы и данные целы (C45).
func TestMigrateRenumbersFormerChain(t *testing.T) {
	s := formerChainBase(t, legacyTop)
	if _, err := s.db.Exec(
		`INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '2026-08-30T09:00:00.000000000Z')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate over former chain: %v", err)
	}
	var rows, version, accounts int
	if err := s.db.QueryRow(`SELECT count(*), MAX(version) FROM schema_migrations`).Scan(&rows, &version); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || version != 1 {
		t.Fatalf("отметок %d, версия %d", rows, version)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM accounts`).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if accounts != 1 {
		t.Fatalf("данные не пережили запуск: accounts = %d", accounts)
	}
	// Второй запуск — уже новая нумерация, ничего не меняется.
	if err := migrate(s.db); err != nil {
		t.Fatalf("второй запуск: %v", err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("после второго запуска отметок %d (%v)", rows, err)
	}
}

// База прежней нумерации, не дошедшая до 0036, не доводится: шагов 0031–0036
// в бинарнике нет. Отказ называет выпуск, который их несёт.
func TestMigrateRefusesFormerChainBeforeTop(t *testing.T) {
	s := formerChainBase(t, 33)
	err := migrate(s.db)
	if err == nil {
		t.Fatal("база версии 33 прежней нумерации должна получать отказ")
	}
	if !strings.Contains(err.Error(), legacyLastRelease) {
		t.Fatalf("err = %v", err)
	}
	var rows int
	if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&rows); err != nil || rows != 4 {
		t.Fatalf("отметки тронуты: %d (%v)", rows, err)
	}
}

// База, остановившаяся в цепочке до свёртки 0.26.0, тоже не доводится:
// отказ называет оба выпуска по порядку.
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
	if !strings.Contains(err.Error(), legacyChainRelease) || !strings.Contains(err.Error(), legacyLastRelease) {
		t.Fatalf("err = %v", err)
	}
}

// Инвариант (C42): счёт текста круга, накопленный триггерами quota_text_*,
// равен пересчёту с нуля по той же формуле (её применила к старым базам
// миграция 0033 прежней нумерации) — запись, правка, удаление, реакции,
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
	if _, err := s.db.Exec(textQuotaBackfill); err != nil {
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
