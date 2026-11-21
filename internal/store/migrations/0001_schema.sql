-- Схема базы целиком (C115): в этот файл свёрнута цепочка 0001–0017 прежней
-- нумерации (0.34.0–0.53.4). Нумерация начата заново (решение владельца
-- 2026-11-20): свежая база получает «версию 1» одним шагом; база прежней
-- нумерации, дошедшая до 17, переводится отметкой без изменений таблиц
-- (migrate.go), остановившаяся раньше — отказ с выпуском, который её доведёт.
-- Форма сверена с цепочкой 0001–0017: те же 52 таблицы, 44 индекса и
-- 33 триггера (TestSquashSchemaMatchesFormerChain); прежняя цепочка хранится
-- в migrations/legacy/ и поднимается только тестами.
--
-- Маяк новой нумерации — отметка версии 0: её прежняя нумерация не ставила,
-- по ней база узнаётся при запуске. Строки времени — TEXT фиксированной
-- ширины (xtime.Layout), порядок строк совпадает с порядком времени.
--
-- Оператору: база после свёртки не открывается бинарниками до свёртки —
-- откат только с восстановлением бэкапа.

-- Таблицы, индексы и триггеры — в порядке создания прежней цепочкой.

CREATE TABLE IF NOT EXISTS account_notify_prefs (
    account_id TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    posts INTEGER NOT NULL DEFAULT 1 CHECK (posts IN (0, 1)),
    comments INTEGER NOT NULL DEFAULT 1 CHECK (comments IN (0, 1)),
    reactions INTEGER NOT NULL DEFAULT 0 CHECK (reactions IN (0, 1)),
    comments_mine INTEGER NOT NULL DEFAULT 1 CHECK (comments_mine IN (0, 1)),
    comments_all INTEGER NOT NULL DEFAULT 0 CHECK (comments_all IN (0, 1)),
    events INTEGER NOT NULL DEFAULT 0 CHECK (events IN (0, 1)),
    mute_until TEXT
);

CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL COLLATE NOCASE UNIQUE,
    created_at TEXT NOT NULL,
    blocked INTEGER NOT NULL DEFAULT 0,
    last_login_at TEXT NULL,
    deleted_at TEXT NULL,
    subscription_expires_at TEXT,
    pay_donate_dismissed_version INTEGER NOT NULL DEFAULT 0,
    pay_reminder_sent_for TEXT
, avatar_shape TEXT NOT NULL DEFAULT 'circle');

CREATE TABLE IF NOT EXISTS admin_credentials (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    password_hash TEXT NOT NULL,
    reset_token TEXT,
    reset_expires_at TEXT,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS blobs (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    sha256 TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    mime_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'complete')),
    created_at TEXT NOT NULL,
    original_filename TEXT
, content_sha256 TEXT REFERENCES blob_contents(sha256) ON DELETE RESTRICT);

CREATE TABLE IF NOT EXISTS circle_archives (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    requested_by_account_id TEXT NOT NULL REFERENCES accounts(id),
    requested_at TEXT NOT NULL,
    snapshot_at TEXT,
    -- Архив с удалением круга (6.32): круг замирает, владелец убирает его сам.
    delete_circle INTEGER NOT NULL DEFAULT 0,
    encrypted INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('building', 'ready', 'failed', 'removed')),
    -- Пока собирается — оценка (место занято заранее), потом — точная длина файла.
    size_bytes INTEGER NOT NULL DEFAULT 0,
    ready_at TEXT,
    expires_at TEXT,
    downloaded_at TEXT,
    removed_reason TEXT CHECK (removed_reason IN ('downloaded', 'expired', 'admin', 'owner', 'canceled')),
    removed_at TEXT
, scope_revision INTEGER NOT NULL DEFAULT 0, full_journal INTEGER NOT NULL DEFAULT 1 CHECK (full_journal IN (0, 1)), posts INTEGER NOT NULL DEFAULT -1);

CREATE TABLE IF NOT EXISTS circle_arrivals (
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('moved', 'revived')),
    created_at TEXT NOT NULL,
    seen_at TEXT,
    PRIMARY KEY (circle_id, account_id)
);

