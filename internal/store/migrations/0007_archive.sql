-- Archive cycle columns on circles.
ALTER TABLE circles ADD COLUMN archive_cutoff_date TEXT;
ALTER TABLE circles ADD COLUMN archive_deadline TEXT;
ALTER TABLE circles ADD COLUMN archive_reminder_before_sec INTEGER;
ALTER TABLE circles ADD COLUMN cutoff_locked_at TEXT;
ALTER TABLE circles ADD COLUMN archive_reminder_sent_at TEXT;
ALTER TABLE circles ADD COLUMN archive_cycle_started_at TEXT;
