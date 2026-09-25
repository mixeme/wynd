-- Wave B: circle color on server (source of truth for street/feed tint).

ALTER TABLE circles ADD COLUMN color TEXT NOT NULL DEFAULT 'ochre';
