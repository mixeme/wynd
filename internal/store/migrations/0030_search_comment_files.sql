-- 0030: поиск по именам файлов из комментариев (вложения — 0029).
--
-- Файл комментария попадает в content_fts своей строкой, как вложение записи
-- (0022), но со своим видом: 'cfile', для звука — 'caudio'. Наружу поиск
-- отдаёт их как 'file' и 'audio' вместе с id комментария: находка ведёт в
-- обсуждение, к реплике. Отдельный вид нужен видимости (search.go): носитель —
-- запись, но и сам комментарий должен попадать в отрезок чтения, поэтому
-- created_at строки — время комментария, а автор — автор комментария.
-- comment_id строки — id строки comment_media. Фото и голосовые не
-- индексируются: имя у них служебное.
-- Обновлений у comment_media нет: вложение уходит только вместе с репликой.

CREATE TRIGGER fts_comment_media_insert AFTER INSERT ON comment_media
WHEN NEW.kind = 'attachment' AND NEW.voice = 0
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT trim(b.original_filename), c.post_id, c.circle_id, NEW.id, c.author_name,
        CASE WHEN b.mime_type LIKE 'audio/%' THEN 'caudio' ELSE 'cfile' END,
        c.created_at, p.entry_date
    FROM comments c
    JOIN posts p ON p.id = c.post_id
    JOIN blobs b ON b.id = NEW.blob_id
    WHERE c.id = NEW.comment_id
      AND trim(COALESCE(b.original_filename, '')) != '';
END;

CREATE TRIGGER fts_comment_media_delete AFTER DELETE ON comment_media
BEGIN
    DELETE FROM content_fts WHERE kind IN ('cfile', 'caudio') AND comment_id = OLD.id;
END;

-- Уже отправленные вложения комментариев.
INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT trim(b.original_filename), c.post_id, c.circle_id, cm.id, c.author_name,
    CASE WHEN b.mime_type LIKE 'audio/%' THEN 'caudio' ELSE 'cfile' END,
    c.created_at, p.entry_date
FROM comment_media cm
JOIN comments c ON c.id = cm.comment_id AND c.deleted = 0
JOIN posts p ON p.id = c.post_id
JOIN blobs b ON b.id = cm.blob_id
WHERE cm.kind = 'attachment' AND cm.voice = 0
  AND trim(COALESCE(b.original_filename, '')) != '';

-- Запись сменила день — строки файлов её комментариев следом: фильтр поиска
-- по дате читает entry_date строки индекса. Автор остаётся автором комментария.
CREATE TRIGGER fts_comment_media_post_update AFTER UPDATE OF entry_date ON posts
BEGIN
    UPDATE content_fts SET entry_date = NEW.entry_date
    WHERE post_id = NEW.id AND kind IN ('cfile', 'caudio');
END;
