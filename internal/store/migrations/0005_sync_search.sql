-- Sync cursors, FTS5 search index.

CREATE TABLE read_cursors (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    last_read_seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

CREATE VIRTUAL TABLE content_fts USING fts5(
    body,
    post_id UNINDEXED,
    circle_id UNINDEXED,
    comment_id UNINDEXED,
    author_name UNINDEXED,
    kind UNINDEXED,
    created_at UNINDEXED,
    tokenize = 'unicode61'
);

-- Populate FTS from existing rows.
INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at)
SELECT body, id, circle_id, '', author_name, 'post', created_at
FROM posts WHERE deleted = 0 AND body IS NOT NULL AND body != '';

INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at)
SELECT body, post_id, circle_id, id, author_name, 'comment', created_at
FROM comments WHERE deleted = 0 AND body IS NOT NULL AND body != '';

-- Keep FTS in sync with posts.
CREATE TRIGGER fts_post_insert AFTER INSERT ON posts WHEN NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != ''
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at)
    VALUES (NEW.body, NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at);
END;

CREATE TRIGGER fts_post_update AFTER UPDATE ON posts
BEGIN
    DELETE FROM content_fts WHERE post_id = OLD.id AND kind = 'post';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at)
    SELECT NEW.body, NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at
    WHERE NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != '';
END;

CREATE TRIGGER fts_post_delete AFTER DELETE ON posts
BEGIN
    DELETE FROM content_fts WHERE post_id = OLD.id;
END;

-- Keep FTS in sync with comments.
CREATE TRIGGER fts_comment_insert AFTER INSERT ON comments WHEN NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != ''
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at)
    VALUES (NEW.body, NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at);
END;

CREATE TRIGGER fts_comment_update AFTER UPDATE ON comments
BEGIN
    DELETE FROM content_fts WHERE comment_id = OLD.id AND kind = 'comment';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at)
    SELECT NEW.body, NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at
    WHERE NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != '';
END;

CREATE TRIGGER fts_comment_delete AFTER DELETE ON comments
BEGIN
    DELETE FROM content_fts WHERE comment_id = OLD.id;
END;
