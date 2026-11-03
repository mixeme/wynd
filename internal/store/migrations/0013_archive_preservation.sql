-- A19: право сохранения не меняет membership_spans и чтение в приложении.
ALTER TABLE circles ADD COLUMN archive_all INTEGER NOT NULL DEFAULT 1 CHECK (archive_all IN (0, 1));
ALTER TABLE circles ADD COLUMN archive_revision INTEGER NOT NULL DEFAULT 0;
CREATE TABLE archive_spans (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    started_at TEXT NOT NULL,
    ended_at TEXT
);
CREATE INDEX archive_spans_circle ON archive_spans(circle_id);
ALTER TABLE circle_archives ADD COLUMN scope_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE circle_archives ADD COLUMN full_journal INTEGER NOT NULL DEFAULT 1 CHECK (full_journal IN (0, 1));
ALTER TABLE circle_archives ADD COLUMN posts INTEGER NOT NULL DEFAULT 0;
UPDATE circle_archives SET posts = (
    SELECT COUNT(*) FROM posts p WHERE p.circle_id = circle_archives.circle_id
      AND p.deleted = 0 AND p.created_at < COALESCE(circle_archives.snapshot_at, circle_archives.requested_at)
);
