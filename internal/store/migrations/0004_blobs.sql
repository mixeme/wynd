-- Blob storage and journal media (stage 4).

ALTER TABLE instance_settings ADD COLUMN storage_quota_bytes INTEGER NOT NULL DEFAULT 107374182400;
ALTER TABLE instance_settings ADD COLUMN compress_photo_max_px INTEGER NOT NULL DEFAULT 2048;
ALTER TABLE instance_settings ADD COLUMN compress_photo_quality INTEGER NOT NULL DEFAULT 80;
ALTER TABLE instance_settings ADD COLUMN compress_video_max_height INTEGER NOT NULL DEFAULT 1080;
ALTER TABLE instance_settings ADD COLUMN compress_video_bitrate_kbps INTEGER NOT NULL DEFAULT 6000;
ALTER TABLE instance_settings ADD COLUMN compress_attachment_max_bytes INTEGER NOT NULL DEFAULT 104857600;

ALTER TABLE circles ADD COLUMN quota_bytes INTEGER;

CREATE TABLE blobs (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    sha256 TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    mime_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'complete')),
    created_at TEXT NOT NULL
);

CREATE INDEX idx_blobs_account ON blobs (account_id);

CREATE TABLE upload_sessions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    expected_size INTEGER NOT NULL CHECK (expected_size > 0),
    mime_type TEXT NOT NULL,
    sha256 TEXT,
    received_bytes INTEGER NOT NULL DEFAULT 0 CHECK (received_bytes >= 0),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_upload_sessions_account ON upload_sessions (account_id, expires_at);

CREATE TABLE blob_refs (
    blob_id TEXT NOT NULL REFERENCES blobs(id) ON DELETE CASCADE,
    ref_type TEXT NOT NULL,
    ref_id TEXT NOT NULL,
    PRIMARY KEY (blob_id, ref_type, ref_id)
);

CREATE INDEX idx_blob_refs_ref ON blob_refs (ref_type, ref_id);

CREATE TABLE post_media (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id),
    kind TEXT NOT NULL CHECK (kind IN ('photo', 'video', 'attachment')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    captured_at TEXT,
    geo_lat REAL,
    geo_lng REAL,
    is_cover INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_post_media_post ON post_media (post_id, sort_order);
CREATE INDEX idx_post_media_blob ON post_media (blob_id);
