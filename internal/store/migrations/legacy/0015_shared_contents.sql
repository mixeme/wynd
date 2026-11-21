-- blobs remain independent logical copies. Only verified bytes enter this registry.
CREATE TABLE blob_contents (
    sha256 TEXT PRIMARY KEY CHECK(length(sha256) = 64),
    storage_path TEXT NOT NULL UNIQUE,
    size_bytes INTEGER NOT NULL CHECK(size_bytes >= 0),
    created_at TEXT NOT NULL
);
ALTER TABLE blobs ADD COLUMN content_sha256 TEXT REFERENCES blob_contents(sha256) ON DELETE RESTRICT;
CREATE INDEX blobs_content ON blobs(content_sha256);
CREATE INDEX blobs_storage_path ON blobs(storage_path);
-- Durable reclamation queue: detach in the referencing transaction, unlink later.
CREATE TABLE blob_file_gc (storage_path TEXT PRIMARY KEY);
-- A live full backup delays physical GC while its database/files are copied.
CREATE TABLE blob_backup_leases (id TEXT PRIMARY KEY, expires_at INTEGER NOT NULL);
