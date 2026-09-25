-- Chronicle domain tables (stage 2).

CREATE TABLE circles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    owner_account_id TEXT NOT NULL,
    edit_window_sec INTEGER, -- NULL = unlimited, 0 = chronicle
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE identities (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
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

CREATE TABLE membership_spans (
    id TEXT PRIMARY KEY,
    membership_id TEXT NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    can_read INTEGER NOT NULL DEFAULT 1,
    can_write INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_membership_spans_membership ON membership_spans (membership_id, started_at);

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

CREATE INDEX idx_events_circle_seq ON events (circle_id, seq);
CREATE INDEX idx_events_circle_created ON events (circle_id, created_at);

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

CREATE INDEX idx_posts_circle_created ON posts (circle_id, created_at);
CREATE INDEX idx_posts_circle_entry_date ON posts (circle_id, entry_date);

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

CREATE INDEX idx_comments_post ON comments (post_id, created_at);

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

CREATE INDEX idx_reactions_post ON reactions (post_id, created_at);

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
