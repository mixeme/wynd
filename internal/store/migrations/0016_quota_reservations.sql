-- Резервации места под ещё не записанные операции (план 60, R3): проверка
-- квоты и увеличение резерва — одно действие, поэтому параллельные запросы
-- не видят одно и то же свободное место. instance_bytes входит в резерв
-- сервера, circle_bytes — в занятое кругом. Строка живёт до подтверждения
-- или освобождения; просроченные (аварийный выход) не считаются.
CREATE TABLE quota_reservations (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL,
    instance_bytes INTEGER NOT NULL DEFAULT 0,
    circle_bytes INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE INDEX quota_reservations_circle ON quota_reservations(circle_id, created_at);
CREATE INDEX quota_reservations_created ON quota_reservations(created_at);
