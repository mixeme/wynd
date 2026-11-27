-- Имя автора — часть поиска (замечание владельца 27.11): раньше author_name
-- лежал в FTS столбцом UNINDEXED, и набранное имя ничего не находило.
-- Индекс пересобирается с индексируемым author_name; заодно в него попадают
-- записи и комментарии без текста (одни фото): по словам они не находятся,
-- а по имени автора — да, как и через фильтр `author`.
DROP TABLE IF EXISTS content_fts;

CREATE VIRTUAL TABLE content_fts USING fts5(
    body,
    post_id UNINDEXED,
    circle_id UNINDEXED,
    comment_id UNINDEXED,
    author_name,
    kind UNINDEXED,
    created_at UNINDEXED,
    entry_date UNINDEXED,
    tokenize = 'unicode61'
);

-- Записи: тело может быть пустым (только фото) — строка нужна ради автора.
INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT COALESCE(p.body, ''), p.id, p.circle_id, '', p.author_name, 'post', p.created_at, p.entry_date
FROM posts p
WHERE p.deleted = 0;

INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT COALESCE(c.body, ''), c.post_id, c.circle_id, c.id, c.author_name, 'comment', c.created_at, p.entry_date
FROM comments c
JOIN posts p ON p.id = c.post_id
WHERE c.deleted = 0;

INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT trim(COALESCE(pm.audio_artist, '') || ' ' || COALESCE(pm.audio_title, '') || ' ' || COALESCE(b.original_filename, '')),
       p.id, p.circle_id, pm.id, p.author_name,
       CASE WHEN b.mime_type LIKE 'audio/%' THEN 'audio' ELSE 'file' END,
       p.created_at, p.entry_date
FROM post_media pm
JOIN posts p ON p.id = pm.post_id
JOIN blobs b ON b.id = pm.blob_id
WHERE pm.kind = 'attachment' AND p.deleted = 0
  AND trim(COALESCE(pm.audio_title, '') || COALESCE(pm.audio_artist, '') || COALESCE(b.original_filename, '')) != '';

INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT trim(COALESCE(b.original_filename, '')), c.post_id, c.circle_id, cm.id, c.author_name,
       CASE WHEN b.mime_type LIKE 'audio/%' THEN 'caudio' ELSE 'cfile' END,
       c.created_at, p.entry_date
FROM comment_media cm
JOIN comments c ON c.id = cm.comment_id
JOIN posts p ON p.id = c.post_id
JOIN blobs b ON b.id = cm.blob_id
WHERE cm.kind = 'attachment' AND cm.voice = 0 AND c.deleted = 0 AND p.deleted = 0
  AND trim(COALESCE(b.original_filename, '')) != '';

-- Название дня: последнее неудалённое на каждый день; пустое («убрали
-- название») в поиск не идёт — как и в триггере fts_day_insert.
INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
FROM day_titles dt
WHERE dt.deleted = 0 AND dt.title IS NOT NULL AND dt.title != ''
  AND dt.event_seq = (
      SELECT MAX(event_seq) FROM day_titles d2
      WHERE d2.circle_id = dt.circle_id AND d2.entry_date = dt.entry_date AND d2.deleted = 0
  );

-- Триггеры записей и комментариев: строка индекса живёт, пока живёт текст
-- или имя автора. Раньше запись без слов не индексировалась вовсе.
DROP TRIGGER IF EXISTS fts_post_insert;
CREATE TRIGGER fts_post_insert AFTER INSERT ON posts WHEN NEW.deleted = 0
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    VALUES (COALESCE(NEW.body, ''), NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at, NEW.entry_date);
END;

DROP TRIGGER IF EXISTS fts_post_update;
CREATE TRIGGER fts_post_update AFTER UPDATE ON posts
BEGIN
    DELETE FROM content_fts WHERE post_id = OLD.id AND kind = 'post';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT COALESCE(NEW.body, ''), NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at, NEW.entry_date
    WHERE NEW.deleted = 0;
END;

DROP TRIGGER IF EXISTS fts_comment_insert;
CREATE TRIGGER fts_comment_insert AFTER INSERT ON comments WHEN NEW.deleted = 0
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT COALESCE(NEW.body, ''), NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at, p.entry_date
    FROM posts p WHERE p.id = NEW.post_id;
END;

DROP TRIGGER IF EXISTS fts_comment_update;
CREATE TRIGGER fts_comment_update AFTER UPDATE ON comments
BEGIN
    DELETE FROM content_fts WHERE comment_id = OLD.id AND kind = 'comment';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT COALESCE(NEW.body, ''), NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at, p.entry_date
    FROM posts p WHERE p.id = NEW.post_id
      AND NEW.deleted = 0;
END;
