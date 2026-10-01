-- 0023: битрейт сжатия звука (план 46, A6).
--
-- Звуковые вложения телефон перекодирует перед загрузкой, как видео
-- (WebCodecs, AAC или Opus в MP4). Порог — здесь, рядом с порогами фото и
-- видео; уже сжатые MP3/AAC/Opus не выше порога уходят как есть.
-- 128 кбит/с — музыка без слышимых потерь, голос с запасом.

ALTER TABLE instance_settings ADD COLUMN compress_audio_bitrate_kbps INTEGER NOT NULL DEFAULT 128;
