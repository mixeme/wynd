-- Вышедший с правом читать остаётся в своих группах: круг у него на улочке
-- («читает, не пишет»), и раскладка его — решение владельца 2026-10-16.
-- Из групп, как и закрепление, круг уходит, только когда человек ушёл совсем.
DROP TRIGGER circle_group_leave;

CREATE TRIGGER circle_group_leave AFTER UPDATE OF status ON memberships
WHEN NEW.status = 'gone'
BEGIN
    DELETE FROM circle_group_items WHERE account_id = NEW.account_id AND circle_id = NEW.circle_id;
END;
