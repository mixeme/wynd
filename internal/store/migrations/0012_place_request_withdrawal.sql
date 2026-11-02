-- A23: отзыв конкретной просьбы при смене принимающего сервера.
-- Статус решения администратора сохраняется; отозванная просьба неактуальна.
ALTER TABLE quota_requests ADD COLUMN withdrawn_at TEXT;
ALTER TABLE quota_requests ADD COLUMN operation_key TEXT;
ALTER TABLE incoming_moves ADD COLUMN source_circle_id TEXT;
