package store

import (
	"testing"
)

// Миграция 0003 пересобирает индекс поиска: имя автора становится
// индексируемым, а записи и комментарии без текста попадают в индекс ради
// имени. Проверяем и пересборку старых данных, и новые триггеры.
func TestMigrateSearchAuthorRebuild(t *testing.T) {
	s := freshBase(t)
	if _, err := s.db.Exec(`
		INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', '2026-11-20T09:00:00.000000000Z');
		INSERT INTO circles (id, name, owner_account_id, created_at, updated_at)
		VALUES ('c1', 'Семья', 'acc', '2026-11-20T09:00:00.000000000Z', '2026-11-20T09:00:00.000000000Z');
		INSERT INTO identities (id, circle_id, created_at) VALUES ('i1', 'c1', '2026-11-20T09:00:00.000000000Z');

		INSERT INTO events (seq, id, circle_id, event_type, is_service, summary, created_at)
		VALUES
			(1, 'e1', 'c1', 'post.created', 0, '', '2026-11-20T09:00:00.000000000Z'),
			(2, 'e2', 'c1', 'post.created', 0, '', '2026-11-20T09:00:00.000000000Z'),
			(3, 'e3', 'c1', 'comment.created', 0, '', '2026-11-20T09:00:00.000000000Z'),
			(4, 'e4', 'c1', 'day.titled', 0, '', '2026-11-20T09:00:00.000000000Z'),
			(5, 'e5', 'c1', 'post.created', 0, '', '2026-11-20T09:00:00.000000000Z');

		INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at, deleted)
		VALUES
			('p-text', 'c1', 1, 'i1', 'Аня', 'снимки с дачи', '2026-11-20', '2026-11-20T09:00:00.000000000Z', 0),
			('p-photo', 'c1', 2, 'i1', 'Аня', NULL, '2026-11-21', '2026-11-20T09:00:00.000000000Z', 0),
			('p-del', 'c1', 5, 'i1', 'Аня', 'удалённое', '2026-11-22', '2026-11-20T09:00:00.000000000Z', 1);

		INSERT INTO comments (id, circle_id, post_id, event_seq, identity_id, author_name, body, created_at, deleted)
		VALUES ('cm1', 'c1', 'p-text', 3, 'i1', 'Боб', 'спасибо', '2026-11-20T09:00:00.000000000Z', 0);

		INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at, original_filename)
		VALUES ('b-file', 'acc', 'aa', 1, 'application/pdf', 'aa/aa', 'complete', '2026-11-20T09:00:00.000000000Z', 'смета.pdf');

		INSERT INTO post_media (id, post_id, blob_id, kind) VALUES ('pm1', 'p-text', 'b-file', 'attachment');

		INSERT INTO days (circle_id, entry_date) VALUES ('c1', '2026-11-20');
		INSERT INTO day_titles (circle_id, entry_date, event_seq, identity_id, title, created_at)
		VALUES ('c1', '2026-11-20', 4, 'i1', 'Компот', '2026-11-20T09:00:00.000000000Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := migrate(s.db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	count := func(where string, args ...any) int {
		t.Helper()
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM content_fts WHERE `+where, args...).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", where, err)
		}
		return n
	}

	// Обе записи Ани в индексе: с текстом и из одних фото; удалённая — нет.
	// Третья строка с её именем — файл записи: он наследует автора записи.
	if n := count(`kind = 'post' AND author_name = 'Аня'`); n != 2 {
		t.Fatalf("записей Ани: %d, ждём 2", n)
	}
	if n := count(`kind = 'post' AND body = ''`); n != 1 {
		t.Fatalf("записей без текста: %d, ждём 1", n)
	}
	// Комментарий и файл на месте, название дня — одно.
	if n := count(`kind = 'comment'`); n != 1 {
		t.Fatalf("комментариев: %d, ждём 1", n)
	}
	if n := count(`kind = 'file'`); n != 1 {
		t.Fatalf("файлов: %d, ждём 1", n)
	}
	if n := count(`kind = 'day'`); n != 1 {
		t.Fatalf("дней: %d, ждём 1", n)
	}
	// Имя находится поиском (unicode61 сводит регистр), тело — по-прежнему.
	for _, q := range []string{"аня*", "смета*", "компот*"} {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM content_fts WHERE content_fts MATCH ?`, q).Scan(&n); err != nil {
			t.Fatalf("match %q: %v", q, err)
		}
		if n == 0 {
			t.Fatalf("поиск %q ничего не нашёл", q)
		}
	}

	// Новые триггеры: запись без текста в индексе, пока жива; удалённая уходит.
	if _, err := s.db.Exec(`
		INSERT INTO events (seq, id, circle_id, event_type, is_service, summary, created_at)
		VALUES (6, 'e6', 'c1', 'post.created', 0, '', '2026-11-20T09:00:00.000000000Z');
		INSERT INTO posts (id, circle_id, event_seq, identity_id, author_name, body, entry_date, created_at)
		VALUES ('p-new', 'c1', 6, 'i1', 'Вера', NULL, '2026-11-23', '2026-11-20T09:00:00.000000000Z');
	`); err != nil {
		t.Fatalf("insert post: %v", err)
	}
	if n := count(`post_id = 'p-new' AND kind = 'post'`); n != 1 {
		t.Fatalf("новая запись без текста: %d, ждём 1", n)
	}
	if _, err := s.db.Exec(`UPDATE posts SET body = '' WHERE id = 'p-new'`); err != nil {
		t.Fatalf("update body: %v", err)
	}
	if n := count(`post_id = 'p-new' AND kind = 'post'`); n != 1 {
		t.Fatalf("после правки тела: %d, ждём 1", n)
	}
	if _, err := s.db.Exec(`UPDATE posts SET deleted = 1 WHERE id = 'p-new'`); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := count(`post_id = 'p-new'`); n != 0 {
		t.Fatalf("удалённая запись в индексе: %d, ждём 0", n)
	}
}
