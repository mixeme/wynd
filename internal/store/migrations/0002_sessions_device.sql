-- Схема 2 (план 72, этап 1): сессии устройств.
-- device — короткая метка из User-Agent («Firefox на Windows», «Wynd Android»);
-- сырую строку UA не храним. last_seen_at — последняя активность для
-- скользящего срока: активность продлевает expires_at (90 дней без
-- деятельности). Пустые значения — сессии, созданные до этой схемы.
ALTER TABLE sessions ADD COLUMN device TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN last_seen_at TEXT NOT NULL DEFAULT '';
-- Сессии, созданные до этой схемы: активность неизвестна — считаем ею вход,
-- иначе список показывал «был(а) 1 января» нулевого года.
UPDATE sessions SET last_seen_at = created_at WHERE last_seen_at = '';
