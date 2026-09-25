CREATE TABLE account_notify_prefs (
    account_id TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    posts INTEGER NOT NULL DEFAULT 1 CHECK (posts IN (0, 1)),
    comments INTEGER NOT NULL DEFAULT 1 CHECK (comments IN (0, 1)),
    reactions INTEGER NOT NULL DEFAULT 0 CHECK (reactions IN (0, 1))
);

CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL COLLATE NOCASE UNIQUE,
    created_at TEXT NOT NULL
, blocked INTEGER NOT NULL DEFAULT 0, last_login_at TEXT NULL, deleted_at TEXT NULL);

CREATE TABLE admin_credentials (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    password_hash TEXT NOT NULL,
    reset_token TEXT,
    reset_expires_at TEXT,
    updated_at TEXT NOT NULL
);

CREATE TABLE blob_refs (
    blob_id TEXT NOT NULL REFERENCES blobs(id) ON DELETE CASCADE,
    ref_type TEXT NOT NULL,
    ref_id TEXT NOT NULL,
    PRIMARY KEY (blob_id, ref_type, ref_id)
);

CREATE TABLE blobs (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    sha256 TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    mime_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'complete')),
    created_at TEXT NOT NULL
, original_filename TEXT);

CREATE TABLE circle_notify_prefs (
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    posts INTEGER CHECK (posts IN (0, 1)),
    comments INTEGER CHECK (comments IN (0, 1)),
    reactions INTEGER CHECK (reactions IN (0, 1)),
    PRIMARY KEY (account_id, circle_id)
);

CREATE TABLE circles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    owner_account_id TEXT NOT NULL,
    edit_window_sec INTEGER, -- NULL = unlimited, 0 = chronicle
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
, quota_bytes INTEGER, archive_cutoff_date TEXT, archive_deadline TEXT, archive_reminder_before_sec INTEGER, cutoff_locked_at TEXT, archive_reminder_sent_at TEXT, archive_cycle_started_at TEXT, color TEXT NOT NULL DEFAULT 'ochre', invite_who TEXT NOT NULL DEFAULT 'all'
    CHECK (invite_who IN ('all', 'owner')), invite_kind_default TEXT NOT NULL DEFAULT 'single'
    CHECK (invite_kind_default IN ('single', 'multi')), quota_custom INTEGER NOT NULL DEFAULT 0);

CREATE TABLE code_request_log (
    client_ip TEXT NOT NULL,
    email TEXT NOT NULL COLLATE NOCASE,
    requested_at TEXT NOT NULL
);

CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    event_seq INTEGER NOT NULL UNIQUE REFERENCES events(seq),
    identity_id TEXT NOT NULL REFERENCES identities(id),
    author_name TEXT NOT NULL,
    body TEXT,
    created_at TEXT NOT NULL,
    edit_window_sec INTEGER,
    editable_until TEXT,
    deleted INTEGER NOT NULL DEFAULT 0
);

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

CREATE TABLE day_covers (
    circle_id TEXT NOT NULL,
    entry_date TEXT NOT NULL,
    event_seq INTEGER NOT NULL UNIQUE REFERENCES events(seq),
    identity_id TEXT NOT NULL REFERENCES identities(id),
    post_id TEXT NOT NULL REFERENCES posts(id),
    blob_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    edit_window_sec INTEGER,
    editable_until TEXT,
    deleted INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (circle_id, entry_date, event_seq),
    FOREIGN KEY (circle_id, entry_date) REFERENCES days (circle_id, entry_date) ON DELETE CASCADE
);

CREATE TABLE day_titles (
    circle_id TEXT NOT NULL,
    entry_date TEXT NOT NULL,
    event_seq INTEGER NOT NULL UNIQUE REFERENCES events(seq),
    identity_id TEXT NOT NULL REFERENCES identities(id),
    title TEXT NOT NULL,
    created_at TEXT NOT NULL,
    edit_window_sec INTEGER,
    editable_until TEXT,
    deleted INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (circle_id, entry_date, event_seq),
    FOREIGN KEY (circle_id, entry_date) REFERENCES days (circle_id, entry_date) ON DELETE CASCADE
);

CREATE TABLE days (
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    entry_date TEXT NOT NULL,
    title TEXT,
    title_event_seq INTEGER,
    cover_post_id TEXT,
    cover_blob_id TEXT,
    cover_event_seq INTEGER,
    PRIMARY KEY (circle_id, entry_date)
);

CREATE TABLE events (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    is_service INTEGER NOT NULL,
    actor_identity_id TEXT REFERENCES identities(id),
    actor_name TEXT NOT NULL DEFAULT '',
    target_id TEXT,
    payload TEXT NOT NULL DEFAULT '{}',
    summary TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE "identities" (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    account_id TEXT,
    created_at TEXT NOT NULL,
    UNIQUE (circle_id, account_id)
);

CREATE TABLE identity_names (
    id TEXT PRIMARY KEY,
    identity_id TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    avatar_blob_id TEXT,
    effective_at TEXT NOT NULL,
    erased_at TEXT
);

CREATE TABLE instance_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL DEFAULT '',
    registration_mode TEXT NOT NULL DEFAULT 'invite'
        CHECK (registration_mode IN ('open', 'invite', 'closed')),
    bootstrapped INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
, storage_quota_bytes INTEGER NOT NULL DEFAULT 107374182400, compress_photo_max_px INTEGER NOT NULL DEFAULT 2048, compress_photo_quality INTEGER NOT NULL DEFAULT 80, compress_video_max_height INTEGER NOT NULL DEFAULT 1080, compress_video_bitrate_kbps INTEGER NOT NULL DEFAULT 6000, compress_attachment_max_bytes INTEGER NOT NULL DEFAULT 104857600, smtp_host TEXT NOT NULL DEFAULT '', smtp_port INTEGER NOT NULL DEFAULT 587, smtp_username TEXT NOT NULL DEFAULT '', smtp_password TEXT NOT NULL DEFAULT '', smtp_from TEXT NOT NULL DEFAULT '', smtp_test_sent_at TEXT, vapid_public_key TEXT NOT NULL DEFAULT '', vapid_private_key TEXT NOT NULL DEFAULT '', last_routine_at TEXT, last_backup_at TEXT, default_circle_quota_bytes INTEGER NULL);

