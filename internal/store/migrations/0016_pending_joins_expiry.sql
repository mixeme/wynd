-- 0016: у отложенного вступления появляются срок и ссылка-источник.
--
-- Аудит 2026-09-22 (SEC-9): строка pending_circle_joins — это и есть право
-- войти в круг (POST /invites/{token}/join проверяет только её). Она жила
-- вечно и ни от чего не зависела: отзыв ссылки, исключение пригласившего и
-- блокировка его учётки её не трогали, и вход по ней открывался и через
-- месяц. Теперь у строки есть срок, а invite_id связывает её со ссылкой,
-- по которой она заведена, — отзыв ссылки снимает и её.
--
-- Существующим строкам срок ставится от их created_at: обычное вступление
-- завершается за минуты, так что живых среди них почти наверняка нет.

ALTER TABLE pending_circle_joins ADD COLUMN expires_at TEXT;
ALTER TABLE pending_circle_joins ADD COLUMN invite_id TEXT REFERENCES invites(id) ON DELETE CASCADE;

UPDATE pending_circle_joins
SET expires_at = strftime('%Y-%m-%dT%H:%M:%f', substr(created_at, 1, 23) || 'Z', '+1 days') || '000000Z'
WHERE expires_at IS NULL;
