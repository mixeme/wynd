-- 0027: кадр видео для «Дней» и «Сетки».
--
-- Ролик сам по себе не картинка: карточка дня и плитка сетки рисуют JPEG
-- первого кадра, который телефон снимает при отправке. Кадра нет — экраны
-- показывают первый кадр самим видео. Старые ролики колонки не имеют.

ALTER TABLE post_media ADD COLUMN video_poster_blob_id TEXT;

CREATE INDEX idx_post_media_video_poster ON post_media (video_poster_blob_id)
	WHERE video_poster_blob_id IS NOT NULL;
