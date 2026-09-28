-- 0017: таблица blob_refs снята.
--
-- Она дублировала ссылающиеся таблицы: строка писалась только в той же
-- транзакции, что post_media (вложение записи) или pay_requests (скриншот
-- оплаты), и снималась вместе с ними. Кто держит блоб, давно решает
-- предикат internal/blob по самим таблицам (post_media, day_covers, days,
-- identity_names, pay_requests), а blob_refs лишь отставала: каскад удаления
-- круга её не снимал (BLB-4). Данных, которых нет в других таблицах, в ней
-- не было, поэтому переносить нечего.

DROP INDEX IF EXISTS idx_blob_refs_ref;
DROP TABLE IF EXISTS blob_refs;