CREATE TABLE IF NOT EXISTS circle_moves (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    started_by_account_id TEXT NOT NULL REFERENCES accounts(id),
    started_at TEXT NOT NULL,
    -- Ключ ссылки: владелец видит ссылку на 6.25, сколько бы раз ни открыл
    -- экран; новый сервер находит перенос по sha256 ключа.
    token TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('waiting', 'pulling', 'accepted', 'canceled', 'completed')),
    -- Новый сервер — из обмена: кто первым пришёл по ссылке.
    dest_url TEXT,
    dest_name TEXT,
    -- Архив под перенос: собирается, готов, сбой; длина и сколько забрано.
    archive_status TEXT NOT NULL DEFAULT 'none' CHECK (archive_status IN ('none', 'building', 'ready', 'failed')),
    archive_bytes INTEGER NOT NULL DEFAULT 0,
    pulled_bytes INTEGER NOT NULL DEFAULT 0,
    -- Что в архиве — для сверки с «принято» нового сервера (6.26).
    archive_posts INTEGER NOT NULL DEFAULT 0,
    archive_names INTEGER NOT NULL DEFAULT 0,
    archive_media_bytes INTEGER NOT NULL DEFAULT 0,
    accepted_at TEXT,
    accepted_posts INTEGER,
    accepted_names INTEGER,
    accepted_bytes INTEGER,
    ended_at TEXT
, snapshot_id TEXT REFERENCES move_snapshots(id) ON DELETE SET NULL, legacy_archive_id TEXT);

CREATE TABLE IF NOT EXISTS circle_notify_prefs (
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    posts INTEGER CHECK (posts IN (0, 1)),
    comments INTEGER CHECK (comments IN (0, 1)),
    reactions INTEGER CHECK (reactions IN (0, 1)),
    comments_mine INTEGER CHECK (comments_mine IN (0, 1)),
    comments_all INTEGER CHECK (comments_all IN (0, 1)),
    events INTEGER CHECK (events IN (0, 1)),
    mute_until TEXT,
    PRIMARY KEY (account_id, circle_id)
);

CREATE TABLE IF NOT EXISTS circles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    owner_account_id TEXT NOT NULL,
    edit_window_sec INTEGER, -- NULL = unlimited, 0 = chronicle
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    quota_bytes INTEGER,
    archive_cutoff_date TEXT,
    archive_deadline TEXT,
    archive_reminder_before_sec INTEGER,
    cutoff_locked_at TEXT,
    archive_reminder_sent_at TEXT,
    archive_cycle_started_at TEXT,
    color TEXT NOT NULL DEFAULT 'ochre',
    invite_who TEXT NOT NULL DEFAULT 'all' CHECK (invite_who IN ('all', 'owner')),
    invite_kind_default TEXT NOT NULL DEFAULT 'single' CHECK (invite_kind_default IN ('single', 'multi')),
    quota_custom INTEGER NOT NULL DEFAULT 0,
    -- Квота круга считает и текст (C42): байты строк круга в базе, ведут
    -- триггеры quota_text_* ниже.
    text_bytes INTEGER NOT NULL DEFAULT 0,
    -- Круг замер под архив с удалением или перенос (6.32, 6.36): читать
    -- можно, писать нельзя.
    frozen_at TEXT
, archive_cutoff_tz TEXT, archive_all INTEGER NOT NULL DEFAULT 1 CHECK (archive_all IN (0, 1)), archive_revision INTEGER NOT NULL DEFAULT 0);

CREATE TABLE IF NOT EXISTS code_request_log (
    client_ip TEXT NOT NULL,
    email TEXT NOT NULL COLLATE NOCASE,
    requested_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
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
    deleted INTEGER NOT NULL DEFAULT 0,
    client_id TEXT
);

