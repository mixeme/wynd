-- 0026: голосовые сообщения (план 46, C14).
--
-- Голосовое — звуковое вложение записи, записанное в приложении. Лента
-- рисует его строкой с волной и длительностью, не скачивая файл, поэтому
-- волну (до 64 уровней 0–100 через запятую) и длительность в миллисекундах
-- телефон кладёт рядом при отправке. Старые строки — обычные звуки.

ALTER TABLE post_media ADD COLUMN voice INTEGER NOT NULL DEFAULT 0;
ALTER TABLE post_media ADD COLUMN audio_duration_ms INTEGER;
ALTER TABLE post_media ADD COLUMN audio_peaks TEXT;
