-- 0013: ключ идемпотентности для записей и комментариев.
--
-- Офлайн-очередь клиента повторяет POST, если ответ не дошёл, — а сервер уже
-- мог создать запись. Получался дубль, и снять его мог только автор вручную
-- (CLI-2). Теперь клиент кладёт в тело свой client_id (UUID), а повтор с тем
-- же идентификатором возвращает уже созданную сущность.
--
-- Поле необязательное: старые клиенты и ручные запросы шлют без него, и
-- уникальный индекс их не трогает (NULL в SQLite не равен NULL).
--
-- (В плане 42 эта миграция названа 0011; номера сдвинуты, см. 0010.)

ALTER TABLE posts ADD COLUMN client_id TEXT;
ALTER TABLE comments ADD COLUMN client_id TEXT;

CREATE UNIQUE INDEX idx_posts_client_id
    ON posts (circle_id, client_id) WHERE client_id IS NOT NULL;

CREATE UNIQUE INDEX idx_comments_client_id
    ON comments (circle_id, client_id) WHERE client_id IS NOT NULL;
