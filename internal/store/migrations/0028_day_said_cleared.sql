-- 0028: «убрали название дня» — своя строка day_titles с пустым названием.
--
-- Название и обложка дня больше не стираются по окну правок: снятие — такая
-- же запись журнала, как установка, и действует последняя. Триггеры поиска
-- брали последнюю НЕПУСТУЮ версию названия, и после снятия день находился бы
-- по старому имени. Теперь берётся последняя версия, и в поиск она идёт,
-- только если не пуста.
--
-- Обложке миграция не нужна: её снятие — строка day_covers с пустым blob_id.

DROP TRIGGER IF EXISTS fts_day_delete;
DROP TRIGGER IF EXISTS fts_day_insert;
DROP TRIGGER IF EXISTS fts_day_update;

CREATE TRIGGER fts_day_insert AFTER INSERT ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.event_seq = (
        SELECT MAX(event_seq) FROM day_titles
        WHERE circle_id = NEW.circle_id AND entry_date = NEW.entry_date AND deleted = 0
    ) AND dt.title IS NOT NULL AND dt.title != '';
END;

CREATE TRIGGER fts_day_update AFTER UPDATE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.event_seq = (
        SELECT MAX(event_seq) FROM day_titles
        WHERE circle_id = NEW.circle_id AND entry_date = NEW.entry_date AND deleted = 0
    ) AND dt.title IS NOT NULL AND dt.title != '';
END;

CREATE TRIGGER fts_day_delete AFTER DELETE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = OLD.circle_id AND entry_date = OLD.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.event_seq = (
        SELECT MAX(event_seq) FROM day_titles
        WHERE circle_id = OLD.circle_id AND entry_date = OLD.entry_date AND deleted = 0
    ) AND dt.title IS NOT NULL AND dt.title != '';
END;