INSERT INTO instance_settings (id, name, registration_mode, bootstrapped, updated_at)
VALUES (1, '', 'invite', 0, '1970-01-01T00:00:00Z');

CREATE TABLE invites (
    id TEXT PRIMARY KEY,
    token TEXT NOT NULL UNIQUE,
    circle_id TEXT REFERENCES circles(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('single', 'multi')),
    max_uses INTEGER NOT NULL CHECK (max_uses > 0),
    uses INTEGER NOT NULL DEFAULT 0 CHECK (uses >= 0),
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_by_account_id TEXT REFERENCES accounts(id),
    created_at TEXT NOT NULL
);

CREATE TABLE membership_spans (
    id TEXT PRIMARY KEY,
    membership_id TEXT NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    can_read INTEGER NOT NULL DEFAULT 1,
    can_write INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE memberships (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    identity_id TEXT NOT NULL REFERENCES identities(id),
    can_settings INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('active', 'left_with_access', 'gone')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (circle_id, account_id)
);

CREATE TABLE pending_circle_joins (
  account_id TEXT NOT NULL,
  circle_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (account_id, circle_id),
  FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
  FOREIGN KEY (circle_id) REFERENCES circles(id) ON DELETE CASCADE
);

CREATE TABLE pending_codes (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL COLLATE NOCASE,
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL,
    flow TEXT NOT NULL CHECK (flow IN ('login', 'register', 'invite')),
    invite_id TEXT REFERENCES invites(id),
    invite_name TEXT,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

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

CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    event_seq INTEGER NOT NULL UNIQUE REFERENCES events(seq),
    identity_id TEXT NOT NULL REFERENCES identities(id),
    author_name TEXT NOT NULL,
    body TEXT,
    entry_date TEXT NOT NULL,
    captured_at TEXT,
    created_at TEXT NOT NULL,
    edit_window_sec INTEGER,
    editable_until TEXT,
    deleted INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE push_subscriptions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE quota_requests (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    requested_by_account_id TEXT NOT NULL REFERENCES accounts(id),
    requested_bytes INTEGER NOT NULL CHECK (requested_bytes > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    admin_note TEXT,
    created_at TEXT NOT NULL,
    resolved_at TEXT
);

CREATE TABLE reactions (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    event_seq INTEGER NOT NULL UNIQUE REFERENCES events(seq),
    identity_id TEXT NOT NULL REFERENCES identities(id),
    author_name TEXT NOT NULL,
    emoji TEXT NOT NULL,
    created_at TEXT NOT NULL,
    edit_window_sec INTEGER,
    editable_until TEXT,
    deleted INTEGER NOT NULL DEFAULT 0,
    UNIQUE (post_id, identity_id)
);

CREATE TABLE read_cursors (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    last_read_seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('participant', 'admin')),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE upload_sessions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    expected_size INTEGER NOT NULL CHECK (expected_size > 0),
    mime_type TEXT NOT NULL,
    sha256 TEXT,
    received_bytes INTEGER NOT NULL DEFAULT 0 CHECK (received_bytes >= 0),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
, original_filename TEXT);

CREATE INDEX idx_blob_refs_ref ON blob_refs (ref_type, ref_id);

CREATE INDEX idx_blobs_account ON blobs (account_id);

CREATE INDEX idx_code_request_log_ip_time ON code_request_log (client_ip, requested_at);

CREATE INDEX idx_comments_post ON comments (post_id, created_at);

CREATE INDEX idx_events_circle_created ON events (circle_id, created_at);

CREATE INDEX idx_events_circle_seq ON events (circle_id, seq);

CREATE INDEX idx_invites_token ON invites (token);

CREATE INDEX idx_membership_spans_membership ON membership_spans (membership_id, started_at);

CREATE INDEX idx_pending_codes_email ON pending_codes (email, created_at);

CREATE INDEX idx_post_media_blob ON post_media (blob_id);

CREATE INDEX idx_post_media_post ON post_media (post_id, sort_order);

CREATE INDEX idx_posts_circle_created ON posts (circle_id, created_at);

CREATE INDEX idx_posts_circle_entry_date ON posts (circle_id, entry_date);

CREATE INDEX idx_push_subscriptions_account ON push_subscriptions (account_id);

CREATE INDEX idx_quota_requests_status ON quota_requests (status, created_at);

CREATE INDEX idx_reactions_post ON reactions (post_id, created_at);

CREATE INDEX idx_sessions_account ON sessions (account_id, kind);

CREATE INDEX idx_upload_sessions_account ON upload_sessions (account_id, expires_at);

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