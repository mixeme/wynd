package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// Инвариант (TST-4): миграции применяются к **заполненной** базе предыдущей
// версии, а не только к пустой. Все прежние тесты миграций шли по пустой
// схеме, поэтому 0010–0011 никто не проверял на настоящих данных.
//
// База собирается так: открываем и мигрируем чистую базу, затем откатываем
// служебную таблицу к состоянию «применено по 0009», возвращаем колонке
// sessions прежнее имя, кладём данные в прежнем формате времени и мигрируем
// снова.
func TestMigrationsOnPopulatedPreviousVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	ctx := context.Background()

	// База версии 9: применяем только миграции до неё включительно.
	if err := migrateUpTo(s.db, 9); err != nil {
		t.Fatalf("migrate to v9: %v", err)
	}
	seedPopulatedV9(t, s.db)

	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate over populated db: %v", err)
	}

	// 0010: сессии удалены, колонка переименована.
	var sessions int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("сессии не удалены: %d", sessions)
	}
	var hasHash int
	if err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM pragma_table_info('sessions') WHERE name = 'token_hash'
	`).Scan(&hasHash); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if hasHash != 1 {
		t.Fatal("колонка token_hash не появилась")
	}

	// 0011: все метки выровнены по ширине, значения сохранены.
	checks := []struct {
		query string
		want  string
	}{
		{`SELECT created_at FROM posts WHERE id = 'p-zero'`, "2026-08-30T10:00:00.000000000Z"},
		{`SELECT created_at FROM posts WHERE id = 'p-two'`, "2026-08-30T10:00:00.250000000Z"},
		{`SELECT created_at FROM posts WHERE id = 'p-seven'`, "2026-08-30T10:00:00.250100000Z"},
		{`SELECT created_at FROM posts WHERE id = 'p-nine'`, "2026-08-30T10:00:00.123456789Z"},
		{`SELECT started_at FROM membership_spans WHERE membership_id = 'm1'`, "2026-08-30T09:00:00.000000000Z"},
		{`SELECT created_at FROM events WHERE id = 'e1'`, "2026-08-30T10:00:00.500000000Z"},
	}
	for _, c := range checks {
		var got string
		if err := s.db.QueryRowContext(ctx, c.query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.query, err)
		}
		if got != c.want {
			t.Fatalf("%s: got %q want %q", c.query, got, c.want)
		}
	}

	// NULL остаётся NULL, а не превращается в строку.
	var ended sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT ended_at FROM membership_spans WHERE membership_id = 'm1'`).Scan(&ended); err != nil {
		t.Fatalf("ended_at: %v", err)
	}
	if ended.Valid {
		t.Fatalf("ended_at перестал быть NULL: %q", ended.String)
	}

	// Порядок строк теперь совпадает с порядком времени.
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM posts ORDER BY created_at`)
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	defer rows.Close()
	var order []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		order = append(order, id)
	}
	want := []string{"p-zero", "p-nine", "p-two", "p-seven"}
	if len(order) != len(want) {
		t.Fatalf("порядок: %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("порядок: got %v want %v", order, want)
		}
	}
	// 0014: из двух pending-заявок одной учётки остаётся старшая, младшая
	// отклонена; индекс не даёт завести вторую.
	var pendingLeft, rejected string
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM pay_requests WHERE status = 'pending' AND account_id = 'acc'`).Scan(&pendingLeft); err != nil {
		t.Fatalf("pending after 0014: %v", err)
	}
	if pendingLeft != "pr-old" {
		t.Fatalf("осталась не старшая заявка: %s", pendingLeft)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT status FROM pay_requests WHERE id = 'pr-new'`).Scan(&rejected); err != nil || rejected != "rejected" {
		t.Fatalf("младшая заявка: status=%q err=%v", rejected, err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO pay_requests (id, account_id, blob_id, status, created_at)
		VALUES ('pr-dup', 'acc', 'b1', 'pending', '2026-08-30T12:00:00.000000000Z')
	`); err == nil {
		t.Fatal("вторая pending-заявка прошла мимо уникального индекса")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// seedPopulatedV9 кладёт данные в прежнем формате времени: с нулём, двумя,
// семью и девятью знаками дроби, а также NULL.
// migrateUpTo применяет миграции по версию maxVersion включительно.
func migrateUpTo(db *sql.DB, maxVersion int) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return err
	}
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version > maxVersion {
			break
		}
		if err := applyMigration(db, m); err != nil {
			return err
		}
	}
	return nil
}

