ALTER TABLE invites ADD COLUMN target_account_id TEXT REFERENCES accounts(id);

CREATE INDEX idx_invites_member_target ON invites (circle_id, target_account_id)
WHERE target_account_id IS NOT NULL AND revoked_at IS NULL;
