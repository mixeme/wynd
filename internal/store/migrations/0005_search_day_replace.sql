DROP TRIGGER IF EXISTS fts_day_insert;
CREATE TRIGGER fts_day_insert AFTER INSERT ON day_titles WHEN NEW.deleted = 0 AND NEW.title IS NOT NULL AND NEW.title != ''
BEGIN
    DELETE FROM content_fts WHERE kind = 'day' AND circle_id = NEW.circle_id AND entry_date = NEW.entry_date;
    INSERT INTO content_fts (body, post_id, circle_id, comment_id, author_name, kind, created_at, entry_date)
    VALUES (NEW.title, '', NEW.circle_id, '', '', 'day', NEW.created_at, NEW.entry_date);
END;
