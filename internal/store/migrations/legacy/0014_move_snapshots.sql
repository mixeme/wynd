-- Metadata and a virtual ZIP index, never another full copy of media.
CREATE TABLE move_snapshots (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    snapshot_at TEXT NOT NULL,
    schema_version INTEGER NOT NULL,
    wynd_version TEXT NOT NULL,
    card BLOB NOT NULL,
    data BLOB,
    layout BLOB,
    archive_size INTEGER NOT NULL DEFAULT 0,
    posts INTEGER NOT NULL,
    names INTEGER NOT NULL,
    media_bytes INTEGER NOT NULL,
    CHECK (data IS NOT NULL OR layout IS NOT NULL)
);
CREATE TABLE move_snapshot_blobs (
    snapshot_id TEXT NOT NULL REFERENCES move_snapshots(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id) ON DELETE RESTRICT,
    PRIMARY KEY (snapshot_id, blob_id)
);
CREATE INDEX move_snapshot_blobs_blob ON move_snapshot_blobs(blob_id);
CREATE TABLE move_snapshot_literals (
    snapshot_id TEXT NOT NULL REFERENCES move_snapshots(id) ON DELETE CASCADE,
    part INTEGER NOT NULL,
    data BLOB NOT NULL,
    PRIMARY KEY (snapshot_id, part)
);
ALTER TABLE circle_moves ADD COLUMN snapshot_id TEXT REFERENCES move_snapshots(id) ON DELETE SET NULL;
-- Already prepared transfers keep their original bytes until completion.
ALTER TABLE circle_moves ADD COLUMN legacy_archive_id TEXT;
UPDATE circle_moves SET legacy_archive_id = id WHERE archive_status = 'ready';
ALTER TABLE incoming_moves ADD COLUMN archive_etag TEXT;
