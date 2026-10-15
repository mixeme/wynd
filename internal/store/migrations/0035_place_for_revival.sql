-- Место для круга, которого пока нет (план 52; кадры 2.23, 2.24, 2.26): кто
-- оживляет большой круг из архива, просит место у администратора до того,
-- как круг появится. Просьба без круга — с именем будущего круга; одобренная
-- отмечается использованной, когда круг ожил (он получает свою квоту).
CREATE TABLE quota_requests_new (
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
);

INSERT INTO quota_requests_new (id, circle_id, requested_by_account_id, requested_bytes, status, admin_note, created_at, resolved_at)
SELECT id, circle_id, requested_by_account_id, requested_bytes, status, admin_note, created_at, resolved_at FROM quota_requests;

DROP TABLE quota_requests;
ALTER TABLE quota_requests_new RENAME TO quota_requests;
CREATE INDEX idx_quota_requests_status ON quota_requests (status, created_at);
