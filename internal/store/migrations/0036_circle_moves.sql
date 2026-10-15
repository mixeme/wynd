-- Перенос круга по ссылке (план 53, A1 этап 3). На прежнем сервере —
-- circle_moves: владелец выдал ссылку, круг замер, новый сервер забирает
-- полный архив без шифрования и сообщает «принято и сверено»; владелец
-- завершает перенос (круг удаляется) или отменяет его (круг оживает).
CREATE TABLE circle_moves (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    started_by_account_id TEXT NOT NULL REFERENCES accounts(id),
    started_at TEXT NOT NULL,
    -- sha256 ключа ссылки: сам ключ не хранится.
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
);

CREATE INDEX idx_circle_moves_circle ON circle_moves (circle_id, started_at);

-- На новом сервере — incoming_moves: кто вставил ссылку (он и станет
-- владельцем), карточка круга с прежнего сервера, ход и итог.
CREATE TABLE incoming_moves (
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
);

CREATE INDEX idx_incoming_moves_account ON incoming_moves (account_id, created_at);
CREATE INDEX idx_incoming_moves_status ON incoming_moves (status);
