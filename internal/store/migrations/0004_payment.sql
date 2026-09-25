ALTER TABLE instance_settings ADD COLUMN pay_requisites TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN pay_donate_text TEXT NOT NULL DEFAULT '';
ALTER TABLE instance_settings ADD COLUMN pay_donate_show INTEGER NOT NULL DEFAULT 0 CHECK (pay_donate_show IN (0, 1));
ALTER TABLE instance_settings ADD COLUMN pay_donate_dismissible INTEGER NOT NULL DEFAULT 1 CHECK (pay_donate_dismissible IN (0, 1));
ALTER TABLE instance_settings ADD COLUMN pay_donate_until TEXT;
ALTER TABLE instance_settings ADD COLUMN pay_donate_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE instance_settings ADD COLUMN pay_subscription_required INTEGER NOT NULL DEFAULT 0 CHECK (pay_subscription_required IN (0, 1));
ALTER TABLE instance_settings ADD COLUMN pay_subscription_remind_days INTEGER NOT NULL DEFAULT 7 CHECK (pay_subscription_remind_days IN (1, 3, 7));

ALTER TABLE accounts ADD COLUMN subscription_expires_at TEXT;
ALTER TABLE accounts ADD COLUMN pay_donate_dismissed_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN pay_reminder_sent_for TEXT;

CREATE TABLE pay_requests (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id),
    comment TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at TEXT NOT NULL,
    resolved_at TEXT,
    blob_deleted INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_pay_requests_pending ON pay_requests(status, created_at);
CREATE INDEX idx_pay_requests_account ON pay_requests(account_id, status);
