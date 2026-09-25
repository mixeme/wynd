-- Wave E: admin can close login without deleting the account.

ALTER TABLE accounts ADD COLUMN blocked INTEGER NOT NULL DEFAULT 0;
