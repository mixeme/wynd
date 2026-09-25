-- Stage 6: mail, push, jobs, notify prefs, quota requests.

ALTER TABLE instance_settings ADD COLUMN smtp_host TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_port INTEGER NOT NULL DEFAULT 587;
ALTER TABLE instance_settings ADD COLUMN smtp_username TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_password TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_from TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN smtp_test_sent_at TEXT;

ALTER TABLE instance_settings ADD COLUMN vapid_public_key TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN vapid_private_key TEXT NOT NULL DEFAULT '';

ALTER TABLE instance_settings ADD COLUMN last_routine_at TEXT;
ALTER TABLE instance_settings ADD COLUMN last_backup_at TEXT;

CREATE TABLE account_notify_prefs (
    account_id TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    posts INTEGER NOT NULL DEFAULT 1 CHECK (posts IN (0, 1)),
    comments INTEGER NOT NULL DEFAULT 1 CHECK (comments IN (0, 1)),
    reactions INTEGER NOT NULL DEFAULT 1 CHECK (reactions IN (0, 1))
);

CREATE TABLE circle_notify_prefs (
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    posts INTEGER CHECK (posts IN (0, 1)),
    comments INTEGER CHECK (comments IN (0, 1)),
    reactions INTEGER CHECK (reactions IN (0, 1)),
    PRIMARY KEY (account_id, circle_id)
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

CREATE INDEX idx_push_subscriptions_account ON push_subscriptions (account_id);

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

CREATE INDEX idx_quota_requests_status ON quota_requests (status, created_at);
