package store

import (
	"embed"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// textQuotaBackfill — пересчёт текста круга с нуля, как в миграции 0033
// разметки 1 (0.30.0). Триггеры quota_text_* обязаны давать то же.
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

//go:embed migrations/legacy/*.sql
var legacyMigrationFS embed.FS

// legacyMigrations — цепочка прежней нумерации 0001–0017 (0.34.0–0.53.4),
// хранится рядом с базовой схемой и поднимается только тестами: свежая
// установка её не видит, а перевод базы прежней нумерации проверяется от неё.
func legacyMigrations(t *testing.T) []migration {
	t.Helper()
	entries, err := fs.Glob(legacyMigrationFS, "migrations/legacy/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]migration, 0, len(entries))
	for _, name := range entries {
		version, err := parseMigrationVersion(path.Base(name))
		if err != nil {
			t.Fatal(err)
		}
		body, err := fs.ReadFile(legacyMigrationFS, name)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, migration{version: version, name: path.Base(name), sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out
}

// freshBase — база базовой схемы: свежая установка одним шагом (C115).
func freshBase(t *testing.T) *SQLite {
	t.Helper()
	s, err := openSQLite(fileDSN(filepath.Join(t.TempDir(), "wynd.db")), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

// legacyBaseAt — база, прошедшая прежнюю цепочку по version включительно
// (как у выпуска, где последней была она). migrate() над ней отказывает:
// шагов прежней нумерации в бинарнике больше нет, доведение — Wynd 0.53.4.
func legacyBaseAt(t *testing.T, version int) *SQLite {
	t.Helper()
	s, _ := legacyBaseAtFile(t, version)
	return s
}

// legacyBaseAtFile — то же с явным путём файла: тесты, которые закрывают базу
// и открывают её заново через Open (полный путь запуска мигратора).
func legacyBaseAtFile(t *testing.T, version int) (*SQLite, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(file), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, m := range legacyMigrations(t) {
		if m.version > version {
			break
		}
		if err := applyMigration(s.db, m); err != nil {
			t.Fatalf("migration %d: %v", m.version, err)
		}
	}
	return s, file
}

// completeLegacy доводит базу прежней цепочки до 17 вручную — теми шагами,
// которые в прежних выпусках применял migrate().
func completeLegacy(t *testing.T, s *SQLite) {
	t.Helper()
	for _, m := range legacyMigrations(t) {
		var applied int
		if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&applied); err != nil {
			t.Fatal(err)
		}
		if applied == 1 {
			continue
		}
		if err := applyMigration(s.db, m); err != nil {
			t.Fatalf("migration %d: %v", m.version, err)
		}
	}
}

// completeLegacyOnFile — то же для закрытой базы: открыть, довести до 17,
// закрыть. Дальше тест открывает её через Open — полный путь запуска.
func completeLegacyOnFile(t *testing.T, file string) {
	t.Helper()
	s, err := openSQLite(fileDSN(file), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	completeLegacy(t, s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// formerChainBase — база разметки 1, дошедшая до top: схема та же, что 0001
// прежней нумерации, отметки — 0030…top.
func formerChainBase(t *testing.T, top int) *SQLite {
	t.Helper()
	s := legacyBaseAt(t, 1)
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

// База прежней нумерации, дошедшая до 17 (рабочий сервер на 0.53.4), при
// запуске переводится отметкой на «версию 1» новой нумерации: маяк и одна
// отметка, таблицы и данные целы (C115).
func TestMigrateRenumbersPrevNumbering(t *testing.T) {
	s := legacyBaseAt(t, squashedTop)
	if _, err := s.db.Exec(
		`INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '2026-08-30T09:00:00.000000000Z')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate over prev numbering: %v", err)
	}
	var rows, version, accounts int
	if err := s.db.QueryRow(`SELECT count(*), MAX(version) FROM schema_migrations`).Scan(&rows, &version); err != nil {
		t.Fatal(err)
	}
	if rows != 2 || version != 1 {
		t.Fatalf("после перевода отметок %d, версия %d — ждём маяк 0 и версию 1", rows, version)
	}
	var mark int
	if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = ?`, baselineMark).Scan(&mark); err != nil {
		t.Fatal(err)
	}
	if mark != 1 {
		t.Fatalf("маяк новой нумерации не поставлен")
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM accounts`).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if accounts != 1 {
		t.Fatalf("данные не пережили перевод: accounts = %d", accounts)
	}
	// Второй запуск — уже новая нумерация, ничего не меняется.
	if err := migrate(s.db); err != nil {
		t.Fatalf("второй запуск: %v", err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("после второго запуска отметок %d (%v)", rows, err)
	}
}

// База прежней нумерации, не дошедшая до 17, не доводится: шагов 2–17 в
// бинарнике нет. Отказ называет выпуск, который доведёт её до точки свёртки.
func TestMigrateRefusesDatabaseBelowSquash(t *testing.T) {
	s := legacyBaseAt(t, 16)
	err := migrate(s.db)
	if err == nil {
		t.Fatal("база версии 16 прежней нумерации должна получать отказ")
	}
	if !strings.Contains(err.Error(), squashPrevRelease) {
		t.Fatalf("err = %v", err)
	}
	var rows int
	if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&rows); err != nil || rows != 16 {
		t.Fatalf("отметки тронуты: %d (%v)", rows, err)
	}
}

// База разметки 1, дошедшая до 0036 (рабочий сервер на 0.33.x), не доводится
// на месте: шагов прежней нумерации в бинарнике нет, её доводит выпуск 0.53.4.
func TestMigrateRefusesFormerNumberingOneAtTop(t *testing.T) {
	s := formerChainBase(t, legacyTop)
	err := migrate(s.db)
	if err == nil {
		t.Fatal("база разметки 1 версии 0036 должна получать отказ")
	}
	if !strings.Contains(err.Error(), squashPrevRelease) || !strings.Contains(err.Error(), "разметке 1") {
		t.Fatalf("err = %v", err)
	}
}

// База разметки 1, не дошедшая до 0036, тоже не доводится — отказ ведёт через
// выпуск, который доведёт её до точки свёртки.
func TestMigrateRefusesFormerChainBeforeTop(t *testing.T) {
	s := formerChainBase(t, 33)
	err := migrate(s.db)
	if err == nil {
		t.Fatal("база разметки 1 версии 33 должна получать отказ")
	}
	if !strings.Contains(err.Error(), squashPrevRelease) {
		t.Fatalf("err = %v", err)
	}
	var rows int
	if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&rows); err != nil || rows != 4 {
		t.Fatalf("отметки тронуты: %d (%v)", rows, err)
	}
}

// База, остановившаяся в цепочке до свёртки 0.26.0, тоже не доводится —
// отказ ведёт через выпуск 0.53.4, который знает её прежний механизм доведения.
func TestMigrateRefusesDatabaseBelowLegacyBaseline(t *testing.T) {
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
		t.Fatal("база версии 16 без маяка должна получать отказ")
	}
	if !strings.Contains(err.Error(), squashPrevRelease) {
		t.Fatalf("err = %v", err)
	}
}

// Маяк отличает базовую схему от базы прежней нумерации, остановившейся на
// версии 1: без маяка та получает отказ, а не читается как свёрнутая (C115).
func TestMigrateRefusesUnmarkedBase(t *testing.T) {
	s := freshBase(t)
	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = ?`, baselineMark); err != nil {
		t.Fatal(err)
	}
	err := migrate(s.db)
	if err == nil {
		t.Fatal("база версии 1 без маяка должна получать отказ")
	}
	if !strings.Contains(err.Error(), squashPrevRelease) {
		t.Fatalf("err = %v", err)
	}
}

// Инвариант (TST-4): триггеры раскладки улочки работают на базовой схеме.
// Люди, круги и участие целы; выход с правом читать раскладку не трогает,
// уход совсем убирает круг из групп человека, и опустевшая группа уходит.
func TestMigrateStreetGroupsOverPopulatedBase(t *testing.T) {
	s := freshBase(t)
	if _, err := s.db.Exec(`
		INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '2026-08-30T09:00:00.000000000Z');
		INSERT INTO circles (id, name, owner_account_id, created_at, updated_at) VALUES ('c1', 'Семья', 'acc', '2026-08-30T09:00:00.000000000Z', '2026-08-30T09:00:00.000000000Z');
		INSERT INTO identities (id, circle_id, created_at) VALUES ('i1', 'c1', '2026-08-30T09:00:00.000000000Z');
		INSERT INTO memberships (id, circle_id, account_id, identity_id, status, created_at, updated_at)
		VALUES ('m1', 'c1', 'acc', 'i1', 'active', '2026-08-30T09:00:00.000000000Z', '2026-08-30T09:00:00.000000000Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var accounts, members int
	if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM accounts), (SELECT count(*) FROM memberships WHERE status = 'active')`).Scan(&accounts, &members); err != nil {
		t.Fatal(err)
	}
	if accounts != 1 || members != 1 {
		t.Fatalf("accounts = %d, active memberships = %d", accounts, members)
	}
	if _, err := s.db.Exec(`
		INSERT INTO circle_groups (id, account_id, name, created_at) VALUES ('g1', 'acc', 'Родные', '2026-10-16T00:00:00Z');
		INSERT INTO circle_group_items (group_id, account_id, circle_id, added_at) VALUES ('g1', 'acc', 'c1', '2026-10-16T00:00:00Z');
		INSERT INTO circle_pins (account_id, circle_id, pinned_at) VALUES ('acc', 'c1', '2026-10-16T00:00:00Z');
		UPDATE memberships SET status = 'left_with_access' WHERE id = 'm1';
	`); err != nil {
		t.Fatalf("раскладка: %v", err)
	}
	var groups, pins int
	count := func() {
		t.Helper()
		if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM circle_groups), (SELECT count(*) FROM circle_pins)`).Scan(&groups, &pins); err != nil {
			t.Fatal(err)
		}
	}
	count()
	if groups != 1 || pins != 1 {
		t.Fatalf("после выхода с доступом: групп %d, закреплений %d", groups, pins)
	}
	if _, err := s.db.Exec(`UPDATE memberships SET status = 'gone' WHERE id = 'm1'`); err != nil {
		t.Fatal(err)
	}
	count()
	if groups != 0 || pins != 0 {
		t.Fatalf("после ухода совсем: групп %d, закреплений %d", groups, pins)
	}
}

// Базовая схема несёт почту администратора: столбец есть, писем ещё не было.
func TestMigrateAdminEmailOverPopulatedBase(t *testing.T) {
	s := freshBase(t)
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO instance_settings (id, updated_at) VALUES (1, '2026-10-01T00:00:00Z');
		UPDATE instance_settings SET name = 'Дом Ани', smtp_host = 'smtp.test.local' WHERE id = 1`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var name, host, adminEmail string
	if err := s.db.QueryRow(`SELECT name, smtp_host, admin_email FROM instance_settings WHERE id = 1`).Scan(&name, &host, &adminEmail); err != nil {
		t.Fatal(err)
	}
	if name != "Дом Ани" || host != "smtp.test.local" || adminEmail != "" {
		t.Fatalf("name %q, smtp_host %q, admin_email %q", name, host, adminEmail)
	}
}

// Базовая схема несёт отметку письма о поломках: пустая, пока письма не было.
func TestMigrateAdminTroubleOverPopulatedBase(t *testing.T) {
	s := freshBase(t)
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO instance_settings (id, updated_at) VALUES (1, '2026-10-01T00:00:00Z');
		UPDATE instance_settings SET admin_email = 'admin@example.com' WHERE id = 1`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var adminEmail string
	var sentAt *string
	if err := s.db.QueryRow(`SELECT admin_email, admin_trouble_sent_at FROM instance_settings WHERE id = 1`).Scan(&adminEmail, &sentAt); err != nil {
		t.Fatal(err)
	}
	if adminEmail != "admin@example.com" || sentAt != nil {
		t.Fatalf("admin_email %q, admin_trouble_sent_at %+v", adminEmail, sentAt)
	}
}

// Базовая схема несёт длительность ролика.
func TestMigrateVideoDurationOverPopulatedBase(t *testing.T) {
	s := freshBase(t)
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('post_media') WHERE name = 'video_duration_ms'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("video_duration_ms columns: %d", n)
	}
}

// Базовая схема несёт итог переноса «уже здесь» (C51): у нового переноса 0.
func TestMigrateIncomingAlreadyHereOverPopulatedBase(t *testing.T) {
	s := freshBase(t)
	const ts = "2026-10-20T09:00:00.000000000Z"
	for _, q := range []string{
		`INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '` + ts + `')`,
		`INSERT INTO incoming_moves (id, account_id, link, source_url, circle_name, expires_at, status, created_at, updated_at)
		 VALUES ('m1', 'acc', '', 'https://old.example', 'Семья', '` + ts + `', 'done', '` + ts + `', '` + ts + `')`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	var here int
	if err := s.db.QueryRow(`SELECT already_here FROM incoming_moves WHERE id = 'm1'`).Scan(&here); err != nil {
		t.Fatal(err)
	}
	if here != 0 {
		t.Fatalf("already_here: %d", here)
	}
}

// Инвариант (C42): счёт текста круга, накопленный триггерами quota_text_*,
// равен пересчёту с нуля по той же формуле (её применяла к старым базам
// миграция 0033 разметки 1) — запись, правка, удаление, реакции, события
// и названия дня. Иначе база, пересчитанная при обновлении, и база, дожившая
// до того же состояния записями, расходились бы.
func TestTextQuotaTriggersMatchBackfill(t *testing.T) {
	s := freshBase(t)
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
