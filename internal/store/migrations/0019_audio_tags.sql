-- 0019: исполнитель, название и обложка звукового вложения.
--
-- Файл сервер не разбирает: телефон отправителя читает теги и кладёт
-- небольшую обложку отдельным блобом. Старые строки остаются пустыми —
-- у них по-прежнему имя файла и пустая плитка. Обложка не входит в сетку
-- и альбом: это не kind=photo, а ссылка со строки вложения.

ALTER TABLE post_media ADD COLUMN audio_artist TEXT;
ALTER TABLE post_media ADD COLUMN audio_title TEXT;
ALTER TABLE post_media ADD COLUMN audio_cover_blob_id TEXT;

CREATE INDEX idx_post_media_audio_cover ON post_media (audio_cover_blob_id)
	WHERE audio_cover_blob_id IS NOT NULL;
