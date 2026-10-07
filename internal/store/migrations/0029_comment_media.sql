-- 0029: вложения комментариев (кадры 4.28–4.30) — фото, голосовое и файл.
--
-- Своя таблица, а не post_media с comment_id: вложение комментария — часть
-- разговора, а не журнала. «Сетка», «Карта», альбом и обложка дня, поиск по
-- файлам читают post_media и вложений комментариев не видят.
--
-- Комментарий стирается пометкой deleted = 1, строка остаётся — поэтому
-- вложения снимаются явно при удалении комментария, записи и при отсечке;
-- ON DELETE CASCADE сработает только вместе с кругом.
CREATE TABLE comment_media (
    id TEXT PRIMARY KEY,
    comment_id TEXT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    blob_id TEXT NOT NULL REFERENCES blobs(id),
    kind TEXT NOT NULL CHECK (kind IN ('photo', 'attachment')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    voice INTEGER NOT NULL DEFAULT 0,
    audio_duration_ms INTEGER,
    audio_peaks TEXT
);

CREATE INDEX idx_comment_media_comment ON comment_media (comment_id, sort_order);
CREATE INDEX idx_comment_media_blob ON comment_media (blob_id);
