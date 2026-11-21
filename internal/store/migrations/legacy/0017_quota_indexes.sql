-- Индексы проверки квоты (план 60, раздел 8; замеры и планы —
-- docs/testing/quota-check-bench-2026-11-07.md). Проверка перед каждой
-- записью и вложением обходила все блобы сервера ради одного круга:
-- подзапрос медиа комментариев разворачивался в полный скан blobs.
-- comments(circle_id) делает его пропорциональным кругу; частичный индекс
-- pending убирает полный скан blobs из резерва; покрывающий индекс по
-- size_bytes считает реестр содержимого без чтения таблицы. Ветка GROUP BY
-- storage_path планировщиком не берётся без ANALYZE — индекс для неё
-- решено не добавлять (протокол, сессия 2).
CREATE INDEX idx_comments_circle ON comments (circle_id);
CREATE INDEX idx_blobs_pending_size ON blobs (size_bytes) WHERE status = 'pending';
CREATE INDEX idx_blob_contents_size ON blob_contents (size_bytes);
