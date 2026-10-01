-- 0021: кадр обложки записи (4.16).
--
-- Квадрат снимка в долях его ширины и высоты. Новый файл не пишется: лента
-- рисует этот кусок того же снимка, альбом и лайтбокс — снимок целиком.
-- Пусто — как раньше, по центру.

ALTER TABLE post_media ADD COLUMN crop_x REAL;
ALTER TABLE post_media ADD COLUMN crop_y REAL;
ALTER TABLE post_media ADD COLUMN crop_w REAL;
ALTER TABLE post_media ADD COLUMN crop_h REAL;
