-- Auth domain tables (stage 3).

CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL COLLATE NOCASE UNIQUE,
    created_at TEXT NOT NULL
);

CREATE TABLE instance_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL DEFAULT '',
    registration_mode TEXT NOT NULL DEFAULT 'invite'
        CHECK (registration_mode IN ('open', 'invite', 'closed')),
    bootstrapped INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

INSERT INTO instance_settings (id, name, registration_mode, bootstrapped, updated_at)
VALUES (1, '', 'invite', 0, '1970-01-01T00:00:00Z');

CREATE TABLE admin_credentials (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    password_hash TEXT NOT NULL,
    reset_token TEXT,
    reset_expires_at TEXT,
    updated_at TEXT NOT NULL
);

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

CREATE INDEX idx_invites_token ON invites (token);

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

CREATE INDEX idx_pending_codes_email ON pending_codes (email, created_at);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('participant', 'admin')),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_account ON sessions (account_id, kind);

CREATE TABLE code_request_log (
    client_ip TEXT NOT NULL,
    email TEXT NOT NULL COLLATE NOCASE,
    requested_at TEXT NOT NULL
);

CREATE INDEX idx_code_request_log_ip_time ON code_request_log (client_ip, requested_at);