func seedPopulatedV9(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '2026-08-30T09:00:00Z')`,
		`INSERT INTO sessions (token, account_id, kind, expires_at, created_at)
		 VALUES ('raw-token', 'acc', 'participant', '2026-09-30T09:00:00Z', '2026-08-30T09:00:00Z')`,
		`INSERT INTO circles (id, name, owner_account_id, created_at, updated_at)
		 VALUES ('c1', 'Семья', 'acc', '2026-08-30T09:00:00Z', '2026-08-30T09:00:00Z')`,
		`INSERT INTO identities (id, circle_id, account_id, created_at)
		 VALUES ('i1', 'c1', 'acc', '2026-08-30T09:00:00Z')`,
		`INSERT INTO memberships (id, circle_id, account_id, identity_id, status, created_at, updated_at)
		 VALUES ('m1', 'c1', 'acc', 'i1', 'active', '2026-08-30T09:00:00Z', '2026-08-30T09:00:00Z')`,
		`INSERT INTO membership_spans (id, membership_id, started_at, ended_at, can_read, can_write)
		 VALUES ('s1', 'm1', '2026-08-30T09:00:00Z', NULL, 1, 1)`,
		`INSERT INTO events (seq, id, circle_id, event_type, is_service, actor_identity_id, actor_name, summary, payload, created_at)
		 VALUES (1, 'e1', 'c1', 'post.created', 0, 'i1', 'Аня', 'сводка', '{}', '2026-08-30T10:00:00.5Z')`,
		`INSERT INTO events (seq, id, circle_id, event_type, is_service, actor_identity_id, actor_name, summary, payload, created_at)
		 VALUES (2, 'e2', 'c1', 'post.created', 0, 'i1', 'Аня', 'сводка', '{}', '2026-08-30T10:00:01Z')`,
		`INSERT INTO events (seq, id, circle_id, event_type, is_service, actor_identity_id, actor_name, summary, payload, created_at)
		 VALUES (3, 'e3', 'c1', 'post.created', 0, 'i1', 'Аня', 'сводка', '{}', '2026-08-30T10:00:02Z')`,
		`INSERT INTO events (seq, id, circle_id, event_type, is_service, actor_identity_id, actor_name, summary, payload, created_at)
		 VALUES (4, 'e4', 'c1', 'post.created', 0, 'i1', 'Аня', 'сводка', '{}', '2026-08-30T10:00:03Z')`,
		`INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at)
		 VALUES ('p-zero', 'c1', 1, 'i1', 'Аня', 'ноль', '2026-08-30', '2026-08-30T10:00:00Z')`,
		`INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at)
		 VALUES ('p-two', 'c1', 2, 'i1', 'Аня', 'два', '2026-08-30', '2026-08-30T10:00:00.25Z')`,
		`INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at)
		 VALUES ('p-seven', 'c1', 3, 'i1', 'Аня', 'семь', '2026-08-30', '2026-08-30T10:00:00.2501Z')`,
		`INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at)
		 VALUES ('p-nine', 'c1', 4, 'i1', 'Аня', 'девять', '2026-08-30', '2026-08-30T10:00:00.123456789Z')`,
		`INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
		 VALUES ('b1', 'acc', 'deadbeef', 1, 'image/jpeg', 'b/1', 'complete', '2026-08-30T09:00:00Z')`,
		`INSERT INTO pay_requests (id, account_id, blob_id, status, created_at)
		 VALUES ('pr-old', 'acc', 'b1', 'pending', '2026-08-30T10:00:00Z')`,
		`INSERT INTO pay_requests (id, account_id, blob_id, status, created_at)
		 VALUES ('pr-new', 'acc', 'b1', 'pending', '2026-08-30T11:00:00Z')`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %.60s: %v", q, err)
		}
	}
}

// Инвариант (MIG-1): пропущенная миграция ниже текущей версии схемы —
// отказ запуска, а не применение поверх более новой схемы.
func TestMigrateRefusesSkippedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wynd.db")
	s, err := openSQLite(fileDSN(path), true)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Дыра в середине: отметка о 0002 потеряна, схема осталась новой.
	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = 2`); err != nil {
		t.Fatalf("punch hole: %v", err)
	}
	err = migrate(s.db)
	if err == nil {
		t.Fatal("миграция поверх дыры должна отказывать")
	}
	if !strings.Contains(err.Error(), "пропущена") {
		t.Fatalf("err = %v", err)
	}
}
