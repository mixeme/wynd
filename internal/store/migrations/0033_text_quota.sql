-- Квота круга считает и текст (план 46, C42): диск сервера можно заполнить
-- текстом, а шкала его не видела. Текст меряется местом на диске — сколько
-- байт занимают строки круга в базе: текст лежит в своей строке, в событии
-- журнала и в поисковом индексе, плюс служебная часть строки и индексов.
-- Постоянные на строку подобраны замером роста файла базы (C42, 2026-10-14).
-- Счёт ведут триггеры, чтобы ни один путь записи его не обошёл.
ALTER TABLE circles ADD COLUMN text_bytes INTEGER NOT NULL DEFAULT 0;

-- Вес строки: служебная часть + байты текста. Тело записи, комментария и
-- название дня лежат ещё в поисковом индексе — и копией, и словарём индекса,
-- поэтому их байты считаются трижды. Замер 2026-10-14 (600 записей по 60 и
-- 3000 байт русского текста, файл после VACUUM): запись — около 1 КБ
-- служебного и 4 байта на байт текста (третий — копия в событии журнала).
-- Служебное: запись 550, комментарий 560, реакция 450, событие 400,
-- название дня 250 байт.
UPDATE circles SET text_bytes =
    COALESCE((SELECT SUM(550 + 3 * length(CAST(COALESCE(p.body, '') AS BLOB)) + length(CAST(p.author_name AS BLOB)))
        FROM posts p WHERE p.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(560 + 3 * length(CAST(COALESCE(c.body, '') AS BLOB)) + length(CAST(c.author_name AS BLOB)))
        FROM comments c WHERE c.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(450 + length(CAST(r.emoji AS BLOB)) + length(CAST(r.author_name AS BLOB)))
        FROM reactions r WHERE r.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(400 + length(CAST(e.payload AS BLOB)) + length(CAST(e.summary AS BLOB)) + length(CAST(e.actor_name AS BLOB)))
        FROM events e WHERE e.circle_id = circles.id), 0)
  + COALESCE((SELECT SUM(250 + 3 * length(CAST(t.title AS BLOB)))
        FROM day_titles t WHERE t.circle_id = circles.id), 0);

CREATE TRIGGER quota_text_post_insert AFTER INSERT ON posts
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 550
        + 3 * length(CAST(COALESCE(NEW.body, '') AS BLOB)) + length(CAST(NEW.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_post_update AFTER UPDATE OF body, author_name ON posts
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + 3 * (length(CAST(COALESCE(NEW.body, '') AS BLOB)) - length(CAST(COALESCE(OLD.body, '') AS BLOB)))
        + length(CAST(NEW.author_name AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_post_delete AFTER DELETE ON posts
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 550
        - 3 * length(CAST(COALESCE(OLD.body, '') AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER quota_text_comment_insert AFTER INSERT ON comments
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 560
        + 3 * length(CAST(COALESCE(NEW.body, '') AS BLOB)) + length(CAST(NEW.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_comment_update AFTER UPDATE OF body, author_name ON comments
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + 3 * (length(CAST(COALESCE(NEW.body, '') AS BLOB)) - length(CAST(COALESCE(OLD.body, '') AS BLOB)))
        + length(CAST(NEW.author_name AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_comment_delete AFTER DELETE ON comments
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 560
        - 3 * length(CAST(COALESCE(OLD.body, '') AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER quota_text_reaction_insert AFTER INSERT ON reactions
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 450
        + length(CAST(NEW.emoji AS BLOB)) + length(CAST(NEW.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_reaction_update AFTER UPDATE OF emoji, author_name ON reactions
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + length(CAST(NEW.emoji AS BLOB)) - length(CAST(OLD.emoji AS BLOB))
        + length(CAST(NEW.author_name AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_reaction_delete AFTER DELETE ON reactions
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 450
        - length(CAST(OLD.emoji AS BLOB)) - length(CAST(OLD.author_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER quota_text_event_insert AFTER INSERT ON events
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 400
        + length(CAST(NEW.payload AS BLOB)) + length(CAST(NEW.summary AS BLOB)) + length(CAST(NEW.actor_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_event_update AFTER UPDATE OF payload, summary, actor_name ON events
BEGIN
    UPDATE circles SET text_bytes = text_bytes
        + length(CAST(NEW.payload AS BLOB)) - length(CAST(OLD.payload AS BLOB))
        + length(CAST(NEW.summary AS BLOB)) - length(CAST(OLD.summary AS BLOB))
        + length(CAST(NEW.actor_name AS BLOB)) - length(CAST(OLD.actor_name AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_event_delete AFTER DELETE ON events
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 400
        - length(CAST(OLD.payload AS BLOB)) - length(CAST(OLD.summary AS BLOB)) - length(CAST(OLD.actor_name AS BLOB))
    WHERE id = OLD.circle_id;
END;

CREATE TRIGGER quota_text_title_insert AFTER INSERT ON day_titles
BEGIN
    UPDATE circles SET text_bytes = text_bytes + 250 + 3 * length(CAST(NEW.title AS BLOB))
    WHERE id = NEW.circle_id;
END;

CREATE TRIGGER quota_text_title_delete AFTER DELETE ON day_titles
BEGIN
    UPDATE circles SET text_bytes = text_bytes - 250 - 3 * length(CAST(OLD.title AS BLOB))
    WHERE id = OLD.circle_id;
END;