CREATE TABLE IF NOT EXISTS day_covers (
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

CREATE TABLE IF NOT EXISTS day_titles (
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

CREATE TABLE IF NOT EXISTS days (
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    entry_date TEXT NOT NULL,
    title TEXT,
    title_event_seq INTEGER,
    cover_post_id TEXT,
    cover_blob_id TEXT,
    cover_event_seq INTEGER,
    PRIMARY KEY (circle_id, entry_date)
);

CREATE TABLE IF NOT EXISTS events (
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

CREATE TABLE IF NOT EXISTS identities (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    account_id TEXT,
    created_at TEXT NOT NULL,
    gender TEXT CHECK (gender IN ('m', 'f')),
    UNIQUE (circle_id, account_id)
);

CREATE TABLE IF NOT EXISTS identity_names (
    id TEXT PRIMARY KEY,
    identity_id TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    avatar_blob_id TEXT,
    effective_at TEXT NOT NULL,
    erased_at TEXT
);

CREATE TABLE IF NOT EXISTS incoming_moves (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- Ссылка с ключом: нужна, пока круг забирается; после — стирается.
    link TEXT NOT NULL,
    source_url TEXT NOT NULL,
    source_name TEXT NOT NULL DEFAULT '',
    circle_name TEXT NOT NULL,
    circle_color TEXT NOT NULL DEFAULT '',
    posts INTEGER NOT NULL DEFAULT 0,
    members INTEGER NOT NULL DEFAULT 0,
    bytes INTEGER NOT NULL DEFAULT 0,
    frozen_at TEXT,
    expires_at TEXT NOT NULL,
    send_invites INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('draft', 'pulling', 'reviving', 'done', 'failed')),
    pulled_bytes INTEGER NOT NULL DEFAULT 0,
    circle_id TEXT,
    result_posts INTEGER,
    result_names INTEGER,
    result_bytes INTEGER,
    verified INTEGER NOT NULL DEFAULT 0,
    invited INTEGER NOT NULL DEFAULT 0,
    not_joined INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
, already_here INTEGER NOT NULL DEFAULT 0, owner_name TEXT NOT NULL DEFAULT '', reviver_absent INTEGER NOT NULL DEFAULT 0, new_member TEXT, people TEXT NOT NULL DEFAULT '[]', source_circle_id TEXT, place_operation_key TEXT, archive_etag TEXT);

CREATE TABLE IF NOT EXISTS instance_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL DEFAULT '',
    registration_mode TEXT NOT NULL DEFAULT 'invite' CHECK (registration_mode IN ('open', 'invite', 'closed')),
    bootstrapped INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    storage_quota_bytes INTEGER NOT NULL DEFAULT 107374182400,
    compress_photo_max_px INTEGER NOT NULL DEFAULT 2048,
    compress_photo_quality INTEGER NOT NULL DEFAULT 80,
    compress_video_max_height INTEGER NOT NULL DEFAULT 1080,
    compress_video_bitrate_kbps INTEGER NOT NULL DEFAULT 6000,
    compress_attachment_max_bytes INTEGER NOT NULL DEFAULT 104857600,
    smtp_host TEXT NOT NULL DEFAULT '',
    smtp_port INTEGER NOT NULL DEFAULT 587,
    smtp_username TEXT NOT NULL DEFAULT '',
    smtp_password TEXT NOT NULL DEFAULT '',
    smtp_from TEXT NOT NULL DEFAULT '',
    smtp_test_sent_at TEXT,
    vapid_public_key TEXT NOT NULL DEFAULT '',
    vapid_private_key TEXT NOT NULL DEFAULT '',
    last_routine_at TEXT,
    last_backup_at TEXT,
    default_circle_quota_bytes INTEGER NULL,
    pay_requisites TEXT NOT NULL DEFAULT '',
    pay_donate_text TEXT NOT NULL DEFAULT '',
    pay_donate_show INTEGER NOT NULL DEFAULT 0 CHECK (pay_donate_show IN (0, 1)),
    pay_donate_dismissible INTEGER NOT NULL DEFAULT 1 CHECK (pay_donate_dismissible IN (0, 1)),
    pay_donate_until TEXT,
    pay_donate_version INTEGER NOT NULL DEFAULT 0,
    pay_subscription_required INTEGER NOT NULL DEFAULT 0 CHECK (pay_subscription_required IN (0, 1)),
    pay_subscription_remind_days INTEGER NOT NULL DEFAULT 7 CHECK (pay_subscription_remind_days IN (1, 3, 7)),
    storage_quota_disk_percent INTEGER NULL,
    smtp_last_error TEXT NOT NULL DEFAULT '',
    compress_audio_bitrate_kbps INTEGER NOT NULL DEFAULT 192,
    -- «Разрешить стирать имена и фото» (A3, 9.13): по умолчанию выключено.
    allow_name_erasure INTEGER NOT NULL DEFAULT 0,
    -- Сколько часов готовый полный архив ждёт скачивания (9.12).
    archive_retention_hours INTEGER NOT NULL DEFAULT 72
, admin_email TEXT NOT NULL DEFAULT '', admin_trouble_sent_at TEXT);

CREATE TABLE IF NOT EXISTS invites (
    id TEXT PRIMARY KEY,
    token TEXT NOT NULL UNIQUE,
    circle_id TEXT REFERENCES circles(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('single', 'multi')),
    max_uses INTEGER NOT NULL CHECK (max_uses > 0),
    uses INTEGER NOT NULL DEFAULT 0 CHECK (uses >= 0),
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_by_account_id TEXT REFERENCES accounts(id),
    created_at TEXT NOT NULL,
    target_account_id TEXT REFERENCES accounts(id)
);

CREATE TABLE IF NOT EXISTS membership_spans (
    id TEXT PRIMARY KEY,
    membership_id TEXT NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    can_read INTEGER NOT NULL DEFAULT 1,
    can_write INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS memberships (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    identity_id TEXT NOT NULL REFERENCES identities(id),
    can_settings INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('active', 'left_with_access', 'gone')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    share_place INTEGER NOT NULL DEFAULT 1,
    UNIQUE (circle_id, account_id)
);

CREATE TABLE IF NOT EXISTS pending_circle_joins (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT,
    invite_id TEXT REFERENCES invites(id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, circle_id),
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (circle_id) REFERENCES circles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS pending_codes (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL COLLATE NOCASE,
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL,
    flow TEXT NOT NULL CHECK (flow IN ('login', 'register', 'invite', 'email_change')),
    invite_id TEXT REFERENCES invites(id),
    invite_name TEXT,
    -- Смена почты (A8): код уходит на новую почту и привязан к учётке.
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS post_media (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id),
    kind TEXT NOT NULL CHECK (kind IN ('photo', 'video', 'attachment')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    captured_at TEXT,
    geo_lat REAL,
    geo_lng REAL,
    is_cover INTEGER NOT NULL DEFAULT 0,
    audio_artist TEXT,
    audio_title TEXT,
    audio_cover_blob_id TEXT,
    crop_x REAL,
    crop_y REAL,
    crop_w REAL,
    crop_h REAL,
    voice INTEGER NOT NULL DEFAULT 0,
    audio_duration_ms INTEGER,
    audio_peaks TEXT,
    video_poster_blob_id TEXT
, video_duration_ms INTEGER);

CREATE TABLE IF NOT EXISTS posts (
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
    deleted INTEGER NOT NULL DEFAULT 0,
    client_id TEXT
);

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS quota_requests (
    id TEXT PRIMARY KEY,
    circle_id TEXT REFERENCES circles(id) ON DELETE CASCADE,
    requested_by_account_id TEXT NOT NULL REFERENCES accounts(id),
    requested_bytes INTEGER NOT NULL CHECK (requested_bytes > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    admin_note TEXT,
    created_at TEXT NOT NULL,
    resolved_at TEXT,
    circle_name TEXT,
    used_at TEXT
, withdrawn_at TEXT, operation_key TEXT);

CREATE TABLE IF NOT EXISTS reactions (
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

CREATE TABLE IF NOT EXISTS read_cursors (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    last_read_seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('participant', 'admin')),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS upload_sessions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    expected_size INTEGER NOT NULL CHECK (expected_size > 0),
    mime_type TEXT NOT NULL,
    sha256 TEXT,
    received_bytes INTEGER NOT NULL DEFAULT 0 CHECK (received_bytes >= 0),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    original_filename TEXT
);

CREATE TABLE IF NOT EXISTS pay_requests (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id),
    comment TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at TEXT NOT NULL,
    resolved_at TEXT,
    blob_deleted INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS response_cursors (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

CREATE TABLE IF NOT EXISTS comment_media (
    id TEXT PRIMARY KEY,
    comment_id TEXT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id),
    kind TEXT NOT NULL CHECK (kind IN ('photo', 'attachment')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    voice INTEGER NOT NULL DEFAULT 0,
    audio_duration_ms INTEGER,
    audio_peaks TEXT
);

CREATE INDEX IF NOT EXISTS idx_blobs_account ON blobs (account_id);

CREATE INDEX IF NOT EXISTS idx_circle_archives_circle ON circle_archives (circle_id, requested_at);

CREATE INDEX IF NOT EXISTS idx_circle_archives_status ON circle_archives (status, expires_at);

CREATE INDEX IF NOT EXISTS idx_circle_moves_circle ON circle_moves (circle_id, started_at);

CREATE INDEX IF NOT EXISTS idx_code_request_log_ip_time ON code_request_log (client_ip, requested_at);

CREATE INDEX IF NOT EXISTS idx_comments_post ON comments (post_id, created_at);

CREATE INDEX IF NOT EXISTS idx_events_circle_created ON events (circle_id, created_at);

CREATE INDEX IF NOT EXISTS idx_events_circle_seq ON events (circle_id, seq);

CREATE INDEX IF NOT EXISTS idx_incoming_moves_account ON incoming_moves (account_id, created_at);

CREATE INDEX IF NOT EXISTS idx_incoming_moves_status ON incoming_moves (status);

CREATE INDEX IF NOT EXISTS idx_invites_token ON invites (token);

CREATE INDEX IF NOT EXISTS idx_membership_spans_membership ON membership_spans (membership_id, started_at);

CREATE INDEX IF NOT EXISTS idx_pending_codes_email ON pending_codes (email, created_at);

CREATE INDEX IF NOT EXISTS idx_pending_codes_account ON pending_codes (account_id) WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_post_media_blob ON post_media (blob_id);

CREATE INDEX IF NOT EXISTS idx_post_media_post ON post_media (post_id, sort_order);

CREATE INDEX IF NOT EXISTS idx_posts_circle_created ON posts (circle_id, created_at);

CREATE INDEX IF NOT EXISTS idx_posts_circle_entry_date ON posts (circle_id, entry_date);

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_account ON push_subscriptions (account_id);

CREATE INDEX IF NOT EXISTS idx_quota_requests_status ON quota_requests (status, created_at);

CREATE INDEX IF NOT EXISTS idx_reactions_post ON reactions (post_id, created_at);

CREATE INDEX IF NOT EXISTS idx_sessions_account ON sessions (account_id, kind);

CREATE INDEX IF NOT EXISTS idx_upload_sessions_account ON upload_sessions (account_id, expires_at);

CREATE INDEX IF NOT EXISTS idx_pay_requests_pending ON pay_requests(status, created_at);

CREATE INDEX IF NOT EXISTS idx_pay_requests_account ON pay_requests(account_id, status);

CREATE INDEX IF NOT EXISTS idx_invites_member_target ON invites (circle_id, target_account_id) WHERE target_account_id IS NOT NULL AND revoked_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_client_id ON posts (circle_id, client_id) WHERE client_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_comments_client_id ON comments (circle_id, client_id) WHERE client_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_pay_requests_one_pending ON pay_requests (account_id) WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_code_request_log_email_time ON code_request_log (email, requested_at);

CREATE INDEX IF NOT EXISTS idx_post_media_audio_cover ON post_media (audio_cover_blob_id) WHERE audio_cover_blob_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_post_media_video_poster ON post_media (video_poster_blob_id) WHERE video_poster_blob_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_comment_media_comment ON comment_media (comment_id, sort_order);

CREATE INDEX IF NOT EXISTS idx_comment_media_blob ON comment_media (blob_id);

CREATE VIRTUAL TABLE IF NOT EXISTS content_fts USING fts5(
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

CREATE TABLE IF NOT EXISTS 'content_fts_data'(id INTEGER PRIMARY KEY, block BLOB);

CREATE TABLE IF NOT EXISTS 'content_fts_idx'(segid, term, pgno, PRIMARY KEY(segid, term)) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS 'content_fts_content'(id INTEGER PRIMARY KEY, c0, c1, c2, c3, c4, c5, c6, c7);

CREATE TABLE IF NOT EXISTS 'content_fts_docsize'(id INTEGER PRIMARY KEY, sz BLOB);

CREATE TABLE IF NOT EXISTS 'content_fts_config'(k PRIMARY KEY, v) WITHOUT ROWID;

CREATE TRIGGER IF NOT EXISTS fts_comment_delete AFTER DELETE ON comments
BEGIN
    DELETE FROM content_fts WHERE comment_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS fts_comment_insert AFTER INSERT ON comments WHEN NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != ''
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT NEW.body, NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at, p.entry_date
    FROM posts p WHERE p.id = NEW.post_id;
END;

CREATE TRIGGER IF NOT EXISTS fts_comment_update AFTER UPDATE ON comments
BEGIN
    DELETE FROM content_fts WHERE comment_id = OLD.id AND kind = 'comment';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT NEW.body, NEW.post_id, NEW.circle_id, NEW.id, NEW.author_name, 'comment', NEW.created_at, p.entry_date
    FROM posts p WHERE p.id = NEW.post_id
      AND NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_post_delete AFTER DELETE ON posts
BEGIN
    DELETE FROM content_fts WHERE post_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS fts_post_insert AFTER INSERT ON posts WHEN NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != ''
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    VALUES (NEW.body, NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at, NEW.entry_date);
END;

CREATE TRIGGER IF NOT EXISTS fts_post_update AFTER UPDATE ON posts
BEGIN
    DELETE FROM content_fts WHERE post_id = OLD.id AND kind = 'post';
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT NEW.body, NEW.id, NEW.circle_id, '', NEW.author_name, 'post', NEW.created_at, NEW.entry_date
    WHERE NEW.deleted = 0 AND NEW.body IS NOT NULL AND NEW.body != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_media_insert AFTER INSERT ON post_media WHEN NEW.kind = 'attachment'
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT trim(COALESCE(NEW.audio_artist, '') || ' ' || COALESCE(NEW.audio_title, '') || ' ' || COALESCE(b.original_filename, '')),
        p.id, p.circle_id, NEW.id, p.author_name,
        CASE WHEN b.mime_type LIKE 'audio/%' THEN 'audio' ELSE 'file' END,
        p.created_at, p.entry_date
    FROM posts p
    JOIN blobs b ON b.id = NEW.blob_id
    WHERE p.id = NEW.post_id
      AND trim(COALESCE(NEW.audio_title, '') || COALESCE(NEW.audio_artist, '') || COALESCE(b.original_filename, '')) != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_media_update AFTER UPDATE ON post_media
BEGIN
    DELETE FROM content_fts WHERE kind IN ('file', 'audio') AND comment_id = OLD.id;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT trim(COALESCE(NEW.audio_artist, '') || ' ' || COALESCE(NEW.audio_title, '') || ' ' || COALESCE(b.original_filename, '')),
        p.id, p.circle_id, NEW.id, p.author_name,
        CASE WHEN b.mime_type LIKE 'audio/%' THEN 'audio' ELSE 'file' END,
        p.created_at, p.entry_date
    FROM posts p
    JOIN blobs b ON b.id = NEW.blob_id
    WHERE p.id = NEW.post_id AND NEW.kind = 'attachment'
      AND trim(COALESCE(NEW.audio_title, '') || COALESCE(NEW.audio_artist, '') || COALESCE(b.original_filename, '')) != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_media_delete AFTER DELETE ON post_media
BEGIN
    DELETE FROM content_fts WHERE kind IN ('file', 'audio') AND comment_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS fts_media_post_update AFTER UPDATE OF entry_date, author_name ON posts
BEGIN
    UPDATE content_fts SET entry_date = NEW.entry_date, author_name = NEW.author_name
    WHERE post_id = NEW.id AND kind IN ('file', 'audio');
END;

CREATE TRIGGER IF NOT EXISTS fts_day_insert AFTER INSERT ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.event_seq = (
        SELECT MAX(event_seq) FROM day_titles
        WHERE circle_id = NEW.circle_id AND entry_date = NEW.entry_date AND deleted = 0
    ) AND dt.title IS NOT NULL AND dt.title != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_day_update AFTER UPDATE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.event_seq = (
        SELECT MAX(event_seq) FROM day_titles
        WHERE circle_id = NEW.circle_id AND entry_date = NEW.entry_date AND deleted = 0
    ) AND dt.title IS NOT NULL AND dt.title != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_day_delete AFTER DELETE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = OLD.circle_id AND entry_date = OLD.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.event_seq = (
        SELECT MAX(event_seq) FROM day_titles
        WHERE circle_id = OLD.circle_id AND entry_date = OLD.entry_date AND deleted = 0
    ) AND dt.title IS NOT NULL AND dt.title != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_comment_media_insert AFTER INSERT ON comment_media
WHEN NEW.kind = 'attachment' AND NEW.voice = 0
BEGIN
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT trim(b.original_filename), c.post_id, c.circle_id, NEW.id, c.author_name,
        CASE WHEN b.mime_type LIKE 'audio/%' THEN 'caudio' ELSE 'cfile' END,
        c.created_at, p.entry_date
    FROM comments c
    JOIN posts p ON p.id = c.post_id
    JOIN blobs b ON b.id = NEW.blob_id
    WHERE c.id = NEW.comment_id
      AND trim(COALESCE(b.original_filename, '')) != '';
END;

CREATE TRIGGER IF NOT EXISTS fts_comment_media_delete AFTER DELETE ON comment_media
BEGIN
    DELETE FROM content_fts WHERE kind IN ('cfile', 'caudio') AND comment_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS fts_comment_media_post_update AFTER UPDATE OF entry_date ON posts
BEGIN
    UPDATE content_fts SET entry_date = NEW.entry_date
    WHERE post_id = NEW.id AND kind IN ('cfile', 'caudio');
END;

CREATE TRIGGER IF NOT EXISTS quota_text_post_insert AFTER INSERT ON posts
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 550
        + 3 * length(CAST(COALESCE(NEW.body, '') AS BLOB)) + length(CAST(NEW.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_post_update AFTER UPDATE OF body, author_name ON posts
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + 3 * (length(CAST(COALESCE(NEW.body, '') AS BLOB)) - length(CAST(COALESCE(OLD.body, '') AS BLOB)))
        + length(CAST(NEW.author_name AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_post_delete AFTER DELETE ON posts
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 550
        - 3 * length(CAST(COALESCE(OLD.body, '') AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_comment_insert AFTER INSERT ON comments
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 560
        + 3 * length(CAST(COALESCE(NEW.body, '') AS BLOB)) + length(CAST(NEW.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_comment_update AFTER UPDATE OF body, author_name ON comments
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + 3 * (length(CAST(COALESCE(NEW.body, '') AS BLOB)) - length(CAST(COALESCE(OLD.body, '') AS BLOB)))
        + length(CAST(NEW.author_name AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_comment_delete AFTER DELETE ON comments
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 560
        - 3 * length(CAST(COALESCE(OLD.body, '') AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_reaction_insert AFTER INSERT ON reactions
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 450
        + length(CAST(NEW.emoji AS BLOB)) + length(CAST(NEW.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_reaction_update AFTER UPDATE OF emoji, author_name ON reactions
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + length(CAST(NEW.emoji AS BLOB)) - length(CAST(OLD.emoji AS BLOB))
        + length(CAST(NEW.author_name AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_reaction_delete AFTER DELETE ON reactions
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 450
        - length(CAST(OLD.emoji AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_event_insert AFTER INSERT ON events
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 400
        + length(CAST(NEW.payload AS BLOB)) + length(CAST(NEW.summary AS BLOB)) + length(CAST(NEW.actor_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_event_update AFTER UPDATE OF payload, summary, actor_name ON events
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + length(CAST(NEW.payload AS BLOB)) - length(CAST(OLD.payload AS BLOB))
        + length(CAST(NEW.summary AS BLOB)) - length(CAST(OLD.summary AS BLOB))
        + length(CAST(NEW.actor_name AS BLOB)) - length(CAST(OLD.actor_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_event_delete AFTER DELETE ON events
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 400
        - length(CAST(OLD.payload AS BLOB)) - length(CAST(OLD.summary AS BLOB)) - length(CAST(OLD.actor_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_title_insert AFTER INSERT ON day_titles
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 250 + 3 * length(CAST(NEW.title AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS quota_text_title_delete AFTER DELETE ON day_titles
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 250 - 3 * length(CAST(OLD.title AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TABLE IF NOT EXISTS circle_pins (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    pinned_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

CREATE TABLE IF NOT EXISTS circle_groups (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (account_id, name)
);

CREATE TABLE IF NOT EXISTS circle_group_items (
    group_id TEXT NOT NULL REFERENCES circle_groups(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    added_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

CREATE INDEX IF NOT EXISTS idx_circle_group_items_group ON circle_group_items(group_id);

CREATE TRIGGER IF NOT EXISTS circle_group_drop_empty AFTER DELETE ON circle_group_items
WHEN NOT EXISTS (SELECT 1 FROM circle_group_items WHERE group_id = OLD.group_id)
BEGIN
    DELETE FROM circle_groups WHERE id = OLD.group_id;
END;

CREATE TRIGGER IF NOT EXISTS circle_pin_gone AFTER UPDATE OF status ON memberships
WHEN NEW.status = 'gone'
BEGIN
    DELETE FROM circle_pins WHERE account_id = NEW.account_id AND circle_id = NEW.circle_id;
END;

CREATE TRIGGER IF NOT EXISTS circle_group_leave AFTER UPDATE OF status ON memberships
WHEN NEW.status = 'gone'
BEGIN
    DELETE FROM circle_group_items WHERE account_id = NEW.account_id AND circle_id = NEW.circle_id;
END;

CREATE TABLE IF NOT EXISTS archive_spans (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    started_at TEXT NOT NULL,
    ended_at TEXT
);

CREATE INDEX IF NOT EXISTS archive_spans_circle ON archive_spans(circle_id);

CREATE TABLE IF NOT EXISTS move_snapshots (
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

CREATE TABLE IF NOT EXISTS move_snapshot_blobs (
    snapshot_id TEXT NOT NULL REFERENCES move_snapshots(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id) ON DELETE RESTRICT,
    PRIMARY KEY (snapshot_id, blob_id)
);

CREATE INDEX IF NOT EXISTS move_snapshot_blobs_blob ON move_snapshot_blobs(blob_id);

CREATE TABLE IF NOT EXISTS move_snapshot_literals (
    snapshot_id TEXT NOT NULL REFERENCES move_snapshots(id) ON DELETE CASCADE,
    part INTEGER NOT NULL,
    data BLOB NOT NULL,
    PRIMARY KEY (snapshot_id, part)
);

CREATE TABLE IF NOT EXISTS blob_contents (
    sha256 TEXT PRIMARY KEY CHECK(length(sha256) = 64),
    storage_path TEXT NOT NULL UNIQUE,
    size_bytes INTEGER NOT NULL CHECK(size_bytes >= 0),
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS blobs_content ON blobs(content_sha256);

CREATE INDEX IF NOT EXISTS blobs_storage_path ON blobs(storage_path);

CREATE TABLE IF NOT EXISTS blob_file_gc (storage_path TEXT PRIMARY KEY);

CREATE TABLE IF NOT EXISTS blob_backup_leases (id TEXT PRIMARY KEY, expires_at INTEGER NOT NULL);

CREATE TABLE IF NOT EXISTS quota_reservations (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL,
    instance_bytes INTEGER NOT NULL DEFAULT 0,
    circle_bytes INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS quota_reservations_circle ON quota_reservations(circle_id, created_at);

CREATE INDEX IF NOT EXISTS quota_reservations_created ON quota_reservations(created_at);

CREATE INDEX IF NOT EXISTS idx_comments_circle ON comments (circle_id);

CREATE INDEX IF NOT EXISTS idx_blobs_pending_size ON blobs (size_bytes) WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_blob_contents_size ON blob_contents (size_bytes);

-- Засев прежней цепочки: единственная строка настроек сервера
-- (потолок хранилища — 80 % диска). Тени полнотекстового индекса
-- заполняют триггеры FTS — их не засеваем.
INSERT INTO "instance_settings" ("id", "name", "registration_mode", "bootstrapped", "updated_at", "storage_quota_bytes", "compress_photo_max_px", "compress_photo_quality", "compress_video_max_height", "compress_video_bitrate_kbps", "compress_attachment_max_bytes", "smtp_host", "smtp_port", "smtp_username", "smtp_password", "smtp_from", "smtp_test_sent_at", "vapid_public_key", "vapid_private_key", "last_routine_at", "last_backup_at", "default_circle_quota_bytes", "pay_requisites", "pay_donate_text", "pay_donate_show", "pay_donate_dismissible", "pay_donate_until", "pay_donate_version", "pay_subscription_required", "pay_subscription_remind_days", "storage_quota_disk_percent", "smtp_last_error", "compress_audio_bitrate_kbps", "allow_name_erasure", "archive_retention_hours", "admin_email", "admin_trouble_sent_at") VALUES (1, '', 'invite', 0, '1970-01-01T00:00:00.000000000Z', 107374182400, 2048, 80, 1080, 6000, 104857600, '', 587, '', '', '', NULL, '', '', NULL, NULL, NULL, '', '', 0, 1, NULL, 0, 0, 7, 80, '', 192, 0, 72, '', NULL);

-- Маяк новой нумерации (C115): отметка 0, у прежней нумерации её не было.
INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (0, '1970-01-01T00:00:00.000000000Z');
