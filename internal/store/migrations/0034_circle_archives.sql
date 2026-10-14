-- Полный архив круга (план 51, A1): владелец просит собрать файл со всем
-- кругом, сервер собирает его в фоне и держит до скачивания или до срока.
-- Готовые и собираемые архивы входят в потолок сервера (9.5), но не в квоту
-- круга (решение владельца 2026-10-13: файл временный).
CREATE TABLE circle_archives (
    id TEXT PRIMARY KEY,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    requested_by_account_id TEXT NOT NULL REFERENCES accounts(id),
    requested_at TEXT NOT NULL,
    snapshot_at TEXT,
    -- Архив с удалением круга (6.32): круг замирает, владелец убирает его сам.
    delete_circle INTEGER NOT NULL DEFAULT 0,
    encrypted INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('building', 'ready', 'failed', 'removed')),
    -- Пока собирается — оценка (место занято заранее), потом — точная длина файла.
    size_bytes INTEGER NOT NULL DEFAULT 0,
    ready_at TEXT,
    expires_at TEXT,
    downloaded_at TEXT,
    removed_reason TEXT CHECK (removed_reason IN ('downloaded', 'expired', 'admin', 'owner', 'canceled')),
    removed_at TEXT
);

CREATE INDEX idx_circle_archives_circle ON circle_archives (circle_id, requested_at);
CREATE INDEX idx_circle_archives_status ON circle_archives (status, expires_at);

-- Круг замер под архив с удалением (6.32, 6.36): читать можно, писать нельзя.
ALTER TABLE circles ADD COLUMN frozen_at TEXT;

-- Сколько часов готовый архив ждёт скачивания (9.12), по умолчанию 72.
ALTER TABLE instance_settings ADD COLUMN archive_retention_hours INTEGER NOT NULL DEFAULT 72;
