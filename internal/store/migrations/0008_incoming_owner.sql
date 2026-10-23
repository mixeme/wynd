-- Перенос круга, когда почты того, кто его поднимает, среди участников нет
-- (план 46, A12, 7.20): «Аня — это вы?». owner_name — имя владельца из
-- карточки прежнего сервера; reviver_absent — почты поднимающего в круге нет;
-- new_member — ответ «Нет»: имя, пол, приветствие и день (JSON), с ним круг
-- поднимается новым участником. Пусто — «Да, это я» или вопроса не было.
ALTER TABLE incoming_moves ADD COLUMN owner_name TEXT NOT NULL DEFAULT '';
ALTER TABLE incoming_moves ADD COLUMN reviver_absent INTEGER NOT NULL DEFAULT 0;
ALTER TABLE incoming_moves ADD COLUMN new_member TEXT;
