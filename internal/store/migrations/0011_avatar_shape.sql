-- Форма аватаров на всех устройствах человека (план 46, A16; владелец
-- 2026-10-24: «Форма аватаров на все устройства»): 'circle' — круг, как было;
-- 'square' — скруглённый квадрат.
ALTER TABLE accounts ADD COLUMN avatar_shape TEXT NOT NULL DEFAULT 'circle';
