-- Почта администратора (2026-10-21): письма о том, что ждёт его решения, —
-- просьбы о месте и заявки об оплате. Пусто — писем нет, только счётчик в
-- шапке панели.
ALTER TABLE instance_settings ADD COLUMN admin_email TEXT NOT NULL DEFAULT '';
