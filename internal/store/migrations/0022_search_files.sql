-- 0022: поиск по именам файлов и названиям звуков (план 46, C11).
--
-- Вложение записи (post_media.kind = 'attachment') попадает в content_fts
-- своей строкой: kind 'audio' для звука (mime audio/*), 'file' для остального.
-- Текст — имя файла, у звука перед ним исполнитель и название из тегов
-- (0019) — в том же порядке, что в строке звука: «исполнитель — название».
-- Видимость — как у записи-носителя (visibleCarrierSQL в search.go): строка
-- привязана к post_id, удалённая или невидимая запись прячет и её.
-- comment_id у такой строки — id строки post_media; сервер отдаёт его как
-- media_id, не как комментарий.

CREATE TRIGGER fts_media_insert AFTER INSERT ON post_media WHEN NEW.kind = 'attachment'
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT trim(COALESCE(NEW.audio_artist, '') || ' ' || COALESCE(NEW.audio_title, '') || ' ' || COALESCE(b.original_filename, '')),
        p.id, p.circle_id, NEW.id, p.author_name,
        CASE WHEN b.mime_type LIKE 'audio/%' THEN 'audio' ELSE 'file' END,
        p.created_at, p.entry_date
    FROM posts p
    JOIN blobs b ON b.id = NEW.blob_id
    WHERE p.id = NEW.post_id
      AND trim(COALESCE(NEW.audio_title, '') || COALESCE(NEW.audio_artist, '') || COALESCE(b.original_filename, '')) != '';
END;

CREATE TRIGGER fts_media_update AFTER UPDATE ON post_media
BEGIN
    DELETE FROM content_fts WHERE kind IN ('file', 'audio') AND comment_id = OLD.id;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT trim(COALESCE(NEW.audio_artist, '') || ' ' || COALESCE(NEW.audio_title, '') || ' ' || COALESCE(b.original_filename, '')),
        p.id, p.circle_id, NEW.id, p.author_name,
        CASE WHEN b.mime_type LIKE 'audio/%' THEN 'audio' ELSE 'file' END,
        p.created_at, p.entry_date
    FROM posts p
    JOIN blobs b ON b.id = NEW.blob_id
    WHERE p.id = NEW.post_id AND NEW.kind = 'attachment'
      AND trim(COALESCE(NEW.audio_title, '') || COALESCE(NEW.audio_artist, '') || COALESCE(b.original_filename, '')) != '';
END;

CREATE TRIGGER fts_media_delete AFTER DELETE ON post_media
BEGIN
    DELETE FROM content_fts WHERE kind IN ('file', 'audio') AND comment_id = OLD.id;
END;

-- Уже загруженные вложения.
INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT trim(COALESCE(pm.audio_artist, '') || ' ' || COALESCE(pm.audio_title, '') || ' ' || COALESCE(b.original_filename, '')),
    p.id, p.circle_id, pm.id, p.author_name,
    CASE WHEN b.mime_type LIKE 'audio/%' THEN 'audio' ELSE 'file' END,
    p.created_at, p.entry_date
FROM post_media pm
JOIN posts p ON p.id = pm.post_id
JOIN blobs b ON b.id = pm.blob_id
WHERE pm.kind = 'attachment'
  AND trim(COALESCE(pm.audio_title, '') || COALESCE(pm.audio_artist, '') || COALESCE(b.original_filename, '')) != '';

-- Запись сменила день или подпись автора — строки её вложений следом:
-- фильтр поиска по дате читает entry_date строки индекса.
CREATE TRIGGER fts_media_post_update AFTER UPDATE OF entry_date, author_name ON posts
BEGIN
    UPDATE content_fts SET entry_date = NEW.entry_date, author_name = NEW.author_name
    WHERE post_id = NEW.id AND kind IN ('file', 'audio');
END;
