-- Wave F: default circle quota, quota_custom, account soft-delete, login tracking.

ALTER TABLE instance_settings ADD COLUMN default_circle_quota_bytes INTEGER NULL;

ALTER TABLE circles ADD COLUMN quota_custom INTEGER NOT NULL DEFAULT 0;
UPDATE circles SET quota_custom = 1 WHERE quota_bytes IS NOT NULL;

ALTER TABLE accounts ADD COLUMN last_login_at TEXT NULL;
ALTER TABLE accounts ADD COLUMN deleted_at TEXT NULL;

PRAGMA foreign_keys=OFF;

CREATE TABLE identities_new (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    account_id TEXT,
    created_at TEXT NOT NULL,
    UNIQUE (circle_id, account_id)
);

INSERT INTO identities_new SELECT id, circle_id, account_id, created_at FROM identities;
DROP TABLE identities;
ALTER TABLE identities_new RENAME TO identities;

PRAGMA foreign_keys=ON;
