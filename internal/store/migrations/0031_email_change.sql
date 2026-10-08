-- Смена почты (A8): код уходит на новую почту и привязан к учётке, которая
-- его запросила. Код живёт 15 минут, поэтому таблица пересоздаётся, а не
-- перекладывается: невведённые коды запрашиваются заново.
DROP TABLE pending_codes;

CREATE TABLE pending_codes (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL COLLATE NOCASE,
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL,
    flow TEXT NOT NULL CHECK (flow IN ('login', 'register', 'invite', 'email_change')),
    invite_id TEXT REFERENCES invites(id),
    invite_name TEXT,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_pending_codes_email ON pending_codes (email, created_at);
CREATE INDEX idx_pending_codes_account ON pending_codes (account_id) WHERE account_id IS NOT NULL;
