-- 0012: триггеры названия дня проецируют актуальную версию, а не последнюю
-- тронутую строку.
--
-- Строки day_titles версионны: у одного дня их несколько, актуальная — с
-- наибольшим event_seq среди неудалённых и непустых. Триггеры же адресовали
-- FTS по паре (circle_id, entry_date) и на любое касание **старой** версии
-- стирали строку поиска, оставляя день ненаходимым; purge старых названий
-- уносил из поиска живой день (SRCH-3).
--
-- Теперь все три триггера пересобирают строку поиска из таблицы: удаляют
-- запись дня и вставляют актуальную версию, если она есть.
--
-- (В плане 42 эта миграция названа 0013; номера сдвинуты, см. 0010.)

DROP TRIGGER IF EXISTS fts_day_delete;
DROP TRIGGER IF EXISTS fts_day_insert;
DROP TRIGGER IF EXISTS fts_day_update;

CREATE TRIGGER fts_day_insert AFTER INSERT ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.circle_id = NEW.circle_id AND dt.entry_date = NEW.entry_date
      AND dt.deleted = 0 AND dt.title IS NOT NULL AND dt.title != ''
    ORDER BY dt.event_seq DESC
    LIMIT 1;
END;

CREATE TRIGGER fts_day_update AFTER UPDATE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.circle_id = NEW.circle_id AND dt.entry_date = NEW.entry_date
      AND dt.deleted = 0 AND dt.title IS NOT NULL AND dt.title != ''
    ORDER BY dt.event_seq DESC
    LIMIT 1;
END;

CREATE TRIGGER fts_day_delete AFTER DELETE ON day_titles
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = OLD.circle_id AND entry_date = OLD.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
    FROM day_titles dt
    WHERE dt.circle_id = OLD.circle_id AND dt.entry_date = OLD.entry_date
      AND dt.deleted = 0 AND dt.title IS NOT NULL AND dt.title != ''
    ORDER BY dt.event_seq DESC
    LIMIT 1;
END;

-- Пересборка поиска по дням: существующие строки могли остаться от старых
-- версий названий или пропасть вовсе.
DELETE FROM content_fts WHERE kind = 'day';

INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
SELECT dt.title, '', dt.circle_id, '', '', 'day', dt.created_at, dt.entry_date
FROM day_titles dt
INNER JOIN (
    SELECT circle_id, entry_date, MAX(event_seq) AS max_seq
    FROM day_titles
    WHERE deleted = 0 AND title IS NOT NULL AND title != ''
    GROUP BY circle_id, entry_date
) cur ON cur.circle_id = dt.circle_id AND cur.entry_date = dt.entry_date AND cur.max_seq = dt.event_seq;
