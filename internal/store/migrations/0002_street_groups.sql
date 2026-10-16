-- Группы и закрепления кругов на улочке — у учётки на сервере, а не только
-- на устройстве (план 46, A2): на втором телефоне и после очистки браузера
-- они те же. Каждый сервер знает лишь свои круги; приложение складывает
-- группы всех серверов человека по названию.
--
-- Отдельными таблицами, а не колонками memberships: строки участия целиком
-- уходят в полный архив круга, а это личная раскладка одного человека.

-- Закрепление — пометка у участия в круге, со временем закрепления
-- (закреплённые на улочке — по нему).
CREATE TABLE circle_pins (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    pinned_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

-- Группа — настройка учётки: название и круги этого сервера в ней. Пустая
-- группа здесь не живёт (её держит устройство, пока в неё не положили круг).
CREATE TABLE circle_groups (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (account_id, name)
);

-- Круг — не больше чем в одной группе человека.
CREATE TABLE circle_group_items (
    group_id TEXT NOT NULL REFERENCES circle_groups(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    added_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

CREATE INDEX idx_circle_group_items_group ON circle_group_items(group_id);

-- Опустевшая группа уходит сама — каким бы путём из неё ни ушёл последний
-- круг (убрали, человек вышел, круг удалён).
CREATE TRIGGER circle_group_drop_empty AFTER DELETE ON circle_group_items
WHEN NOT EXISTS (SELECT 1 FROM circle_group_items WHERE group_id = OLD.group_id)
BEGIN
    DELETE FROM circle_groups WHERE id = OLD.group_id;
END;

-- Ушёл из круга — круг уходит из его групп; ушёл совсем — и закрепление.
CREATE TRIGGER circle_group_leave AFTER UPDATE OF status ON memberships
WHEN NEW.status != 'active'
BEGIN
    DELETE FROM circle_group_items WHERE account_id = NEW.account_id AND circle_id = NEW.circle_id;
END;

CREATE TRIGGER circle_pin_gone AFTER UPDATE OF status ON memberships
WHEN NEW.status = 'gone'
BEGIN
    DELETE FROM circle_pins WHERE account_id = NEW.account_id AND circle_id = NEW.circle_id;
END;
