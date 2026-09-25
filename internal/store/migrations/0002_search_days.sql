CREATE TABLE content_fts_backup (
    body TEXT,
    post_id TEXT,
    circle_id TEXT,
    comment_id TEXT,
    author_name TEXT,
    kind TEXT,
    created_at TEXT
);

INSERT INTO content_fts_backup
SELECT body, post_id, circle_id, comment_id, author_name, kind, created_at FROM content_fts;

DROP TRIGGER IF EXISTS fts_comment_delete;
DROP TRIGGER IF EXISTS fts_comment_insert;
DROP TRIGGER IF EXISTS fts_comment_update;
DROP TRIGGER IF EXISTS fts_post_delete;
DROP TRIGGER IF EXISTS fts_post_insert;
DROP TRIGGER IF EXISTS fts_post_update;

DROP TABLE content_fts;

CREATE VIRTUAL TABLE content_fts USING fts5(
    body,
    post_id UNINDEXED,
    circle_id UNINDEXED,
    comment_id UNINDEXED,
    author_name UNINDEXED,
    kind UNINDEXED,
    created_at UNINDEXED,
    entry_date UNINDEXED,
    tokenize = 'unicode61'
);

INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT b.body, b.post_id, b.circle_id, b.comment_id, b.author_name, b.kind, b.created_at,
    (SELECT p.entry_date FROM posts p WHERE p.id = b.post_id)
FROM content_fts_backup b;

DROP TABLE content_fts_backup;

CREATE TRIGGER fts_comment_delete AFTER DELETE ON comments
BEGIN
    DELETE FROM content_fts WHERE comment_id = OLD.id;
END;

CREATE TRIGGER fts_comment_insert AFTER INSERT ON comments WHEN NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != ''
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT NEW.body, NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at, p.entry_date
    FROM posts p WHERE p.id = NEW.post_id;
END;

CREATE TRIGGER fts_comment_update AFTER UPDATE ON comments
BEGIN
    DELETE FROM content_fts WHERE comment_id = OLD.id AND kind = 'comment';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT NEW.body, NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at, p.entry_date
    FROM posts p WHERE p.id = NEW.post_id
      AND NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != '';
END;

CREATE TRIGGER fts_post_delete AFTER DELETE ON posts
BEGIN
    DELETE FROM content_fts WHERE post_id = OLD.id;
END;

CREATE TRIGGER fts_post_insert AFTER INSERT ON posts WHEN NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != ''
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    VALUES (NEW.body, NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at, NEW.entry_date);
END;

CREATE TRIGGER fts_post_update AFTER UPDATE ON posts
BEGIN
    DELETE FROM content_fts WHERE post_id = OLD.id AND kind = 'post';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT NEW.body, NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at, NEW.entry_date
    WHERE NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != '';
END;

DROP TRIGGER IF EXISTS fts_day_delete;
DROP TRIGGER IF EXISTS fts_day_insert;
DROP TRIGGER IF EXISTS fts_day_update;

CREATE TRIGGER fts_day_delete AFTER DELETE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = OLD.circle_id AND entry_date = OLD.entry_date;
END;

CREATE TRIGGER fts_day_insert AFTER INSERT ON day_titles WHEN NEW.deleted = 0 AND NEW.title IS NOT NULL AND NEW.title != ''
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    VALUES (NEW.title, '', NEW.circle_id, '', '', 'day', NEW.created_at, NEW.entry_date);
END;

CREATE TRIGGER fts_day_update AFTER UPDATE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = OLD.circle_id AND entry_date = OLD.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT NEW.title, '', NEW.circle_id, '', '', 'day', NEW.created_at, NEW.entry_date
    WHERE NEW.deleted = 0 AND NEW.title IS NOT NULL AND NEW.title != '';
END;

INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
FROM day_titles dt
INNER JOIN (
    SELECT circle_id, entry_date, MAX(event_seq) AS max_seq
    FROM day_titles
    WHERE deleted = 0 AND title IS NOT NULL AND title != ''
    GROUP BY circle_id, entry_date
) latest ON dt.circle_id = latest.circle_id AND dt.entry_date = latest.entry_date AND dt.event_seq = latest.max_seq;
