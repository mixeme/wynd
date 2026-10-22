-- Письмо администратору о поломках сервера (план 46, A10, ч. 2): когда ушло
-- последнее. Пишем не чаще раза в сутки и только когда что-то не так.
ALTER TABLE instance_settings ADD COLUMN admin_trouble_sent_at TEXT;
