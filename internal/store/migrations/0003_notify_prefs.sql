ALTER TABLE account_notify_prefs ADD COLUMN comments_mine INTEGER NOT NULL DEFAULT 1 CHECK (comments_mine IN (0, 1));
ALTER TABLE account_notify_prefs ADD COLUMN comments_all INTEGER NOT NULL DEFAULT 0 CHECK (comments_all IN (0, 1));
ALTER TABLE account_notify_prefs ADD COLUMN events INTEGER NOT NULL DEFAULT 0 CHECK (events IN (0, 1));
ALTER TABLE account_notify_prefs ADD COLUMN mute_until TEXT;

UPDATE account_notify_prefs SET comments_mine = comments;

ALTER TABLE circle_notify_prefs ADD COLUMN comments_mine INTEGER CHECK (comments_mine IN (0, 1));
ALTER TABLE circle_notify_prefs ADD COLUMN comments_all INTEGER CHECK (comments_all IN (0, 1));
ALTER TABLE circle_notify_prefs ADD COLUMN events INTEGER CHECK (events IN (0, 1));
ALTER TABLE circle_notify_prefs ADD COLUMN mute_until TEXT;

UPDATE circle_notify_prefs SET comments_mine = comments WHERE comments IS NOT NULL;
