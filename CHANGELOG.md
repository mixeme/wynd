# Изменения

Формат — [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/).
Версионирование — [SemVer](https://semver.org/lang/ru/). Текущая версия: **0.3.5**
(файл `VERSION` в корне репозитория).

## [Unreleased]

## [0.3.5] — 2026-09-10

Макеты подписки и сбора 10.1–10.12, закрытие очереди code-review-followup.

Закрыта очередь сопровождения после рецензии. Решения — в справочниках;
хвост сторожа — [`ui-guard-row2.plan.md`](docs/plans/ui-guard-row2.plan.md).

### Добавлено

- Тесты: `writeDomainError` для `mail`/`push` `ErrNotConfigured` → 400;
  `WaitNotify` по контексту; SMTP-test без SMTP — 400, не 500.
  `svelte-check` — клики в тестах реакций; `ui-guard.mjs` не тянется в
  JS-check; `matchMedia` в vitest setup живёт между сюитами.

### Изменено

- Админ SMTP/push test — через `writeError` / `writeDomainError`, без
  дублирующей карты.
- Справочники: notify shutdown, `ErrNotConfigured`, `sanitizeFilename` по
  рунам, `TextArea` `compose`/`comment`, `OverlayLayout.ondismiss`,
  CommentBar `disabled`.
- Лента: ширина знака — `--mark-w`, не `46px`.
- Макеты: «+» реакций и фото — ink, не faint; подсказки `.hint` — muted,
  чтобы не сливались с бумагой.
- Макеты **10.1–10.12**: карта, «Оплата» как пункт навбара с переключателем,
  «Куда помочь», «Продлить» с улочки; реквизиты — когда круги закрыты,
  когда человек продлевает и когда открывает баннер.

### Удалено

- План `code-review-followup.plan.md` (очередь закрыта; решения в
  справочниках).

## [0.3.4] — 2026-09-10

Цельный образ продукта в `wynd.html`, макеты 2.7–10.8, вход «Войти»,
сопровождение после code review.

### Добавлено

- Макеты **10.1–10.8** в `docs/visual/screens.html`: подписка и донаты
  (не в 0.x) — заглушка доступа, заявка со скрином, баннер на улочке,
  очередь и настройки в панели.
- Макеты **6.16** и **6.17** в `docs/visual/screens.html`: запрос расширения
  квоты (необязательный шаг с 6.8) и замершая отсечка после первого скачивания
  — срок ещё двигается, другой диапазон только новым циклом.
- Макет **2.8 «Закрепить»** в `docs/visual/screens.html`: удержание строки
  списка, под ней слово, без панели и без булавки.
- Макеты **6.12–6.15** в `docs/visual/screens.html`: что открывается из рамки
  «Необратимо» — передать владение, покинуть круг (два ухода и отказ владельцу).
  Удаление по-прежнему 6.7.
- Макет **2.7 Позвать**: шаг после «Создать и позвать» — QR и ссылка, тихая
  строка в пустой круг. Тот же кадр, что 6.4, не настройки. После создания
  приглашение открывается с `?from=create`: назад и «позову потом» ведут
  в круг.

### Изменено

- **Образ продукта:** дни, журнал, квота, серверы и будущие донаты собраны в
  `docs/wynd.html`. Уточняющие `docs/wynd-*.md` удалены.
- На 2.7 «Сначала в круг, позову потом» — обведённая кнопка, не тихая
  строка (макет и экран после создания).
- Счётчик, лиды секций и футер `docs/visual/screens.html` приведены
  к 85 экранам и к порядку по сценариям; в «Что решилось» — реквизиты,
  новый цикл, баннер доната и очередь без лиц.
- Галереи в `docs/visual/screens.html` стоят по сценариям, не по номеру
  макета: письмо в полосе у записи, день и подсказка вместе, цикл места
  одним блоком, панель начинается с первого запуска. Якоря `#e…` те же.
- Макеты **6.1** и **6.2**: один экран, стык по строкам «Пригласить» →
  «Кто ты в этом круге» → «Уведомления» → «Место», а не разные продолжения
  после чипов приглашений.
- Экран 1.5: название и заголовок «Войти», не «Возвращение» / «Вернуться»
  (макет, карта, smoke, каталог). Внизу — «Регистрация без приглашения»,
  а не «Впервые? Прийти без приглашения» (макет, `/`, smoke).
- Экран 1.5 и присоединение: у поля почты — «Пароля нет», не «Пароля не будет»
  (макет 1.1, 1.5–1.7, живые экраны, smoke); фразы «Wynd не помнит устройств» нет.
- **Сервер:** `notifyCircle` логирует ошибки членства, prefs и push; горутины
  дожидаются при shutdown (`WaitNotify`), таймаут 10 с. `mail`/`push`
  `ErrNotConfigured` → 400, не 500. `writeJSON` и немаппленные domain-ошибки —
  в лог. `sanitizeFilename` обрезает по рунам.
- **GUI:** Circle denied через `PlainLayout`; `CommentBar` на `TextArea`
  variant `comment`; сырые `.row2` → `SettingsRow` (архив, identity, servers,
  deadlines); compose — `IconButton` в правке, без иконки места; админ-хранилище
  на `DataTable`; лента — `OverlayLayout.ondismiss`, токены `--mark-h` /
  `--empty-ink`; smoke `/dev/smoke/e4-1`, `AvatarCrop` в `/dev/ui`.

### Исправлено

- Макеты: текстовые ссылки внутри экрана — одна черта `.under` и цвет
  подсказки, не цвет круга и не терракота страницы документации.
  «поднимите свой» и «исходный код» — `<a class="under">`; «добавить фото»
  с чертой, как «сменить фото». В `ui.css` то же для живых `<a class="under">`.
  `button.under` снова рисует черту: сброс `border: none` её съедал, и
  TextButton рядом с `<a class="under">` выглядел иначе.
- В «Что решилось при прорисовке» заголовок «двадцать шесть» совпадает
  с карточками (в том числе «Три строки рамки»).
- В «Что не нарисовано» добавлен системный шаринг: заголовок «двенадцать»
  совпадает со списком.

## [0.3.3] — 2026-09-10

Сверка CHANGELOG с git-историей с момента создания репозитория. Даты
релизов и покрытие коммитов — без расхождений.

## [0.3.2] — 2026-09-10

Сверка CHANGELOG с git-историей.

### Исправлено

- В [0.3.0] восстановлена запись про синхронизацию `web/package.json` `version` с
  [`VERSION`](VERSION) и сторож `TestMatchesVERSIONFile` — потеряна при закрытии
  [Unreleased] в [0.3.0].

## [0.3.1] — 2026-09-09

Лента через `$ui`, guard `check:ui`, исправления CSS сбросов кнопок и FAB «+».

### Добавлено

- План цельного образа продукта: [product-image.plan.md](docs/plans/product-image.plan.md).
  Пять уточняющих спек влить в `docs/wynd.html` и удалить; развилки закрыты.
- Тест отказа в открытии `wynd.db` со схемой версии > 1.
- **Лента:** `ReactionBar`, `CommentPreview`, `ReactionListRow` в `$ui/data`; план
  [feed-reactions-ui.plan.md](docs/plans/feed-reactions-ui.plan.md) закрыт.
- **`check:ui`:** сверка `button.X` ↔ `.X` в `ui.css`, запрет сырого
  `<button class="row2|one|cm|…">` на prod-маршрутах; vitest `ui-guard.test.ts`.

### Изменено

- Удалён выполненный план снятия legacy; решения в `legacy-inventory.md` и followup.
- Лента и обсуждение: реакции и превью комментария — через `$ui`, не сырой markup
  на маршруте.
- Админка: кнопки «Скопировать» / «Отправить» — `TextButton variant="adminBox"` (`.inp`).

### Исправлено

- FAB «+»: иконка наследует `currentColor` кнопки, а не `--muted` — плюс снова контрастный на тёмном фоне.
- `present.test.ts`: фикстуры с обязательными полями `Reaction` / `Comment` / `FeedPost`
  (`npm run check`).
- Кнопки `$ui/forms/Button`: общий сброс `button.btn` перебивал `.btn`, из‑за чего
  пропадало оформление (bootstrap, login и др.).
- Клиент: после пересоздания сервера (новый `wynd.db`) не пускал по устаревшему
  токену из IndexedDB — при старте проверка сессии на сервере, сброс токена,
  курсора sync и кэша snapshots при 401/403; то же для админ-сессии.
- Чипы `$ui/forms/Chip`: общий сброс `button.chip` перебивал `.chip`, из‑за чего
  пропадали отступы и фон (админка «Квота круга по умолчанию», настройки круга и др.).
- **Строки и кнопки на `<button>`:** сброс `button.row2` / `button.r` и др. обнулял
  padding и border — пропадали отступы `SettingsRow`, рамки вложений, чипы реакций;
  у layout-классов в `ui.css` явные правила `button.*`.

## [0.3.0] — 2026-09-09

Снятие legacy: одна SQL-схема, `ttl_days`, redirect `/admin/accounts`.

### Добавлено

- **Baseline SQL:** `internal/store/migrations/0001_schema.sql` — полная схема v1
  (nullable `identities.account_id`, FTS, Wave F). Старый `wynd.db` с версией > 1 —
  ошибка «удалите wynd.db».

### Изменено

- **Миграции:** цепочка `0001_init`…`0013_wave_f` удалена; `Version()` = 1.
  `migrationNeedsFKOff` / `applyMigrationWithFKOff` сняты.
- **Инвайты:** только `ttl_sec` в API, клиенте, OpenAPI и тестах; `ttl_days` удалён.
- **`web/package.json` `version`:** совпадает с [`VERSION`](VERSION); пакет
  `private`. Сторож — `TestMatchesVERSIONFile`.
- **Админ GUI:** маршрут `/admin/accounts/[id]` удалён (был redirect на people).
- **Время:** `xtime.Parse` в `admin_storage.go` и `cmd/wynd/main.go` вместо прямого
  `RFC3339Nano`.
- **Документация:** `legacy-inventory.md`, `server-reference.md`, `client-reference.md`,
  `code-review-followup.plan.md`.

## [0.2.3] — 2026-09-09

Обсуждение 4.12 (реакции, правка записи), compose 4.7, QA-сборка и исправления
ленты, форм и миграции v12→v13.

### Добавлено

- **Инвентаризация legacy:** `docs/reference/legacy-inventory.md` — SQL-миграции (v1–13),
  совместимость API (`ttl_days`, RFC3339/Nano), незавершённые миграции GUI (`$ui`),
  политики до 1.0 и приоритеты уборки.
- **QA-сборка:** `scripts/pack-qa.bat` — ZIP `dist/wynd-qa-{VERSION}.zip` с `scripts/run.bat`,
  `dist/wynd.exe`, `dev/data/` и `qa-manual.md`; инструкция для тестировщика —
  `docs/testing/qa-manual.md` (запуск, данные, bootstrap, панель, коды, чек-листы).
- **Обсуждение 4.12:** реакции в карточке записи (пикер `.rxpick`, sheet `?reactions=`);
  карандаш в шапке своей записи ведёт на compose 4.7. Макет 4.12, карта переходов и
  `client-reference`.

### Изменено

- **Правка записи:** в шапке обсуждения тот же карандаш, что у комментария
  (`IconButton` `edit`), а не подчёркнутое «править». Макет 4.12.
- **Compose 4.7:** экран правки по макету — `TextArea variant="compose"`, кнопка «+»
  для фото и без подсказки об обложке; `TextArea` — вариант `compose` и `bind:el`.

### Исправлено

- **Реакции в ленте:** клик по «+» и чипам уходил в обсуждение — `PostCard` игнорирует
  вложенные кнопки; сниппет реакций перенесён внутрь карточки; офлайн-реакция видна
  сразу; ошибка API показывается внизу ленты. Сброс стилей `button.one` / `button.add`.
- **Формы в приложении:** длинные экраны на `FormLayout` (настройки круга и др.) не
  прокручивались — контент без `.form-body` обрезался в `.ph.app`. Обёртка в layout,
  стили прокрутки как у `.compose-body`.
- **Пригласить в круг:** кнопки «Поделиться» и «Скопировать» без обратной связи — текст
  «Отправлено»/«Скопировано»; сброс при пересоздании ссылки.
- **Лента круга:** `button.idn` в шапке — системная белая плашка; длинная строка без
  пробелов давала горизонтальный скролл; превью комментария (`button.cm`) не на всю
  ширину карточки. Сброс chrome identity, `overflow-wrap: anywhere`, ширина `.cm`.
- **Шапка круга:** аватар с именем вёл в настройки только на части экранов — `circleId`
  не передавался в `CircleLayout` вне вкладок ленты. Берётся из контекста круга;
  кликабельно только при наличии аватара (дата дня остаётся текстом).
- **Лайтбокс:** фото и видео вылезали за экран — у `.lb .mid` не было `min-height: 0`,
  проценты `max-height` у медиа не срабатывали.
- **Миграция 0013:** `PRAGMA foreign_keys=OFF` в SQL не действует внутри транзакции
  SQLite; обновление v12→v13 падало на `DROP TABLE identities`, если в БД есть
  `memberships`. FK отключаются на соединении до начала транзакции.
- **Подсказка «назвать день»:** карточка в ленте после первой записи за дату
  срабатывала только из строки ввода — не после публикации из compose. Переход с
  `?dayPrompt=` и проверка при загрузке ленты.

## [0.2.2] — 2026-09-08

Сверка CHANGELOG с git-историей с момента создания репозитория.

### Исправлено

- Даты релизов [0.0.15], [0.1.6], [0.1.8] и [0.2.1] — по дате коммита
  версии, а не соседних записей.
- В [0.1.34]–[0.1.35] восстановлены записи, потерянные при закрытии
  [Unreleased] в [0.2.0]: хвосты `cards-crop-tails`, закрытие планов
  `wynd-ui` и `cards-crop-tails`, правки панели после волны F, макет 3.11.

## [0.2.1] — 2026-09-08

Закрыта рецензия кода 0.2 (волны 0–5) и batch `ListAccountCircles`.
Решения — в справочниках; очередь — [`code-review-followup.plan.md`](docs/plans/code-review-followup.plan.md).

### Добавлено

- **Волна 4 (code-review):** тест ZIP при отсутствующем блобе; `present.test.ts` (реакции, preview, divider);
  drain очереди с mock fetch; `takeSSEDataEvents` в sync; таблица инвариантов в README.
- **`ListAccountCircles`:** `snapshot_circles_test.go` — batch unread, курсор, видимость.
- **Wynd UI:** слот `control` в `SettingsRow` — строка с переключателем справа
  (`div.row2` + `Switch`); smoke `/dev/smoke/e6-6`; каталог и `/dev/ui` обновлены.
- **Волна 3 (code-review):** пакеты `internal/xtime` и `internal/uid`.
- **Волна 1 (code-review):** backup через `VACUUM INTO` (WAL-safe); `/ready` с Ping SQLite;
  тесты backup+WAL и истечения upload в ту же секунду.
- **Волна 0 (code-review):** корневой `README.md` — продукт, AGPL, канон исходников
  (GitHub), loopback-прогон, тесты, указатель на `docs/`.
- `web/static/fonts/OFL.txt` (Golos Text) и ссылка в `fonts/README.md`.
- `scripts/test.bat` — `go test ./...` и web `check` + `check:ui` + `test`.
- План очереди [`code-review-followup.plan.md`](docs/plans/code-review-followup.plan.md) —
  notify goroutine, GUI-костыли, сторож `.row2`.

### Изменено

- **`ListAccountCircles`:** unread, курсор и последнее видимое событие — три batch-запроса
  вместо N+1 на каждый круг (`sqlVisibleAtMembership`, `ROW_NUMBER` по событиям).
  Справочник: batch-запросы и пустой `0001_init.sql`.
- Экраны notify круга и «Приложение» — строки уведомлений через `SettingsRow`+`control`,
  сырой `.row2` убран.
- **Волна 3 (code-review):** снимки ленты/сетки — SQL-фильтр спана, batch-загрузка,
  лимит 2000 постов; notify SQL в `auth`, quota в `blob`; поиск без повторного
  `CanReadEvent`, `entry_date` в JOIN; store — модель «один процесс на БД».
- **Волна 2 (code-review):** compose на `FormLayout` (шапка bar/barAction, `SettingsRow` для даты);
  `CircleBar` identity — `<button class="idn">`; лист реакций — `reactionIconName`;
  notify/app и notify круга — baseline, без PUT при первом `ready`; убран скрытый PATCH ochre.
- **Волна 1:** blob timestamps — `RFC3339Nano`; `expireBefore` в рутине для legacy `Z`-суффикса;
  `parseTime` на спанах видимости — ошибка вместо zero time; создание поста с медиа — одна транзакция;
  rollback с логом; SSE-ошибки в лог; архивный reminder не помечается при ошибке почты;
  ZIP архива — ошибка при отсутствующем блобе; `POST .../approve` ставит `quota_custom=1` и абсолютную квоту.
- OpenAPI `info.version` = `0.2.1` (`VERSION`).
- `internal/store/store.go` — комментарий «SQLite only», без PostgreSQL later.
- Спеки: квота не пишется в журнал (`wynd-event-log-immutability.md`);
  пустые учётки — soft-delete рутиной (`wynd-servers-and-registration.md`);
  донаты — «не в 0.x» (`wynd-donations-subscription-spec.md`).
- `client-reference.md` — `/admin/compress`, `bits-ui`/`qrcode`, PhoneFrame/StatusBar,
  поля IDB (`circle_meta`, очередь); CircleBar identity; notify/app через `SettingsRow`+`control`;
  SSE = опрос 2 с.
- `server-reference.md` — `/ready`, VACUUM INTO, потолок снимка 2000, один процесс,
  `POST .../approve` = `quota_custom=1`, UNIQUE+NULL identities.
- `ui-components.md` — identity `CircleBar` = `<button class="idn">`.
- `stack.html` — актуальный стек (SQLite, PWA, runes); PostgreSQL/TWA/chi/stores/группы
  в отклонённых.
- Competitive analysis: сигнальные пуши — VAPID/Web Push, без UnifiedPush как черты Wynd.
- Аудит безопасности: срез 0.1.14, актуальная версия — `VERSION`.

### Исправлено

- Bootstrap URL в лог только пока bootstrap не выполнен.
- `ListAccountCircles`: невалидный `created_at` последнего события — ошибка, не нулевое время.

### Удалено

- План `code-review-tails.plan.md` (`ListAccountCircles` закрыт; очередь —
  [`code-review-followup.plan.md`](docs/plans/code-review-followup.plan.md)).
- Планы `code-review.plan.md`, `settings-row-control.plan.md`, `notify-app-settings-row.plan.md`
  (волны 0–5 и экраны notify/app закрыты; решения в справочниках).

## [0.2.0] — 2026-09-07

Закрыты макеты после волны F и хвосты панели. Решения — в справочниках.

### Добавлено

- **Сторож экранов Wynd UI** в [ui-components.md](docs/reference/ui-components.md):
  `check:ui`, хук `.cursor/hooks/ui-screens.mjs`, правило `.cursor/rules/wynd-ui-screens.mdc`.
  Экран из существующих `$ui` и layout’ов; пробел библиотеки — план `docs/plans/<slug>.plan.md`
  и отдельная задача, не новый файл в том же заходе.
- **9.9:** `joined_at` в `GET /admin/accounts/{id}` — в строке круга «участник · с {дата}».
- **Админ-тесты хвостов F:** default quota не трогает круги; `PUT .../quota` (custom false/true,
  pending → approved); `ttl_sec`; sentinel DELETE → 404; повторная почта после soft-delete;
  identities NULL и посты на месте.

### Изменено

- **Поле фильтра = поиск.** `SearchField` на 9.8 «почта» вместо `Input admin`.
- **9.7:** QR в правой колонке, подпись «та же ссылка кодом».
- **Удаление учётки:** `LeaveInTx` и soft-delete в одной транзакции.
  `cleanEmptyAccounts` — soft-delete, не `DELETE FROM accounts`.
  `circle_count` считает только `active`.
- Планы `screens-after-f` и `wave-f-tails` закрыты и удалены. Решения — в
  [client-reference.md](docs/reference/client-reference.md) и
  [server-reference.md](docs/reference/server-reference.md).

### Исправлено

- **Правка комментария (4.9):** имя и часы остаются, карандаш и корзина прячутся;
  `.ced` без бокового margin от `.fld`.
- **Пикер реакций:** клик по `.rxpick` не открывает обсуждение.
- **Routine:** курсор SELECT закрывается до UPDATE/DELETE — иначе SQLite зависал
  на `cleanEmptyAccounts`. Тест схемы — версия 13.

## [0.1.35] — 2026-09-07

Макеты после волны F: полоса 3.11, нить обсуждения, реакции с пикером.

### Добавлено

- **Макет 3.11** в `docs/visual/screens.html`: черновик письма в полосе ленты;
  шеврон в поле журнала — дверь на 4.1.
- **Полоса 3.11:** шеврон и фото в `CommentBar` при `oncompose`; `.f.ink` и `.send.off`;
  smoke `/dev/ui` и `e3-1` с колбэками полосы.
- **Обсуждение 4.2/4.8–4.9:** нить `.thread`/`.cmt`, `formatClock`, `CircleLayout` без табов;
  превью `.cm` — сосед `PostCard`.
- **Реакции 4.10–4.11:** иконки `laugh`/`surprise`/`anger`; пикер `.rxpick` в ленте;
  чипы по виду; whitelist ключей на сервере; `reactionResponse` с окном правок;
  `DELETE .../reactions` по посту ищет свою реакцию.

### Изменено

- **Комментарий:** тот же `CommentBar` без `oncompose`; Enter — перенос, не отправка.
- **Плюс в ленте:** открывает пикер, не ставит реакцию втихаря; гасится по окну своей
  реакции и в соло-круге.
- [client-reference.md](docs/reference/client-reference.md): полоса, комментарий, реакции.

## [0.1.34] — 2026-09-07

Волна F: панель по макету — квота, люди, soft-delete.

### Добавлено

- **Панель · квота:** `default_circle_quota_bytes`, `quota_custom`; `PUT /admin/storage/default_quota`
  и `PUT /admin/circles/{id}/quota`; чипы умолчания на 9.1; карточка круга на `/admin?circle=`;
  `owner_email` в таблице хранилища.
- **Панель · люди:** `/admin/people`, карточка `GET/DELETE /admin/accounts/{id}`; `last_login_at`;
  soft-delete учётки (409 для владельца круга); redirect `accounts/{id}` → `people/{id}`.
- **Панель · доступ:** QR под ссылкой серверного инвайта; `ttl_sec` (1 ч / 72 ч / неделя).
- **Миграция 0013:** `quota_custom`, `deleted_at`, nullable `identities.account_id`.

### Изменено

- **Панель · нав:** «Люди» вместо SMTP; SMTP — через «Настроить» на проверке (9.10, шеврон назад).
- **Панель · хранилище:** «Дать» в запросе квоты ведёт на карточку круга, не на approve;
  pending закрывается при `PUT` квоты.
- **CheckMediaQuota:** `custom=0` → умолчание инстанса; `custom=1` → своя (`NULL` = без квоты).
- **Доступ 9.7:** колонка учёток убрана — список на `/admin/people`.
- **Волна F закрыта.** Решения — в [server-reference.md](docs/reference/server-reference.md)
  и [client-reference.md](docs/reference/client-reference.md). `docs/roadmap.md` удалён.

### Исправлено

- **Лента и дни:** клик по записи и дню — `onclick` на корне `PostCard` / `DayCard`,
  без лишней обёртки-`div`; реакции и альбом по-прежнему со `stopPropagation`.
- **Кадр (6.5):** pinch — зум вокруг текущей середины двух касаний (и сдвиг жеста);
  колесо — к курсору относительно вьюпорта. Затем clamp.
- Планы `wynd-ui.plan.md` и `cards-crop-tails.plan.md` закрыты и удалены.
  Решения — в [ui-components.md](docs/reference/ui-components.md) и
  [client-reference.md](docs/reference/client-reference.md).
- **Панель:** `GET /admin/accounts/{id}` отдаёт `circles: []`, не `null`; неизвестный
  круг в `PUT .../quota` — 404. «Дать»/«Отказать» — `.btn`. Целые ГБ в таблице 9.1.
  9.8: «вход закрыт» рядом с числом кругов. 9.10: шеврон над «Почта». 9.7: QR в `.qr`.

## [0.1.33] — 2026-09-07

Wynd UI ($ui), обсуждение 4.8–4.11, вложения и compose, roadmap — волна F.

### Добавлено

- **Wynd UI:** алиас импорта `$ui` → `lib/components/`; guard `check:ui` запрещает
  `$lib/components` в `web/src`.
- **Макеты 4.8–4.11** в `docs/visual/screens.html`: написание и правка комментария,
  выбор реакции и своя реакция. Счётчик экранов 63→67. Иконки смеха, удивления,
  злости, карандаша и корзины.

### Изменено

- **Roadmap:** волны A–E и G убраны из очереди (закрыты в main); осталась волна F
  с развилками по квоте, людям и soft-delete учёток.
- **Wynd UI:** официальное имя design system; миграция импортов `$lib/components` →
  `$ui`; обновлены `ui-components.md`, `client-reference.md`, `stack.html`, `roadmap.md`,
  dev-каталог `/dev/ui`.
- **4.2 Обсуждение:** запись остаётся карточкой, реплики — на бумаге; имя и время
  в одной строке, карандаш и корзина в зарезервированной колонке.
- **Лента:** превью комментария в бумажном подвале рамки, черта отделяет его от записи.
- **Реакции:** закрытый список из четырёх значков без подписей раскрывается в карточке,
  не второй панелью.

### Исправлено

- **Вложения:** сохраняется исходное имя файла при загрузке; в записи — имя и размер
  вместо «Вложение» / «скачать»; при скачивании — реальное имя, а не `attachment`.
- **Вложения:** запись только с файлом больше не показывает пустую обложку — обложкой
  считаются только фото и видео.
- **Compose (4.1, 4.7):** экран создания и правки записи — цветная шапка круга,
  текст на всю ширину, строка «Отнести к дате», нижняя панель с вложениями и
  «пишете как …»; миниатюры 88×88.
- **Compose:** шапка — «Отмена» / название / «Опубликовать» по краям (grid вместо
  flex `.cbar .top`); поле текста растёт по содержимому, «+» сразу под текстом.
- **CommentBar на ПК:** на длинной ленте поле ввода было внизу всей страницы, а не
  экрана — приходилось скроллить. `.ph.app` — flex-колонка на вьюпорт; контент в
  `.circle-body`, панель ввода закреплена внизу видимой области.

## [0.1.32] — 2026-09-06

UI круга: аватар, кнопки, поле ввода; сборка — убить Vite, не стартовать старый exe.

### Исправлено

- **Compose:** кнопка «+» у миниатюр — сброс `margin: auto` у `.addph` в flex-ряду.
- **Аватар:** после загрузки фото видно в шапке, ленте и комментариях; `seedMediaUrl` и
  контекст круга (`avatarUrl`).
- **Обсуждение:** «править» / «удалить» — иконки карандаша и корзины.
- **Экран дня:** «Сохранить» и «убрать…» — полноценные кнопки, колонка под полем названия.
- **CommentBar на ПК:** клик по пустому полю только фокусирует, без перехода в compose.
- **`build.bat`:** перед сборкой останавливает процесс на порту 5173 (Vite dev).
- **`run.bat`:** при устаревшем бинарнике всегда пересборка; без запуска старого exe.

## [0.1.31] — 2026-09-06

Иконка настроек — шестерёнка, а не солнце.

### Исправлено

- **`i-gear`:** контур с зубцами и отверстием вместо круга с лучами; тот же символ в
  `docs/visual/screens.html`.

## [0.1.30] — 2026-09-06

Подсказка на экране дня — в тоне «День общий», без жаргона last-write-wins.

### Изменено

- **Экран дня (5.2):** подсказка про название и обложку — «День общий… останется последнее»
  вместо «кто написал позже — тот и прав»; тот же текст в `docs/visual/screens.html`.

## [0.1.29] — 2026-09-06

Макет SMTP в панели: не пункт навбара, а экран из проверки.

### Добавлено

- **Макет 9.10 «Почта»** в `docs/visual/screens.html`: форма релея, шеврон назад на проверку.
  Счётчик экранов 62→63. На 9.5 у строки SMTP — «Настроить»; smoke e9-4 совпадает.

### Изменено

- Волна F в [roadmap.md](docs/roadmap.md): `/admin/smtp` по 9.10, назад на `/admin/check`.

## [0.1.28] — 2026-09-06

Десктоп: видимая колонка, встраивание фронта и свежая сборка при локальном запуске.

### Изменено

- **Веб на ПК:** на широком экране фон по краям колонки (`--app-gutter`) — сужение 480px
  видно без сравнения с телефоном.

### Исправлено

- **`embed.go`:** `//go:embed all:dist` — чанки Vite с именами на `_` (например `_-82-yAF.js`)
  попадают в бинарник; без этого после сборки — пустая страница.
- **`serve.go`:** `Cache-Control: no-cache` для `index.html`, `sw.js`, manifest и `/_app/*` —
  локальные пересборки видны без очистки данных сайта.
- **`app.html` / `+layout.svelte`:** на localhost service worker снимается и PWA не регистрируется.
- **`vite.config.ts`:** `spa.fallbackRevision` вместо `spa: true` — сборка без ENOENT
  `version.json`.
- **`run.bat`:** если пересборка упала, а `dist` новее бинарника — выход с ошибкой, а не старт
  устаревшего `wynd.exe`.

## [0.1.27] — 2026-09-06

Windows: `run.bat` — первый запуск без каталога данных.

### Исправлено

- **`run.bat`:** первый запуск определяется до старта `wynd.exe` (флаг `FIRST_RUN`), а не по
  наличию `wynd.db` после подъёма сервера — иначе при отсутствии `dev/data` bootstrap в браузере
  не открывался.

## [0.1.26] — 2026-09-06

Windows: `run.bat` — запуск при Vite dev и повторный bootstrap.

### Исправлено

- **`run.bat`:** при устаревшем `dist/wynd.exe` и запущенном Vite (`npm run dev`) сборка
  больше не блокирует старт — выводится предупреждение и поднимается существующий бинарник.
- **`run.bat`:** мастер bootstrap в браузере открывается только при первом запуске (ещё нет
  `wynd.db`); если bootstrap не завершён, при следующих запусках открывается главная, URL — в консоли.

## [0.1.25] — 2026-09-06

Ограничение ширины интерфейса на десктопе.

### Изменено

- **Веб на ПК:** production-интерфейс ограничен колонкой 480px по центру
  (`--app-max-width`); admin — до 820px. Sheet и scrim привязаны к ширине колонки,
  а не ко всему вьюпорту.

## [0.1.24] — 2026-09-06

Приёмка хвостов кадра аватара и UI-kit: правки, решения в справочник, планы закрыты.

### Исправлено

- **Кадр (6.5):** «Готово» не ждало upload — страница отдавала `ondone` через `void`, оверлей
  снимал `loading` сразу после кадра. Теперь `await` до конца `uploadBlob` + `PUT /identity`.
- **Кадр:** Escape и крестик во время upload больше не закрывают оверлей; Tab не уходит на
  поля под кадром (ловушка на `window`, в том числе пока идёт сохранение).
- **Вступление (1.3):** hint «фото не загрузилось» писался в `error` ленты и сразу стирался
  `loadFeedData`, а при непустой ленте не показывался. Отдельный hint над записями.
- **Доступ (9.7):** URL инвайта был `.fld.inp`; статика админки — `FieldDisplay admin` (`.inp`).
- **`AdminNav`:** у кнопок сброшен браузерный padding, как у прежних `span`.

### Добавлено

- **План оставшихся хвостов.** [docs/plans/cards-crop-tails.plan.md](docs/plans/cards-crop-tails.plan.md):
  клик по карточкам ленты/дней без обёртки-`div`; pinch и колесо к якорю жеста.

### Изменено

- Решения хвостов кадра (оверлей до конца upload, hint после join, clamp, фокус) — в
  [client-reference.md](docs/reference/client-reference.md); UI (`AdminNav` = `<button>`,
  `FieldDisplay admin`) — в [ui-components.md](docs/reference/ui-components.md). Планы
  `avatar-crop-tails.plan.md` и `unified-ui-kit-tails.plan.md` удалены.

## [0.1.23] — 2026-09-06

Хвосты кадра аватара и единого UI-kit.

### Исправлено

- **Кадр аватара (6.5):** оверлей остаётся открытым до конца upload; «Готово» в
  `loading`, ошибка сети показывается на кадре.
- **Вступление (1.3):** если join прошёл, а фото не загрузилось — hint в ленте
  «поставьте в профиле», вход не блокируется.
- **`AvatarCrop`:** при повороте/ресайзе — `clampCropTransform` без сброса в
  центр; Tab циклически по крестику и «Готово», Escape как раньше.
- **`AdminNav`:** пункты с `links` — `<button>`, не `span` + `role="button"`.
- **Smoke e1-5, e2-3, e9-4:** мёртвые `.under` / `.act` / `.inp` заменены на
  `TextButton` / `FieldDisplay`.
- **Smoke e6-3:** точки меню у `MemberRow` с `menu`.
- **Доступ (9.7):** URL инвайта через `FieldDisplay`, не сырой `span.inp`.

## [0.1.22] — 2026-09-06

Сверка CHANGELOG с git-историей с момента создания репозитория.

### Исправлено

- В [0.1.13]–[0.1.16] восстановлены записи, потерянные при закрытии
  [Unreleased] (фикс `.chip`, макеты панели, перенос справочников и
  `docs/visual/`).
- В [0.1.14] — подсказки клиента для кодов ошибок аудита безопасности.
- В [0.1.13] и [0.1.15] путь `docs/plans/roadmap.md` заменён на
  `docs/roadmap.md`.
- Дата [0.1.16] — 2026-09-05 (коммит волны B).

## [0.1.21] — 2026-09-06

Wynd UI: единый UI-kit (фазы 1–5), исправления интерактива и guard `check:ui`.

### Исправлено

- **Кнопка «Назад» ничего не делала** на экранах с `FormLayout`, где не был
  передан `onback`: новый круг, поиск, код из письма, вход без приглашения,
  приглашение на сервер, вступление в круг.
- **Иконки поиска и настроек в `AppBar` кликались в пустоту**, когда обработчик
  не передан (например, поиск на пустой улице). Теперь кнопки не показываются
  без `onsearch` / `onsettings`.
- **Кнопки без обработчика в общих компонентах** больше не рисуются и не
  кликаются в пустоту: `BackBar`, `CircleBar`, `ArchiveBanner`, `AttachmentRow`,
  `MemberRow` (три точки), `QuotaRequestRow`, `CommentBar` (фото), `Lightbox`
  (закрыть). `CircleLayout` не показывает нижнюю панель без `onCommentCompose` /
  `onCommentSend`.
- **`disabled`/`loading` у `Button` выглядел как живой CTA** (повтор кода, «Получить код»
  без сервера). Теперь `disabled`/`loading` дают класс `.off`, как макет; `.btn.off`
  и `button.btn:disabled` не кликаются.
- **`check:ui` всегда проходил**: скрипт искал `web/scripts/src/routes`. Теперь
  сканирует `web/src/routes` без вызова `rg`.
- **Передача владения:** строка участника была обёрткой-`div` вокруг `MemberRow`.
  Теперь `onclick` на корне, без вложенной кнопки меню.
- **Кадр аватара (6.11) не на весь экран**: корень с `.ph` без `.app` давал рамку
  390×800. Шапка на `role="button"`. Сдвиг сбрасывался при ресайзе вьюпорта.
  Экспорт мог не учесть EXIF превью. Ошибка открытия фото глоталась.

### Добавлено

- **План Wynd UI.** [docs/plans/wynd-ui.plan.md](docs/plans/wynd-ui.plan.md):
  официальное имя design system (компоненты, layouts, CSS), алиас импорта `$ui` →
  `lib/components/`, миграция с `$lib/components`, guard `check:ui`.
- **UI-kit, фаза 1 — примитивы.** `Button` (`<button>`, обязательный `onclick`, `loading`/`disabled`),
  `Chip` (кнопка при `onclick`), `IconButton`, `TextButton` (`link`/`admin`/`bar`/`barAction`);
  `SettingsRow`, `CircleRow`, `ServerRow`, `SearchResultRow` с опциональным `onclick`;
  `AddPhotoButton` на `<button>`. CSS-reset для `button.btn`, `button.chip`, `button.row2` и др.
- **UI-kit, фаза 2 — lib/components.** `BackBar`, `AppBar`, `CircleBar` (назад), `Lightbox`,
  `MemberRow` (меню), `ColorSwatches`, `DangerZone`, `ArchiveBanner`, `AttachmentRow`,
  `QuotaRequestRow` → `TextButton variant="admin"`; без `role="button"` в общих компонентах.
- **UI-kit, фаза 3 — routes.** Prod-экраны (волны A–F) на `Button`, `Chip`, `TextButton`,
  `SettingsRow`, `CircleRow`, `SearchResultRow`, `AddPhotoButton`; compose/cover — `TextButton`
  `bar`/`barAction`; админка — `TextButton variant="admin"`. В routes нет `class="btn"`.
- **UI-kit, фаза 4 — guard и тесты.** `web/scripts/check-ui.mjs` + `npm run check:ui`
  (запрет `role="button"` и сырого `class="btn"` в routes); vitest на `Button`, `Chip`,
  `SettingsRow` (`mount` + `element.click()`). Справочник и каталог обновлены.
- **UI-kit, фаза 5 — формы.** `Label`, `Input` (`admin` → `.inp`), `FieldDisplay`
  (переименование `Field`), `TextArea` (`variant: area|field`), `SearchField` с
  `bind:value`; prod-экраны и dev-каталог на компонентах вместо сырой разметки.
  `check:ui` дополнен запретом `class="lab"`, `<input class="fld">` и `<textarea>`.
- **План хвостов UI-kit** (закрыт в 0.1.24): AdminNav, мёртвые контролы в smoke, URL инвайта
  на 9.7. Остаток — [cards-crop-tails.plan.md](docs/plans/cards-crop-tails.plan.md).
- **План хвостов кадра** (закрыт в 0.1.24): индикатор upload на 6.5, hint при срыве фото
  после join, clamp при повороте, фокус внутри оверлея. Остаток — тот же план.

### Изменено

- `docs/roadmap.md` — волна F по макетам панели после E: квота круга по умолчанию,
  своя квота в строке (9.2), пункт «Люди», soft-delete учётки. Развилки закрыты.
- Решения кадра аватара (6.11) перенесены в
  [client-reference.md](docs/reference/client-reference.md); план `avatar-crop.plan.md`
  удалён.
- Решения единого UI-kit (кнопка = `<button>`, `FieldDisplay`, guard `check:ui`)
  перенесены в [ui-components.md](docs/reference/ui-components.md); план
  `unified-ui-kit.plan.md` удалён.

## [0.1.20] — 2026-09-05

Волна G: кадр фото аватара.

### Добавлено

- **Кадр аватара (G).** Экран 6.11 «Кадр»: оверлей на бумаге с круглым окном, pan/pinch,
  JPEG 512×512 (quality 0.85). Точки входа — «сменить фото» на 6.5 и «добавить фото» на 1.3;
  join-API без изменений, загрузка после входа.
- **`crop.ts` + `AvatarCrop.svelte`** — клиентское кадрирование без новых npm-пакетов;
  vitest на математику кадра.
- **Макет 6.11** в `docs/visual/screens.html`: экран, подписи 6.5/1.3, счётчик 62, узел на карте.

### Изменено

- `docs/roadmap.md` — волна G отмечена выполненной.

## [0.1.19] — 2026-09-05

Волна E: админка и push.

### Добавлено

- **Учётки (E).** `POST /admin/accounts/{id}/block` и `.../unblock`, колонка `accounts.blocked`;
  карточка `/admin/accounts/{id}` с переключателем входа; строки списка на 9.7 ведут в карточку.
- **Хранилище (E).** `GET /admin/storage` отдаёт `circles[]` (id, name, posts, media_bytes,
  quota_bytes, color); `StackBar` — сегмент на круг.
- **SMTP (E).** Пункт нав на существующие `GET/PUT /admin/smtp` и проверочное письмо.
- **Пуш (E).** `initPush` вызывает `subscribePush` в secure context; публичный
  `vapid_public_key` в `GET /instance`.

### Изменено

- `docs/roadmap.md` — волна E отмечена выполненной.

## [0.1.18] — 2026-09-05

Волна D: настройки круга, квота, архив.

### Добавлено

- **Настройки приглашений (D).** `invite_who`, `invite_kind_default` на круге; чипы на 6.1;
  `ttl_sec` (1 ч / 72 ч / неделя) и QR-код на экране приглашения.
- **Участники (D).** Меню строки: исключить, «может менять настройки»;
  `PUT /members/{account_id}`; список всегда доступен; подпись «это вы».
- **Личность (D).** `PUT /identity` с `avatar_blob_id`; загрузка фото; `Avatar` через blob URL.
- **Квота (D).** `VolumeChart` из `quota.volume`; оценка «~N записей»;
  `median_post_bytes` в API.
- **Архив (D).** Редактирование сроков после старта (`cutoff_locked` блокирует отсечку);
  экран 6.10 с чипами `layout=feed|posts` на скачивании.

### Изменено

- `docs/roadmap.md` — волна D отмечена выполненной.

## [0.1.17] — 2026-09-05

Волна C: вход по приглашению в круг.

### Добавлено

- **API (C).** Публичный `GET /invites/{token}` — превью круга и участников без
  `account_id`; `POST /invites/{token}/join` — завершение вступления с именем и
  опциональной первой записью; `pending_circle_joins` и `pending_circle_id` в verify.
- **1.1 (C).** `InviteCard` с данными peek; на экране только почта (как в макете).
- **1.3 (C).** Вступление: `PeopleStrip`, имя без подстановки, первая запись,
  «Войти в круг».
- **1.4 (C).** `/invite/{token}?members=1` — полный список `MemberRow` без ссылок.

### Изменено

- `docs/roadmap.md` — волна C отмечена выполненной.

## [0.1.16] — 2026-09-05

Волна B: лента, улица, цвет круга на сервере.

### Добавлено

- `docs/reference/`: `server-reference.md`, `client-reference.md`, `ui-components.md` —
  выжимка из закрытых планов.
- `docs/visual/screens.html`: два экрана панели. **9.8 Люди** — список с поиском.
  **9.9 Почта на сервере** — круги без лиц, закрыть вход, удалить в рамке;
  владельца с панели не удалить.
- **Лента (B).** `GET /feed` — `events`, `visible_from`, `circle_started_at`; `EventDivider`
  для служебных событий; маркеры 3.3/3.4; pull-to-refresh с дорисовкой знака; «На карте»
  на обложке с geo; кнопка «Пригласить» в пустом круге.
- **Улица (B).** `GET /circles` — `last_summary`, `last_at`, `color`; превью в `CircleRow`;
  пины (500 мс); без кругов поиск скрыт.
- **Создание и настройки (B).** После создания — инвайт; режим «дневник»; чип «Своё…»;
  `circles.color` на сервере, PATCH из `ColorSwatches`.
- **Поиск (B).** Клик по хиту на `/search` открывает запись.

### Изменено

- Закрытые планы удалены; `docs/plans/` убрана; `roadmap.md` — в корне `docs/`.
- Макеты марки — в `docs/visual/` (`screens`, `logo`, `mark`); `wynd.html` и
  `stack.html` — в корне `docs/`.
- Ссылки обновлены в `stack.html`, `assets/`, `scripts/extract-ui-assets.py` и
  `web/static/fonts/README.md`.
- Соло-круг: реакции и «+» скрыты, комментарии остаются.
- `docs/roadmap.md` — волна B отмечена выполненной.

## [0.1.15] — 2026-09-04

Волна A журнала: поле в ленте, обложки записи, день, подсказка 3.9, правка и видео.

### Добавлено

- **Поле в ленте (A0).** `CommentBar` — настоящий `textarea` и Send на полосе:
  текст публикуется без смены маршрута (`createPost` / `enqueuePost`); пустое
  поле и иконка фото открывают compose с черновиком в `sessionStorage`.
- **Обложка при создании (A1).** Тап по миниатюре photo/video в compose — единственная
  обложка с `outline` цветом круга; то же в `media_meta` очереди.
- **Обложка после публикации (A2).** `PATCH /posts/{id}` принимает опциональный
  `cover_blob_id`; `Chronicle.SetPostCover` и тест инвариантов.
- **День (A3).** Экран дня: inline-название, «сменить» обложку, пикер
  `/days/{date}/cover`; в `GET /days` — `title_editable_until` и
  `cover_editable_until`; записи дня по `COALESCE(captured_at, created_at)`.
- **Подсказка 3.9 (A4).** Карточка после первой записи дня; счётчик и флаг в IDB.
- **Правка и видео (A5).** В ленте у поста и комментария — `edit_window_sec` и
  `editable_until`; compose при правке — DangerZone удаления, строка окна при
  расхождении с кругом; inline-правка своих комментариев; lightbox с
  `<video controls>`.

### Исправлено

- **Ряды чипов всё ещё слипались.** Правило `.chips + .chips { margin-top: 8px }`
  (0.1.12) не помогало: `.chip` был обычным `inline`, вертикальный padding не
  входил в высоту строки и перекрывал зазор. У `.chip` стоит `display:
  inline-block` — padding в коробке, `gap` и отступ между рядами работают
  на всех экранах с чипами.

### Изменено

- `docs/visual/screens.html` (ранее `docs/screens.html`), 9.1: квота круга по
  умолчанию — чипы рядом с потолком инстанса; в таблице «своя» vs умолчание,
  строка с шевроном. **9.2** — тот же экран, открыта «Семья»: владелец почтой,
  поле и чипы своей квоты. Бывшие 9.2–9.6 (сжатие…доступ) стали 9.3–9.7. На 9.7
  «Доступ» без списка людей: кого пускать и кто уже здесь — разные пункты.
- `docs/roadmap.md` — волна A отмечена выполненной; очередь волн A–E по
  жестам из макета (развилки для Composer).

## [0.1.14] — 2026-09-03

Аудит безопасности всего проекта (`docs/security-audit-2026-09-03.md`):
ручной обзор сервера, клиента, деплой-конфигов и зависимостей. Ниже — всё,
что он изменил.

### Исправлено

- **`X-Forwarded-For` принимался от любого клиента**, поэтому лимит «10 кодов
  в час на IP» обходился одним заголовком. Теперь он читается только от
  доверенного прокси (`trusted_proxies` в `config.json` или
  `WYND_TRUSTED_PROXIES`, по умолчанию — только loopback), клиентом считается
  самый правый недоверенный адрес. Добавлен лимит 5 кодов в час на почту.
- **Вход администратора и bootstrap без ограничений** на перебор: 5 неверных
  попыток с адреса за 15 минут блокируют его на 15 минут, глобально — 50.
  Пароль администратора не короче 8 символов (`weak_password`).
- **Гонка в проверке кода** позволяла больше трёх попыток параллельными
  запросами: попытка захватывается одним атомарным `UPDATE` до сравнения.
- **Bootstrap- и reset-токены, хеш кода** сравниваются за постоянное время.
- **Push-подписку можно было перепривязать** на другой аккаунт, зная URL
  endpoint: перепривязка требует те же ключи `p256dh`/`auth`, иначе 403.
- **По умолчанию сервер слушал все интерфейсы** при `public_url` на
  127.0.0.1 — «loopback»-режим был виден из локальной сети. Теперь
  `127.0.0.1:7676`; Docker-образ задаёт `WYND_LISTEN=:7676` сам, compose —
  `WYND_TRUSTED_PROXIES` для сети caddy. Существующие `config.json` с
  `":7676"` не трогаются.
- **В открытом режиме `/auth/register` и `/auth/code` больше не выдают,
  есть ли аккаунт**: оба всегда отвечают «код отправлен» (известному адресу
  уходит код входа, новому — код регистрации). В режиме по инвайтам 404
  на `/auth/code` оставлен намеренно.

### Добавлено

- `POST /api/v1/auth/logout` и `POST /api/v1/admin/logout` — отзыв токена.
  Клиент вызывает logout при удалении учётки на экране «Серверы и учётки».
- Границы запросов: JSON-тело до 1 МиБ на всех маршрутах, кроме загрузки
  кусков файла (её ограничивает админский лимит вложений) — ответ 413
  `payload_too_large`; текст поста до 20 000 символов, комментарий до 2 000,
  имена до 100, заголовок дня до 200 — ответ 400 `too_long`; MIME до 255
  байт. У `http.Server` появились `ReadHeaderTimeout` 10 с и `IdleTimeout`
  2 мин (без `ReadTimeout`, чтобы не рвать загрузки и SSE).
- Security-заголовки на SPA и API: `nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy`, CSP `frame-ancestors 'none'; object-src 'none';
  base-uri 'self'`. HSTS в сниппетах Caddy/nginx/Traefik (в `deploy/` и в
  админке «Проверка»).
- Ежедневная рутина чистит просроченные `sessions`, `pending_codes` и старые
  `code_request_log`.
- Dev-страницы `/dev/*` отвечают 404 вне `vite dev`.
- Тесты на всё перечисленное: `internal/api/security_test.go`,
  `web/serve_test.go`.
- Подсказки на экранах auth для `weak_password`, `too_long` и
  `payload_too_large`.

### Зависимости

- `golang-jwt/jwt/v5` 5.2.1 → 5.2.2 (GO-2025-3553), `golang.org/x/crypto`
  0.55 → 0.56 (GO-2026-6354/6355); `go 1.26`, образ сборки
  `golang:1.26-alpine`.

## [0.1.13] — 2026-09-03

Клиентский слой: находки при живом просмотре круга в браузере — тихая кнопка
публикации, нерабочее меню альбома, оверлеи ниже края экрана на десктопе.

### Исправлено

- **Кнопка «Опубликовать» была неотличима от неактивной.** Она сидела на
  классе `.bar .rt`, который в остальной системе означает приглушённую
  подпись (метку времени и т.п.) — цвет не менялся, была ли запись готова к
  публикации. Добавлено состояние `.rt.on` (акцентный цвет, полужирный),
  кнопка включается только когда есть текст или вложение и не реагирует на
  клик, пока публиковать нечего.
- **Три точки в альбоме ничего не открывали.** Иконка в `Lightbox.svelte`
  была декоративной, без обработчика — как и в макете (`screens.html`,
  экран 4.4), где она тоже ничего не открывает. Раз единственное осмысленное
  действие там — скачать открытую фотографию, три точки заменены на прямую
  кнопку скачивания (новая иконка `i-download`), а не на меню с одним
  пунктом.
- **Листать фотографии в полноэкранном режиме было нечем, кроме мелких
  точек-индикаторов внизу.** Добавлены стрелки клавиатуры (`←`/`→`, `Esc`
  закрывает), свайп на тач-устройствах и клик по краям фото.
- **Список отреагировавших (и любая нижняя панель) на десктопе открывался
  ниже края экрана** — приходилось скроллить страницу, чтобы его увидеть.
  `.ph.app` растёт по высоте контента (лента длиннее вьюпорта), а
  `.sheet`/`.scrim` были `position:absolute` и анкерились на нижний край
  этого выросшего контейнера, а не на видимый экран. В режиме приложения они
  теперь `position:fixed`; фиксированные макеты (`docs/screens.html`,
  `/dev/ui`) не затронуты — правило ограничено `.ph.app`.

### Добавлено

- `docs/roadmap.md` — пробелы «макет обещал — реализация не сделала» и
  открытые задачи админки: фото личности в круге (макет рисует «сменить
  фото», ни бэкенд, ни фронтенд этого не поддерживают), страница управления
  пользователями сервера, вынос остальных настроек сервера из env в UI
  панели.
- Иконка `i-download` (`web/static/icons.svg`, `Icon.svelte`).

### Изменено

- Экран входа: заголовок «Войти» вместо «Вернуться»; плейсхолдер имени
  сервера при создании — «Мой сервер» вместо примера с личным именем.
- `VERSION` и `internal/version` — 0.1.13

## [0.1.12] — 2026-09-02

Клиентский слой: прогон этапов 5–9 в браузере на loopback и починка найденного.

### 2026-09-02 — Клиент: приёмка этапов 5–9

Пройден сценарий из `docs/plans/client.plan.md`: инвайт → круг → запись с фото с диска →
лента у второго участника по SSE → офлайн-черновик → дни, сетка, карта, поиск →
настройки круга → `/admin`. Проверка шла и на `vite dev`, и на вшитом в бинарник
SPA — часть дефектов проявлялась только при одинаковом origin у страницы и API.

### Исправлено

- **Круг не открывался в бою.** `resolveCircleOrigin` возвращает `''` для
  собственного origin (так задано в плане), а `+layout.svelte` проверял результат
  на истинность и уводил на `/circles`. Все экраны круга — лента, дни, сетка,
  карта, поиск, настройки, квота, архив, compose, запись — были недостижимы, если
  SPA отдаётся с того же адреса, что и API. Теперь «не найден» — только `null`;
  по той же причине не работал `refreshMeta`.
- **Карточка записи рисовала только автора.** `{#snippet}` внутри `{#if}` не
  попадает в пропсы компонента (Svelte 5 поднимает только прямых детей тега), так
  что `text`, `media`, `headerRight`, `reactions` и `comments` приходили
  `undefined`: ни текста, ни фото, ни реакций в ленте, в дне и на экране записи.
  Сниппеты вынесены из `<PostCard>` и передаются пропсами.
- **EXIF терял координаты.** `exifr.parse(file, { pick: [...] })` со списком
  `latitude`/`longitude` не возвращает их: это вычисляемые поля, а не теги. Фото
  уходило на сервер без места, и карта круга не могла заполниться. Теперь
  выбираются сегменты `{ exif: true, gps: true }`.
- **Карта оставалась пустой.** `renderMap()` вызывался, пока контейнер ещё не
  отрисован (он в ветке `{:else}` после `loading`), и выходил на первой строке;
  а `leaflet.markerClusterGroup` не существует на ES-namespace — UMD-плагин
  расширяет глобальный `L`, который ставит сам leaflet. Рендер перенесён в
  `$effect`, фабрика кластера ищется и в namespace, и в глобали, ошибки рендера
  больше не молчат.
- **Поиск не искал.** Debounce читал `query` внутри `setTimeout`, поэтому у
  `$effect` не было зависимостей и он не перезапускался при вводе. И в круге, и
  в поиске по всем серверам.
- **Квота показывала «из 0 Б».** Если у круга нет своего потолка, API не
  присылает `quota_bytes`; экран печатал ноль и рисовал полную шкалу. Теперь —
  «ограничение не задано» и пустая шкала.
- **Шрифт был не шрифтом.** `web/static/fonts/GolosText-Variable.woff2` содержал
  HTML-страницу «Page not found» с GitHub (305 КБ), браузер на каждом экране
  ронял `OTS parsing error` и откатывался на системный шрифт. Заменён на
  настоящий Golos Text (SIL OFL), variable, четырьмя файлами по `unicode-range` —
  как его отдаёт апстрим.
- **`go test ./...` падал на `internal/blob`.** Тесты создавали upload-сессию с
  фиксированной датой 2026-08-30, а `loadSession` сверяет `expires_at` с
  настоящими часами: после 31 августа все сессии выглядели просроченными.
- **Даты в поиске по серверам** печатались срезом ISO-строки мимо
  `format/time.ts`.
- `manifest.webmanifest` отдавался как `text/plain`: в таблице MIME Go нет
  `.webmanifest`, и браузер игнорировал манифест.
- Service worker не клал в precache `woff2` — офлайн откатывался на системный
  шрифт. У страницы не было `<title>`: вкладка называлась «untitled page».
- Кнопка зума Leaflet накрывала бейдж карты в том же углу.
- **Пропал отступ между рядами чипов.** Второй `ChipGroup` («Без ограничения», срок
  приглашения и т.п.) прилипал к первому. Отступ восстановлен правилом
  `.chips + .chips { margin-top: 8px }` в дизайн-системе, как в макете.

### Добавлено

- `web/src/lib/media/exif.test.ts` и фикстура `web/src/test/fixtures/exif-gps.jpg`
  — регрессия на потерю GPS (падает на старом коде)
- `.gitattributes` — `core.autocrlf` в репозитории включён, а бинарники без
  явного `binary` он портит на checkout
- Проверка MIME манифеста в `web/serve_test.go`

### Изменено

- `web/src/routes/circles/[id]/+layout.svelte`, `+page.svelte`,
  `days/[date]/+page.svelte`, `posts/[postId]/+page.svelte`, `map/+page.svelte`,
  `search/+page.svelte`, `quota/+page.svelte`
- `web/src/routes/search/+page.svelte`, `web/src/app.html`, `+layout.svelte`
- `web/src/lib/media/exif.ts`, `web/src/lib/styles/map.css`
- `web/vite.config.ts` — `globPatterns` с `woff2`
- `web/serve.go` — `application/manifest+json`
- `internal/blob/blob_test.go` — время сессии от настоящих часов
- `web/static/fonts/` — четыре `woff2` вместо битого файла, README
- `docs/plans/client.plan.md`: этап 9 и критерии готовности отмечены выполненными
- `web/src/lib/styles/ui.css`, `docs/screens.html` — отступ между соседними рядами чипов
- `VERSION` и `internal/version` — 0.1.12

### Известные ограничения

- Регистрация service worker не проходит во встроенном браузере, которым шла
  проверка (сам `sw.js` отдаётся и разбирается); installability PWA план и не
  закрывает на loopback.
- Экран приглашения показывает «Круг» вместо названия: на сервере нет
  `GET /api/v1/invites/{token}` — это вопрос к `server.plan.md`.
- Запись, отправленная при живой сети, но недоступном сервере, не попадает в
  очередь: `compose` смотрит на `navigator.onLine`. Очередь ловит только
  настоящий офлайн.

## [0.1.11] — 2026-09-02

Клиентский слой: этап 9 «Приёмка» — vitest, `npm run check`, сборка SPA.

### 2026-09-02 — Клиент: этап 9 «Приёмка»

Автоматическая приёмка ядра: vitest на `queue`, `session`, `format`, `objectUrl`
с `fake-indexeddb` и jsdom. `npm run check` без ошибок, `vite build` → `web/dist`
для `go:embed`. Ручная проверка на loopback (два браузера, офлайн-черновик,
`/admin` без журнала) — по сценарию в `docs/plans/client.plan.md`.

### Добавлено

- `web/src/lib/format/bytes.test.ts`, `time.test.ts`
- `web/src/lib/queue/queue.test.ts`
- `web/src/lib/session/session.test.ts`
- `web/src/lib/media/objectUrl.test.ts`
- `web/src/test/setup.ts` — `fake-indexeddb`, полифиллы jsdom
- Скрипт `npm run test` (`vitest run`), devDependency `jsdom`

### Изменено

- `web/vite.config.ts` — конфиг vitest
- `web/src/lib/idb/db.ts` — `closeDb()` для изоляции тестов
- `web/src/routes/dev/ui/+page.svelte` — тип `IconName` в каталоге иконок
- `docs/plans/client.plan.md`: vitest, check и build отмечены выполненными
- `VERSION` и `internal/version` — 0.1.11

## [0.1.10] — 2026-09-02

Клиентский слой: этап 8 «PWA» — манифест, service worker, иконки.

### 2026-09-02 — Клиент: этап 8 «PWA»

`@vite-pwa/sveltekit` с `generateSW` и `navigateFallback` для SPA на
`adapter-static`. Манифест Wynd, иконки из `assets/icon.svg` и
`icon-small.svg` (192, 512, favicon). Service worker кэширует только статику;
`/api` в denylist, без runtime caching и без подстановки Authorization.
Регистрация SW в корневом layout для Web Push.

### Добавлено

- `@vite-pwa/sveltekit` в `web/package.json`
- `web/static/icon-192.png`, `icon-512.png`, `favicon-32.png`, `favicon.svg`, `icon.svg`
- PWA-конфиг в `web/vite.config.ts`

### Изменено

- `web/svelte.config.js` — `serviceWorker.register: false` (регистрация через PWA)
- `web/src/routes/+layout.svelte` — манифест, favicon, регистрация SW
- `docs/plans/client.plan.md`: этап 8 отмечен выполненным
- `VERSION` и `internal/version` — 0.1.10

## [0.1.9] — 2026-09-02

Клиентский слой: этап 7 «Оболочка и админка» — настройки приложения, тема, кэш,
панель администратора.

### 2026-09-02 — Клиент: этап 7 «Оболочка и админка»

Настройки приложения: серверы и учётки, уведомления по умолчанию, очистка кэша
медиа, тема (система / светлая / тёмная). Панель администратора на
`admin_session` и AdminWideLayout: хранилище, проверка, сжатие, доступ; вход
по паролю. Web Push — модуль `push.ts`, без secure context и без service worker
не подписывается.

### Добавлено

- `web/src/lib/push/push.ts` — подписка no-op без secure context / SW
- `web/src/lib/admin/admin.ts` — вызовы admin API
- `web/src/lib/settings/notify.ts` — дефолты уведомлений аккаунта
- Маршруты: `/settings`, `/settings/servers`, `/settings/app`
- Маршруты: `/admin/login`, `/admin/compress`, `/admin/access`
- Очистка IDB store `media`

### Изменено

- Тема из сессии ставит `.dark` на боевом `.ph.app`
- `AdminNav` в режиме `app` ведёт на pathname
- `SettingsRow` — опциональная иконка; `QuotaRequestRow` — Дать / Отказать
- `docs/plans/client.plan.md`: этап 7 отмечен выполненным
- `VERSION` и `internal/version` — 0.1.9

## [0.1.8] — 2026-09-02

Клиентский слой: этап 6 «Настройки и квота» — настройки круга, участники,
квота, архивация.

### 2026-09-01 — Клиент: этап 6 «Настройки и квота»

Экраны настроек круга: имя, цвет, окно правок, приглашение, идентичность,
уведомления, участники. Удаление круга — подтверждение вводом названия.
Квота владельца: VolumeChart, отсечка, сроки архивации. Участники скачивают
персональный архив. В CircleBar переход в настройки по тапу на идентичность.

### Добавлено

- `web/src/lib/circles/settings.ts` — API настроек, квоты и архивации
- `internal/chronicle/members.go` — участники, переименование, удаление круга
- `internal/api/circle_settings.go` — REST для настроек круга
- Маршруты: `/circles/[id]/settings/*`, `quota/`, `quota/deadlines/`, `archive/`
- `DangerZone` — опциональный обработчик `onitem`

### Изменено

- `internal/api/archive.go` — `edit_window_sec`, `is_owner`, `can_settings` в деталях круга
- `internal/api/openapi-participant.yaml` — новые participant-эндпоинты
- `CircleBar` — тап по идентичности открывает настройки
- `docs/plans/client.plan.md`: этап 6 отмечен выполненным
- `VERSION` и `internal/version` — 0.1.8

## [0.1.7] — 2026-09-01

Клиентский слой: этап 5 «Виды круга» — дни, сетка, карта и поиск в круге.

### 2026-09-01 — Клиент: этап 5 «Виды круга»

Дни и экран дня читают snapshot; обложка дня — из метаданных или первого
фото. Сетка группирует фото по месяцам, одна плитка на запись. Карта —
Leaflet 1.9, OSM, MarkerCluster, fitBounds / zoom 15. Поиск в круге с
debounce 300 мс и фильтром по автору на клиенте. Табы CircleBar ведут на
pathname.

### Добавлено

- `web/src/lib/journal/days.ts`, `grid.ts`, `map.ts`, `search.ts`, `group.ts`
- `web/src/lib/styles/map.css`
- `leaflet`, `leaflet.markercluster` — карта круга
- Маршруты: `/circles/[id]/days`, `.../days/[date]`, `grid`, `map`, `search`
- `DayCard.coverUrl` — реальная обложка дня
- `formatMonthYear`, `pluralPosts`, `formatDayCardSubtitle` в `format/time.ts`

### Изменено

- `docs/plans/client.plan.md`: этап 5 отмечен выполненным
- `VERSION` и `internal/version` — 0.1.7

## [0.1.6] — 2026-08-31

### 2026-09-01 — Bootstrap-страница и SPA-fallback в бинарнике

Ссылка `/admin/bootstrap?token=…` из лога больше не отдаёт `404 page not
found`: Go отдаёт `index.html` для клиентских маршрутов, добавлен экран
первого запуска. `run.bat` пишет в `dev/data`, перезапускает старый
`wynd.exe`, держит окно открытым вместе с процессом и на первом запуске
открывает bootstrap-URL в браузере.

### Добавлено

- `web/serve.go` — SPA-fallback для вшитого фронта
- `web/src/routes/admin/bootstrap` — имя инстанса и пароль админа
- `completeBootstrap` в `web/src/lib/auth/auth.ts`

### Изменено

- `cmd/wynd/main.go` — `web.SPA` вместо `http.FileServer`
- `scripts/run.bat` — `dev/data`, остановка старого процесса, bootstrap в браузере

### 2026-09-01 — Windows: `run.bat` — запуск из `dist/`

Сценарий поднимает собранный бинарник: при отсутствии или устаревании
`dist/wynd.exe` вызывает `build.bat`, после готовности `/health` открывает
браузер. На первом запуске печатает bootstrap-URL, как `install.sh` после
деплоя.

### Добавлено

- `scripts/run.bat` — локальный запуск из `dist/wynd.exe`
- `scripts/binary-stale.ps1` — проверка, новее ли исходники бинарника

### 2026-09-01 — Локальный каталог данных `dev/data`

При запуске из репозитория без `WYND_DATA_DIR` состояние пишется в
`dev/data`. Весь `dev/` в git и в Docker-контекст не попадает.

### Изменено

- `internal/config` — `DefaultDataDir` = `dev/data`
- `.gitignore`, `.dockerignore` — `/dev/`

### 2026-09-01 — Сборка только с фронтом, без embed-заглушки

`go:embed` больше не обходится пустым `web/dist/index.html`: перед
`go build` нужен `npm run build` в `web/`. Docker и `install.sh
--from-source` собирают SPA в том же пайплайне.

### Изменено

- `web/dist/` целиком в `.gitignore`; `go build` требует готовый фронт
- `deploy/docker/Dockerfile` — stage Node (`vite build`) перед Go
- `deploy/install.sh --from-source` — `npm ci` и `npm run build` перед
  `go build`
- `scripts/build.bat` — ошибка, если Vite dev слушает `:5173`
- `web/vite.config.ts` — `dist/` и `build/` не триггерят reload dev-сервера
- `web/embed.go` — комментарий о порядке сборки

### Удалено

- `web/dist/index.html` — заглушка для `go build` без фронта
- `/wynd` из корневого `.gitignore` (бинарник только в `dist/`)

### 2026-09-01 — Windows: dev.bat, bootstrap и коды входа в файле

`dev.bat` после подъёма стека сам проверяет инстанс: при первом запуске
делает bootstrap, на loopback открывает регистрацию и печатает, куда
смотреть шестизначный код. Коды дублируются в `dev/data/dev-auth-codes.log`,
чтобы не вылавливать их в окне backend.

### Добавлено

- `scripts/dev-first-run.ps1` — bootstrap, режим `open` на loopback,
  подсказки по созданию аккаунта
- `dev/data/dev-auth-codes.log` на loopback — коды входа с меткой времени
- `internal/auth/logcodes_test.go` — запись кодов в файл

### Изменено

- `dev.bat` открывает `/join` и оставляет окно с итоговой сводкой
- `dev-backend.bat` напоминает путь к логу кодов

### 2026-09-01 — Вёрстка полей ввода `.fld`

На экране «Вернуться» поле почты было уже блока «Сервер» и кнопки:
`input` с классом `.fld` не растягивался на всю ширину, в отличие от
`div.fld`.

### Исправлено

- `input.fld` и `textarea.fld` занимают ту же ширину, что и остальные
  поля формы (`ui.css`, `docs/screens.html`)

### 2026-08-31 — Windows: bat-сценарии локального запуска и тестов

Двойной клик поднимает dev-стек или гоняет тесты без ручного ввода команд в
терминале. Общие хелперы вынесены в `_common.bat` (пути, ping портов, ожидание
готовности) — по образцу `open-stubs.bat`.

### Добавлено

- `scripts/dev.bat` — backend (`:7676`) и frontend (`:5173`, proxy `/api`)
- `scripts/dev-backend.bat`, `scripts/dev-frontend.bat` — по отдельности
- `scripts/test.bat` — `go test ./...` и `npm run check`
- `scripts/test-go.bat`, `scripts/test-web.bat` — раздельно
- `scripts/smoke.bat` — `/health`, `/api/v1/instance`, Vite
- `scripts/build.bat` — `vite build` и `wynd.exe`
- `scripts/_common.bat` — общие хелперы для dev/test/smoke

Сверка этапов 0–4 клиентского слоя: очередь, лента, вход.

### 2026-08-30 — Сверка этапов 0–4: журнал, очередь и вход

Этапы отмечены выполненными, но сверка с планом нашла дыры: черта
непрочитанного рисовалась после первого прочитанного поста, очередь
теряла session/blob после 5xx, пины ломались на `https://` origin.

### Исправлено

- Черта «выше — новое» стоит перед первым прочитанным постом, а не после
  него; очередь сверху её не сдвигает.
- Очередь сохраняет `blob_id` после `/complete`; 5xx больше не затирает
  прогресс исходной записью. Drain один на все origin.
- `listPins` режет ключ с конца: origin `https://host` больше не
  превращается в `https`.
- SSE идёт через `apiFetch`; список кругов инвалидируется вместе с
  лентой, refetch улицы — по всем origin сессий.
- 403/404 ленты — «Нет доступа», а не кэш или «не удалось загрузить».
- Черновик открывается из `.q` (`?queue=`), правится и удаляется до
  отправки; онлайн с `?queue=` больше не плодит второй пост.
- Код из письма снова вводится тапом по клеткам; после инвайта имя
  круга пишется в identity, а не локальная часть почты.
- Лайтбокс закрывается крестиком, точки листают; вложения качаются
  через `downloadBlob`, не через URL API.
- Pull-to-refresh — знаком, пустой круг — «Пока ничего»; `/join` на
  invite-only сразу показывает 1.8.

### Изменено

- `AppBar` — поиск и настройки ведут на `/search` и `/settings`.
- `/` собран из `PlainLayout`; `/circles/[id]/join` — из контекста круга.
- `VERSION` и `internal/version` — 0.1.6 (Go оставался на 0.1.0 после
  клиентских 0.1.1–0.1.5).

## [0.1.5] — 2026-08-30

Клиентский слой: этап 4 «Журнал» — лента, запись, медиа, реакции.

### 2026-08-30 — Клиент: этап 4 «Журнал»

Лента круга читает feed snapshot; черта непрочитанного фиксируется при входе,
read_cursor — при уходе. Pull-to-refresh, офлайн-оверлей `.q`, баннер архива.
Compose: сжатие Canvas, EXIF, objectUrl; онлайн POST и офлайн enqueue.
Реакция heart и Sheet `?reactions=`; пост, альбом, lightbox; правка `?post=`.

### Добавлено

- `web/src/lib/journal/` — feed, read-cursor, posts, present, context, types
- `web/src/lib/media/` — objectUrl, compress, exif (`exifr`)
- `web/src/lib/format/` — time, bytes
- `web/src/lib/circles/origin.ts` — origin круга в sessionStorage
- IDB: `getMedia` / `putMedia`; `circle_meta.identity_name`
- Маршруты: лента, compose, пост, альбом; `+layout.svelte` круга

### Изменено

- `CircleBar`, `CommentBar`, `CircleLayout`, `BackBar`, `ArchiveBanner` — onclick/onback
- `circles/+page.svelte`, `circles/new` — запоминают origin при открытии
- `docs/plans/client.plan.md`: этап 4 отмечен выполненным

## [0.1.4] — 2026-08-30

Клиентский слой: этап 3 «Вход и улица» — auth по коду, мульти-origin, экраны 1.* и 2.*.

### 2026-08-30 — Клиент: этап 3 «Вход и улица»

Экраны входа и улицы: инвайт, join, код из письма, вступление в круг.
`GET /instance` перед регистрацией; закрытый сервер → 1.8. Список кругов
склеивает все origin; пины сверху. Создание круга — выбор сервера из sessions.
Глобальный поиск — веер по origin, без фильтра автора.

### Добавлено

- `web/src/lib/auth/auth.ts`, `pending.ts`, `origin.ts` — код, verify, instance
- `web/src/lib/circles/circles.ts`, `meta.ts` — список, создание, поиск
- IDB: `circle_meta` в settings, CRUD `pins`
- Маршруты: `invite`, `join`, `auth/code`, `circles`, `circles/new`, `search`

### Изменено

- `web/src/routes/+page.svelte` — 1.5 возвращение / редирект на улицу
- `docs/plans/client.plan.md`: этап 3 отмечен выполненным

## [0.1.3] — 2026-08-30

Клиентский слой: этап 2 «Очередь» — офлайн-черновики, чанковая загрузка, drain при онлайне.

### 2026-08-30 — Клиент: этап 2 «Очередь»

`queue/queue.ts` — пост, комментарий и реакция в IndexedDB; чанки 1 MiB
(session/chunk/resume/complete), после complete — POST сущности. Drain при
`online` и после enqueue. Черновик правится и удаляется до отправки; 4xx →
state `failed` и код ошибки для Hint. `listQueuedPosts` — оверлей `.q` в ленте.

### Добавлено

- `web/src/lib/queue/queue.ts` — enqueue, drain, upload, `QueuedPostView`
- IDB: типы очереди, `addQueueItem` / `listQueueForCircle` / CRUD

### Изменено

- `web/src/routes/+layout.svelte` — `initQueueDrain` при старте
- `docs/plans/client.plan.md`: этап 2 отмечен выполненным

## [0.1.2] — 2026-08-30

Клиентский слой: этап 1 «API и sync-invalidate» — fetch + Bearer, снимки в IDB, SSE инвалидирует кэш.

### 2026-08-30 — Клиент: этап 1 «API и sync-invalidate»

`api/client.ts` — JSON-ошибки `{ error }`, participant/admin токены из IDB, при 403
снимки круга выбрасываются. Снимки и курсоры в IndexedDB; SSE `/api/v1/sync`
держит один поток на origin, на событие — инвалидация снимков и refetch
зарегистрированной страницы.

### Добавлено

- `web/src/lib/api/client.ts` — `apiFetch`, `apiJson`, `ApiError`
- `web/src/lib/api/snapshots.ts` — кэш снимков, `invalidateCircleSnapshots`
- `web/src/lib/sync/sync.ts` — SSE-поток, `registerRefetch`, `startSyncForAllSessions`
- IDB: `getCursor` / `putCursor`, `getSnapshot` / `putSnapshot`, `invalidateSnapshots`

### Изменено

- `web/src/routes/+layout.svelte` — запуск sync для всех сессий после `initSession`
- `docs/plans/client.plan.md`: этап 1 отмечен выполненным

## [0.1.1] — 2026-08-30

Клиентский слой: этап 0 «Каркас» — боевые маршруты-заглушки, IDB, типы API, `.ph.app`.

### 2026-08-30 — Клиент: этап 0 «Каркас»

Маршруты из `client.plan.md` с верными layout’ами; `/` больше не редиректит на spike.
Ядро: IndexedDB `wynd` v1, сессии на runes, `appinfo` из `VERSION`, OpenAPI-типы
participant/admin. Локальный Golos Text, proxy `/api` → `:7676`, `open-stubs.bat`.

### 2026-08-30 — open-stubs.bat: ожидание Vite на Windows

Скрипт не открывал заглушки или показывал «ошибку соединения»: сломанные
кавычки CMD, цикл ожидания внутри `()`, проверка до готовности HTTP.
Vite по умолчанию слушал только IPv6 (`[::1]:5173`), а health-check шёл на
`127.0.0.1`.

### Добавлено

- `web/src/lib/idb/db.ts`, `session/session.svelte.ts`, `appinfo.ts`
- `web/src/lib/api/schema-participant.d.ts`, `schema-admin.d.ts` (`npm run api:types`)
- Боевые маршруты-заглушки (`/circles`, `/join`, `/admin`, …)
- `scripts/open-stubs.bat` — все заглушки в браузере
- `web/static/fonts/GolosText-Variable.woff2`

### Изменено

- Layout’ы: режим `app` (`.ph.app`, без `PhoneFrame`/`StatusBar` в бою)
- `svelte.config.js`: `strict: false`; SPA без prerender
- `vite.config.ts`: proxy и `__WYND_VERSION__`; `server.host: 127.0.0.1` для dev

### Исправлено

- `scripts/open-stubs.bat`: корректный `cd` в окне Vite, ожидание HTTP 200
  перед открытием вкладок, кавычки вокруг URL с `?`, fallback на `[::1]`
- `openapi-participant.yaml`: `/sync` — один ответ `200` с content negotiation (для `openapi-typescript`)

## [0.1.0] — 2026-08-30

Серверный слой закрыт: сверка этапов 8–9, деплой доходит до процесса, приёмка зелёная.

### 2026-08-30 — Сверка этапов 8–9: деплой и приёмка

Этапы отмечены выполненными, `go test ./...` был зелёный, но выкладка
врала про версию и не ставила прокси на контейнер Wynd. 0.1.0 — слой
по плану закрыт; UI по-прежнему отдельный трек.

### Исправлено

- Версия больше не читается из CWD: установленный бинарник (systemd,
  Docker) отдавал `0.0.0-dev` на `GET /api/v1/instance`.
- Docker Compose: Caddy проксирует на `wynd:7676`, а не на свой
  localhost; порт 7676 не торчит мимо HTTPS.
- `.dockerignore` — в образ не уезжают `node_modules`, `.git` и `data/`.
- Сценарий приёмки проверяет, что вышедший с доступом читает и не пишет.
- `install.sh` предупреждает, если новый `config.json` пишется без
  `--public-url`.

### Изменено

- OpenAPI participant и admin — версия 0.1.0.

## [0.0.36] — 2026-08-30

Серверный слой: этап 9 — приёмка без браузера; слой закрыт.

### 2026-08-30 — Серверный слой: этап 9 «Приёмка»

Журнал, дни, квота и архив прогоняются через httptest на loopback.
OpenAPI совпадает с mux. Бинарник на пустом каталоге печатает bootstrap-URL
и держит данные под одним корнем. UI не нужен.

### Добавлено

- Participant API: `POST /circles`, `POST /circles/{id}/invites`,
  `POST /circles/{id}/leave` — сценарии приёмки идут только по HTTP.
- `internal/api/acceptance_test.go` — три сквозных сценария этапа 9:
  инвайт и отрезок, дни, квота и архив.
- `internal/api/openapi_test.go` — OpenAPI покрывает все маршруты mux.
- `cmd/wynd/main_test.go` — бинарник на пустом `WYND_DATA_DIR`.

### Изменено

- `docs/plans/server.plan.md`: этап 9 и критерии готовности отмечены выполненными.
- OpenAPI participant и admin — версия 0.0.36; маршруты круга, инвайта и ухода.

## [0.0.35] — 2026-08-30

Этап 0 клиента: все заглушки открываются одним сценарием.

### 2026-08-30 — Посмотреть каркас без ручного набора URL

После каркаса маршруты уже стоят, но смотреть их — это печатать каждый
путь. Двойной клик по bat поднимает Vite, если его нет, и открывает
вкладки боевых заглушек. Smoke и spike не трогает: это не приложение.

### Изменено

- `docs/plans/client.plan.md`: этап 0 — `scripts/open-stubs.bat`; плейсхолдеры
  `demo` / `2026-08-30`; query compose и reactions; пауза 200 мс.

## [0.0.34] — 2026-08-30

Библиотека догнала макеты: черта непрочитанного и съехавшие номера.

### 2026-08-30 — Что экран 3.2 потребовал от компонентов

`ui.css` — ручная копия классов из `screens.html`, и после появления
черты непрочитанного она отстала на два правила. Нового компонента
экран не потребовал: черта — тот же `EventDivider`, что и события
хроники, только чернила заменены цветом круга.

### Добавлено

- `web/src/lib/styles/ui.css`: `.ev.new` — линии и подпись цветом круга.
- `web/src/lib/styles/ui.css`: `a.link { text-decoration:none }`.
  Правила для `a` в библиотеке не было вовсе: пока `.link under` жил
  на `<span>`, это не всплывало, но «поднимите свой» — ссылка, и без
  правила браузерное подчёркивание легло бы поверх бордера `.under`.
- `EventDivider` — проп `variant: 'event' | 'unread'`, как `variant`
  у `Button` и `ServerRow`. Черта непрочитанного — не оформление,
  а второй смысл компонента, и через `class` его передавать нечестно.
- Dev-каталог: карточка `#e3-2` с `variant="unread"`.

### Исправлено

- Съехавшие после переномерации 3.2…3.9 → 3.3…3.10 ссылки на экраны:
  `EntryDateMark` e3-8 → e3-9, `ArchiveBanner` и `DangerNote`
  e3-9 → e3-10 — в `catalog.ts`, в dev-каталоге и в `ui-library.plan.md`.

## [0.0.33] — 2026-08-30

Серверный слой: этап 8 — деплой одной командой, systemd, Docker и Caddy.

### 2026-08-30 — Серверный слой: этап 8 «Деплой»

Одна команда ставит бинарник, systemd-сервис и печатает bootstrap-URL.
Docker и Caddy — для выкладки с HTTPS, не для loopback-прогона.

### Добавлено

- `deploy/install.sh` — установка бинарника, пользователя `wynd`, unit-файла;
  `--from-source`, `--bin`, `--public-url`; bootstrap-URL в конце.
- `deploy/systemd/wynd.service` — `WYND_DATA_DIR`, hardening, restart.
- `deploy/docker/Dockerfile` и `deploy/docker/compose.yaml` — образ и стек
  с Caddy.
- `deploy/caddy/Caddyfile` — HTTPS, SSE без буферизации, тело до 100 MB.

### Изменено

- `docs/plans/server.plan.md`: этап 8 отмечен выполненным.

## [0.0.32] — 2026-08-30

Локальный прогон закрывает почту: релей не зависит от хоста.

### 2026-08-30 — Почта в локальном прогоне

Раздел «Локальный прогон» ставил доставку писем рядом с HTTPS и прокси
и оставлял её только на выкладке. Сервер только отправляет через внешний
SMTP-релей; входящая не нужна, домен для From уже есть. С loopback
письмо уходит тем же релеем, что и с VPS.

### Изменено

- `docs/plans/server.plan.md`: почта через релей — в том, что закрывается
  локально; без релея код по-прежнему в лог, это запасной путь, не
  замена доставке. На выкладке остаются прокси и HTTPS, не почта.

## [0.0.31] — 2026-08-30

Доверие серверу сказано через роль администратора, а не через имя.

### 2026-08-30 — Последняя из четырёх редакций

Предупреждение о незашифрованном хранении осталось разным на 1.7:
«записи и фотографии» вместо «данные» и «Слава отвечает за этот
компьютер, и доверять придётся ему, а не программе». Имя хозяина
работает, только пока сервер зовут «У Славы»; на любом другом сервере
фразу пришлось бы собирать заново. Роль администратора работает везде.

### Изменено

- `docs/screens.html`, 1.7: «Сервер хранит записи и фотографии
  незашифрованными. Слава отвечает за этот компьютер, и доверять
  придётся ему, а не программе» → «Сервер хранит данные
  незашифрованными. Присоединение к этому серверу означает, что вы
  доверяете его администратору». Первое предложение теперь общее
  с 1.6, 2.3 и 2.4; второе — своё, потому что сервер здесь уже выбран
  за человека и выбирать ему нечего.

## [0.0.30] — 2026-08-30

Сверка этапов 6–7 серверного слоя: почта, пуши, jobs, admin, квота и архив.

### 2026-08-30 — Сверка этапов 6–7: почта, пуши, квота и архив

Этапы отмечены выполненными, тесты были зелёными, но сверка с планом
нашла дыры: напоминание об архиве уходило сразу, снимок архива брал
комментарии после отсечки, пуш не уходил из журнала.

### Исправлено

- Напоминание цикла считается в Go, а не через SQLite `datetime()`:
  сравнение RFC3339 с `YYYY-MM-DD HH:MM:SS` было всегда истинным
  (пробел меньше `T`), письмо уходило в момент старта.
- Сдвиг deadline снова ставит напоминание за тот же интервал N.
- Снимок архива режет комментарии и реакции по cutoff, а не только посты.
- После первого скачивания другой диапазон — новый цикл, а не отказ.
- Лица из `identity_names` попадают в ZIP и HTML; имена по-прежнему
  не анонимизируются.
- ZIP не создаёт пустую запись `media/`, если файл блоба не открылся.
- SMTP на loopback — как обычно (WARN, если не настроен), а не «не
  применимо». NA остаются HTTPS, прокси, DKIM и «снаружи».
- Сигнальный пуш (круг, тип, число) уходит при записи, комментарии и
  реакции, с учётом `notify_prefs`; упоминания не выключаются.
- Jobs архивации не зависят от 20-часового троттлинга суточной рутины
  и крутятся раз в час, пока сервер жив.

### Изменено

- OpenAPI participant: `notify_prefs`, push, запрос квоты, `layout`
  у скачивания архива.

## [0.0.29] — 2026-08-30

Про незашифрованное хранение сказано одной фразой, а не двумя.

### 2026-08-30 — Одна формулировка на все места

На 1.6 предупреждение было длиннее и мягче, чем на 2.3 и 2.4: «тот, у
кого он в руках, может их прочитать» и «заводитесь у того, кому
доверяете». Смысл тот же, слова разные — читающий макеты подряд видел
две редакции одного решения и искал между ними разницу.

### Изменено

- `docs/screens.html`, 1.6: «Записи и фотографии сервер хранит
  незашифрованными: тот, у кого он в руках, может их прочитать.
  Заводитесь у того, кому доверяете, или поднимите свой» →
  «Сервер хранит данные незашифрованными. Выбирайте сервер, которому
  доверяете, или поднимите свой» — слово в слово как на 2.3 и 2.4.

## [0.0.28] — 2026-08-30

Черта непрочитанного нарисована. «Поднимите свой» ведёт в репозиторий.

### 2026-08-30 — Где человек остановился в прошлый раз

Счётчик в списке кругов обещал непрочитанное, а внутри круга его нечем
было найти: лента открывалась сверху, и где кончается новое — человек
угадывал. Теперь у обещания есть вторая половина.

### Добавлено

- `docs/screens.html`: экран **3.2 «Непрочитанное»** — черта между новым
  и прочитанным. Рисуется цветом круга, тем же, что счётчик в списке:
  одно и то же непрочитанное, показанное дважды. Лента идёт от свежего
  к старому, поэтому новое лежит выше черты, и это сказано словами —
  «выше — новое», — а не стрелкой. Числа рядом нет: сколько именно —
  вопрос списка кругов. Пока экран открыт, черта не двигается:
  прочитанное засчитывается при уходе. События хроники непрочитанным
  не считаются и черту не двигают.
- Модификатор `.ev.new` — тот же разделитель, что у событий хроники,
  но чернила заменены цветом круга.

### Изменено

- `docs/screens.html`: «поднимите свой» на 1.6, 2.3 и 2.4 — ссылка на
  https://github.com/mixeme/wynd. Внутри макета она выглядит так, как
  выглядела бы в продукте: цвет круга и подчёркивание `.link under`,
  без браузерного.
- `docs/screens.html`: экраны круга сдвинуты — 3.2…3.9 стали 3.3…3.10.
- `docs/plans/client.plan.md`: этап 4 — черта непрочитанного по `read_cursors`,
  ссылки на 3.5 / 3.8 / 3.10 вместо старых номеров.
- Счётчики экранов: 57 → 58 в `screens.html` и `ui-library.plan.md`.

## [0.0.27] — 2026-08-30

Серверный слой: этап 7 — квота, цикл архивации и персональный ZIP+HTML.

### 2026-08-30 — Серверный слой: этап 7 «Квота и архив»

Круг, упёршийся в квоту, может освободить место: владелец задаёт
отсечку и дедлайн, участники скачивают персональный архив, по сроку
сказанное и медиа до отсечки удаляются; служебные события остаются.

### Добавлено

- `internal/chronicle/archive.go` — события `cutoff_set`, `cutoff_moved`,
  `deadline_set`, `deadline_moved`; `cutoff_locked_at`, purge по отсечке.
- `internal/chronicle/volume.go` — график объёма медиа по месяцам.
- `internal/archive/` — ZIP с `media/` и HTML (лента или файл на пост),
  инлайн CSS, офлайн.
- Participant API: `GET /circles/{id}`, `GET .../quota`, `POST .../archive`,
  `PUT .../archive/cutoff`, `PUT .../archive/deadline`,
  `GET .../archive/download`; баннер `archive_cycle` в списке кругов.
- `internal/jobs/archive.go` — purge по deadline и напоминания.
- `internal/mail/archive.go` — письма старта цикла и напоминания.
- `internal/store/migrations/0007_archive.sql`.
- Тесты инвариантов этапа 7.

### Изменено

- `cmd/wynd/main.go`, admin routine — archive jobs в суточной рутине.
- `docs/plans/server.plan.md`: этап 7 отмечен выполненным.

## [0.0.26] — 2026-08-30

Серверный слой: этап 6 — почта, пуши, jobs, admin API и проверка инстанса.

### 2026-08-30 — Серверный слой: этап 6 «Почта, пуши, jobs, admin»

Инфраструктура без UI: SMTP-релей, VAPID и сигнальные пуши, суточная
рутина, `wynd backup`, admin API и движок проверки инстанса.

### Добавлено

- `internal/mail/` — SMTP-релей; на loopback без SMTP код входа в лог.
- `internal/push/` — VAPID-ключи при старте, подписки, сигнальный пуш
  (круг, тип, число).
- `notify_prefs` — дефолт аккаунта и оверрайд круга; упоминания всегда
  включены.
- `internal/jobs/` — суточная рутина: осиротевшие блобы, брошенные
  загрузки, пустые учётки, истёкшие инвайты.
- `internal/backup/` и `wynd backup [-incremental] <каталог>` — полный
  бэкап данных с инкрементом блобов.
- `internal/check/` — проверка инстанса; HTTPS/прокси/DKIM/«снаружи» —
  «не применимо» на loopback.
- Admin API: хранилище, квота, сжатие, доступ, учётки, инвайты, SMTP,
  VAPID, проверка, рутина, запросы на расширение квоты.
- `deploy/proxy/` — фрагменты nginx, Caddy и Traefik на порт 7676.
- `internal/api/openapi-admin.yaml` — OpenAPI admin-маршрутов.
- `internal/store/migrations/0006_infra.sql`.

### Изменено

- `cmd/wynd/main.go` — mail/push, рутина при старте, подкоманда `backup`.
- `docs/plans/server.plan.md`: этап 6 отмечен выполненным.

## [0.0.25] — 2026-08-30

Сверка этапов 4–5 серверного слоя: журнал, блобы, sync и поиск.

### 2026-08-30 — Сверка этапов 4–5: журнал, блобы, sync

Этапы отмечены выполненными, тесты были зелёными, но сверка с планом
нашла дыры: запись без текста отклонялась, квота считала блоб дважды,
доступ к файлу не уважал отрезок видимости.

### Исправлено

- Запись только с медиа (без текста) принимается; пустое тело без
  вложений по-прежнему отклоняется.
- При упоре в квоту текст пишется, медиа — нет; уже залитый блоб
  не считается второй раз при привязке к записи.
- Скачивание блоба режет по отрезку видимости поста, а не по
  «сейчас открыт span»: новичок не забирает прошлое, вышедший
  с доступом забирает своё.
- HTML и JS отдаются как файл (`application/octet-stream` +
  `X-Content-Type-Options: nosniff`), как SVG.
- Невалидный JSON журнала — 400, а не 500.
- Поиск FTS5 фильтрует spans в SQL; глобальный поиск находит
  видимое и не отдаёт автора.
- Удаление ветки снимает блоб без других ссылок (тест).
- В снимке ленты у комментария и реакции есть автор.

## [0.0.24] — 2026-08-30

Серверный слой: этап 5 — sync по курсору, снимки ленты, FTS-поиск и read_cursors.

### 2026-08-30 — Серверный слой: этап 5 «Sync и поиск»

Клиент получает события после `seq` только в видимых отрезках, снимки
ленты/сетки/карты/дней уже обрезаны по spans. Поиск FTS5 с тем же фильтром.

### Добавлено

- `GET /api/v1/sync?cursor=` — JSON, SSE (`Accept: text/event-stream`) или
  NDJSON (`Accept: application/x-ndjson`).
- `GET /api/v1/circles` — список кругов с `unread` и `last_read_seq`;
  `PUT /api/v1/circles/{id}/read_cursor`.
- Снимки: `feed`, `grid`, `map`, `days`, `days/{date}`.
- `GET /api/v1/circles/{id}/search` (с автором) и `GET /api/v1/search` (без автора).
- `internal/search/` — FTS5 с фильтром spans.
- `internal/store/migrations/0005_sync_search.sql` — `read_cursors`, `content_fts`.
- `internal/chronicle/sync.go`, `snapshot.go`, `readcursor.go`.
- `internal/api/openapi-participant.yaml` — OpenAPI participant-маршрутов.
- Тесты: sync и поиск не протекают за отрезок видимости.

### Изменено

- `internal/api/server.go` — маршруты этапа 5.
- `docs/plans/server.plan.md`: этап 5 отмечен выполненным.

## [0.0.23] — 2026-08-30

Серверный слой: этап 4 — журнал по HTTP, блобы, квоты и пороги сжатия.

### 2026-08-30 — Серверный слой: этап 4 «Журнал и блобы»

Записи, комментарии, реакции и файлы доступны через REST. Сервер не
сжимает медиа — хранит как есть, пороги отдаёт клиенту.

### Добавлено

- `internal/blob/` — чанковая загрузка (session/chunk/resume/complete),
  отдача с `Content-Disposition: attachment`, SVG как файл, GC при
  удалении ветки, проверка квот инстанса и круга.
- `internal/store/migrations/0004_blobs.sql` — `blobs`, `upload_sessions`,
  `post_media`, `blob_refs`, квота инстанса, пороги сжатия, опциональная
  квота круга.
- `internal/chronicle/media.go` — метаданные медиа (обложка поста, гео,
  `captured_at`, вложение ≠ фото), валидация обложки дня по `post_media`.
- `internal/api/journal.go`, `blobs.go` — маршруты журнала и загрузки;
  `GET /api/v1/instance` отдаёт `compression`.

### Изменено

- `cmd/wynd/main.go` — подключение blob store к API.
- `docs/plans/server.plan.md`: этап 4 отмечен выполненным.

## [0.0.22] — 2026-08-30

Название и обложка дня — сказанное, как запись. Удаление не строка журнала.

### 2026-08-30 — Реальное удаление не оставляет следа в хронике

Журнал — то, что сказано, и структура круга. «Аня удалил запись» и
«название дня удалено» туда не входят: сказанного больше нет.

### Исправлено

- Удаление ветки зачищает текст в снимке и в payload; событие
  `post.deleted` больше не пишется.
- Схлопывание дня снимает проекцию и уносит `day.titled` /
  `day.cover_set` из журнала — без `*.deleted`.
- Комментарий, реакция, название и обложка дня правятся и стираются
  по своему окну, без строки в журнале.
- Удаление записи-обложки при живом дне откатывает обложку к предыдущей.
- Уточнены спеки: журнал разделён на сказанное и структуру;
  источник правды — `docs/wynd-event-log-immutability.md`. Образ продукта,
  экраны, дни, квота и планы приведены к той же модели.

## [0.0.21] — 2026-08-30

Сверка этапов 2–3 серверного слоя: инварианты хроники и потоки auth.

### 2026-08-30 — Сверка этапов 2–3: хроника и auth

Этапы отмечены выполненными, тесты были зелёными, но сверка с планом
нашла дыры: scrub ветки не трогал `post.edited`, владелец мог уйти
с доступом без передачи, инвайт в круг потреблялся до Join.

### Исправлено

- Удаление ветки зачищает payload всех событий поста, комментариев и
  реакций, а не только `event_seq` снимка.
- Схлопывание дня уносит из лога `day.titled` и `day.cover_set` —
  возрождённый день не подхватывает старое название.
- Владелец не может уйти (в том числе с доступом) до передачи круга.
- Смена окна правок — только у активного члена.
- `captured_at` в payload события — RFC3339, а не `sql.NullString`.
- `loadPost` читает NULL-тело после удаления.
- Комментарий и реакция не принимают пост из другого круга.
- Принятие инвайта в круг и Join — в одной транзакции.
- Служебный адрес админа нельзя зарегистрировать как участника.
- Закрытый режим отклоняет и ещё живые инвайты (тест).

### Изменено

- Инварианты хроники и тесты auth/API покрывают найденные случаи.

## [0.0.20] — 2026-08-30

Серверный слой: этап 3 — аутентификация по коду, инвайты, режимы
регистрации и Bearer-сессии.

### 2026-08-30 — Серверный слой: этап 3 «Auth»

Два пути к учётке: приглашение в круг или на сервер и регистрация без
круга в режиме `open`. Паролей у участников нет — вход по 6-значному коду
из письма; на loopback без SMTP код печатается в лог.

### Добавлено

- `internal/auth/` — учётки, инвайты (одноразовые и многоразовые с лимитом),
  коды (15 мин, 3 попытки, лимит по `X-Forwarded-For`), Bearer-сессии
  участников и админа, bootstrap и сброс пароля админа.
- `internal/api/` — `GET /api/v1/instance`, `POST /api/v1/auth/register`,
  `POST /api/v1/auth/code`, `POST /api/v1/auth/verify`,
  `POST /api/v1/invites/{token}/accept`, bootstrap и admin login.
- `internal/store/migrations/0003_auth.sql` — `accounts`, `invites`,
  `instance_settings`, `admin_credentials`, `pending_codes`, `sessions`.
- `internal/config/loopback.go` — определение локального профиля по
  `public_url`.
- Тесты потоков регистрации, инвайта в круг, истечения кода и разделения
  admin/participant сессий.

### Изменено

- `cmd/wynd/main.go` — подключение API auth поверх хроники.
- `docs/plans/server.plan.md`: этап 3 отмечен выполненным.

## [0.0.19] — 2026-08-30

Серверный слой: этап 2 — хроника с отрезками видимости, окном правок,
днями и табличными тестами инвариантов.

### 2026-08-30 — Серверный слой: этап 2 «Хроника»

Ядро продукта: событие и проекция снимка в одной транзакции SQLite.
Без хроники остальные этапы серверного плана бессмысленны.

### Добавлено

- `internal/chronicle/` — круги, идентичности, членства, отрезки видимости,
  посты, комментарии, реакции, дни, окно правок со снимком на публикацию,
  удаление ветки со зачисткой текста, стирание `identity_names` по GDPR.
- `internal/store/migrations/0002_chronicle.sql` — таблицы `events`, `circles`,
  `identities`, `identity_names`, `memberships`, `membership_spans`, `days` и
  снимки контента.
- Табличные тесты инвариантов из `docs/plans/server.plan.md` (новичок, отрезки,
  окно правок, дни, служебные события, GDPR).

### Изменено

- `internal/store/sqlite.go` — `DB()` для доменных пакетов.
- `docs/plans/server.plan.md`: этап 2 отмечен выполненным.

## [0.0.18] — 2026-08-29

Сверка этапов 7–8 UI-библиотеки с макетами: шесть layout’ов и dev-каталог.

### 2026-08-29 — Сверка этапов 7–8 UI-библиотеки с макетами

Smoke `#e1-1`, `#e2-1`, `#e2-3`, `#e3-1`, `#e4-5` и `#e9-1` сняты рядом
с `docs/screens.html`. Каркасы совпали; расхождения были в каталоге
(счётчик включал layout’ы, BackBar стоял на оболочке, Fab подписан как
лента) и в сборке e2-3 без `ServerRow`.

### Исправлено

- `countComponents` не считает layout’ы компонентами — в шапке `/dev/ui`
  снова «58 компонентов и 6 layout-шаблонов».
- Демо BackBar в каталоге — на охра, как `#e2-3`, а не на `.shell`.
- Overlays в каталоге: Fab с `#e2-1`, CommentBar с `#e3-1`, а не одна
  подпись на чужой каркас.
- Smoke e2-3 собирает строку сервера из `ServerRow`, как остальные экраны.

### Изменено

- Демо FormLayout в каталоге берёт цвет из тех же swatch’ей и показывает
  строку сервера.

## [0.0.17] — 2026-08-29

Библиотека UI-компонентов: этап 8 — dev-каталог `/dev/ui` для проверки
библиотеки без сборки 57 экранов; финальная приёмка.

### 2026-08-29 — Библиотека UI-компонентов: этап 8 «Dev-каталог и приёмка»

После этапа 7 компоненты и layout'ы существовали, но сверять их можно было
только по отдельным smoke-экранам. Теперь одна страница `/dev/ui` показывает
все группы, таблицу «компонент → эталонный экран», демо шести layout'ов и
ссылки на smoke-тесты.

### Добавлено

- `web/src/routes/dev/ui/+page.svelte` — dev-каталог: секции Brand, Chrome,
  Forms, Data, Overlays, Admin, Layouts.
- `web/src/routes/dev/ui/catalog.ts` — таблица компонентов, smoke-маршруты,
  критерии приёмки.

### Изменено

- `docs/plans/ui-library.plan.md`: этап 8 и критерии готовности отмечены выполненными.

## [0.0.16] — 2026-08-29

Библиотека UI-компонентов: этап 7 — шесть layout-шаблонов, из которых
собираются экраны; сверка ленты круга целиком из layout + компонентов.

### 2026-08-29 — Библиотека UI-компонентов: этап 7 «Layout-шаблоны»

После этапа 6 экраны собирались из `PhoneFrame` и хрома вручную. Теперь
шесть каркасов в `web/src/lib/layouts/`; сверка идёт с `#e3-1` в
`docs/screens.html`, плюс `#e1-1`, `#e2-1`, `#e2-3`, `#e4-5` и `#e9-1`.

### Добавлено

- `web/src/lib/layouts/` — `PlainLayout`, `ShellLayout`, `CircleLayout`,
  `FormLayout`, `OverlayLayout`, `AdminWideLayout`.

### Изменено

- `web/src/routes/dev/smoke/e1-1`, `e2-1`, `e2-3`, `e3-1`, `e4-5`, `e9-1`
  — переведены на layout’ы.
- `docs/plans/ui-library.plan.md`: этап 7 отмечен выполненным.

## [0.0.15] — 2026-08-29

Сверка этапов 4–6 UI-библиотеки с макетами: подписи дней, панель
администратора и полнота smoke-экранов.

### 2026-08-29 — Сверка этапов 4–6 UI-библиотеки с макетами

Smoke-экраны `#e1-1`–`#e9-4` сняты рядом с `docs/screens.html`. Расхождения
были в данных и в разметке, которая не собиралась: подписи месяцев
не попадали в `DayCard`, у `AdminNav` дублировался `style`.

### Исправлено

- `MonthLabel` принимает содержимое слотом — на e5-1 снова «Август 2026»
  и «Июль 2026».
- `AdminNav` и `StackBar` — один атрибут `style`, без оверлея Vite.
- `SettingsRow` в режиме ссылки — класс `.link` на `.g`, без лишнего
  начертания 600.
- `ColorSwatches` рисует `.sws u`, как в макете; `DayCard` ставит
  компактный счётчик на обложке.

### Изменено

- Smoke e6-2, e6-8, e9-1, e9-4, e4-5 — добраны недостающие строки
  и классы макета.

## [0.0.14] — 2026-08-28

Библиотека UI-компонентов: этап 6 — модальные слои и панель администратора,
smoke-test экранов ленты, лайтбокса, реакций, пушей и проверки инстанса.

### 2026-08-28 — Библиотека UI-компонентов: этап 6 «Overlays и Admin»

После этапа 5 оставались только разметка и CSS-классы из макетов. Теперь
шестнадцать Svelte-компонентов в `overlays/` и `admin/`; сверка идёт с
`#e3-1`, `#e4-4`, `#e4-5`, `#e7-4`, `#e9-1` и `#e9-4` в `docs/screens.html`.

### Добавлено

- `web/src/lib/components/overlays/` — `Fab`, `CommentBar`, `Scrim`, `Sheet`,
  `Dialog`, `PushBanner`, `Lightbox`.
- `web/src/lib/components/admin/` — `AdminNav`, `AdminSection`, `DataTable`,
  `StackBar`, `CheckRow`, `StatusIcon`, `CodeBlock`, `InlineInput`,
  `QuotaRequestRow`.
- `web/src/routes/dev/smoke/e4-4` — фотография во весь экран с Lightbox.
- `web/src/routes/dev/smoke/e4-5` — список реакций со Scrim и Sheet.
- `web/src/routes/dev/smoke/e7-4` — пуш-уведомления с PushBanner.
- `web/src/routes/dev/smoke/e9-4` — проверка инстанса с CheckRow.

### Изменено

- `web/src/lib/components/chrome/AdminBar.svelte` — навигация вынесена в
  `AdminNav`.
- `web/src/routes/dev/smoke/e2-1`, `e3-1`, `e9-1` — переведены на компоненты
  overlays/ и admin/.
- `docs/plans/ui-library.plan.md`: этап 6 отмечен выполненным.

## [0.0.13] — 2026-08-28

Библиотека UI-компонентов: этап 5 — контентные блоки, smoke-test экранов
списка кругов, ленты, поиска, участников и дней.

### 2026-08-28 — Библиотека UI-компонентов: этап 5 «Data»

После этапа 4 оставались только разметка и CSS-классы из макетов. Теперь
двадцать один Svelte-компонент в `data/`; сверка идёт с `#e2-1`, `#e3-1`,
`#e2-5`, `#e6-3` и `#e5-1` в `docs/screens.html`.

### Добавлено

- `web/src/lib/components/data/` — `SectionLabel`, `Avatar`, `EventDivider`,
  `CircleRow`, `PostCard`, `SettingsRow`, `MemberRow`, `SearchGroupHeader`,
  `SearchResultRow`, `ServerRow`, `FoldHeader`, `AttachmentRow`,
  `PhotoPlaceholder`, `PhotoGrid`, `MonthLabel`, `DayCard`, `DayGrid`,
  `DayHeader`, `EntryDateMark`, `ArchiveBanner`.
- `web/src/routes/dev/smoke/e2-5` — поиск по всем кругам с SearchResultRow.
- `web/src/routes/dev/smoke/e5-1` — вкладка «Дни» с DayCard и DayGrid.
- `web/src/routes/dev/smoke/e6-3` — список участников с MemberRow.

### Изменено

- `web/src/routes/dev/smoke/e2-1`, `e3-1` — переведены на компоненты data/.
- `docs/plans/ui-library.plan.md`: этап 5 отмечен выполненным.

## [0.0.12] — 2026-08-28

Библиотека UI-компонентов: этап 4 — формы и действия, smoke-test экранов
приглашения, кода, нового круга, опасной зоны и сроков архивации.

### 2026-08-28 — Библиотека UI-компонентов: этап 4 «Forms»

После этапа 3 оставались только разметка и CSS-классы из макетов. Теперь
восемнадцать Svelte-компонентов в `forms/`; сверка идёт с `#e1-1`, `#e1-2`,
`#e2-3`, `#e6-2`, `#e6-8` и `#e6-9` в `docs/screens.html`.

### Добавлено

- `web/src/lib/components/forms/` — `Field`, `TextArea`, `ScreenTitle`, `Hint`,
  `Button`, `Chip`, `ChipGroup`, `Switch`, `ColorSwatches`, `CodeBox`,
  `InviteCard`, `SearchField`, `DangerZone`, `DangerNote`, `Meter`,
  `PeopleStrip`, `AddPhotoButton`, `VolumeChart`.
- `web/src/routes/dev/smoke/e1-2` — код из письма с CodeBox.
- `web/src/routes/dev/smoke/e6-2` — Meter и DangerZone в настройках круга.
- `web/src/routes/dev/smoke/e6-8` — VolumeChart с линией отсечки.
- `web/src/routes/dev/smoke/e6-9` — DangerNote и ChipGroup на сроках архивации.

### Изменено

- `web/src/lib/theme/colors.ts` — hex-значения и `CIRCLE_COLOR_ORDER` для
  ColorSwatches.
- `web/src/routes/dev/smoke/e1-1`, `e2-3` — переведены на компоненты forms/.
- `docs/plans/ui-library.plan.md`: этап 4 отмечен выполненным.

## [0.0.11] — 2026-08-28

Сверка smoke-экранов с макетами по снимкам: стили иконок доходили
до компонента, таблица панели не собиралась.

### 2026-08-28 — Снимки smoke против screens.html

Шесть экранов сняты рядом с `#e1-1`, `#e1-5`, `#e2-1`, `#e2-3`, `#e3-1`,
`#e9-1`. Шапка круга, список, форма «Новый круг», приглашение и возвращение
совпали. На e3-1 сердце, фото в поле ввода и плюс FAB не брали цвет и размер
из scoped CSS — класс оставался на родителе. e9-1 падал: Svelte 5 не
принимает `<tr>` прямо в `<table>`.

### Исправлено

- `Icon` — prop `style`, чтобы инлайн с макета доходил до `<svg>`.
- Smoke e3-1 / e2-1 и спайк: цвет сердца, фото справа, размер send и plus.
- Smoke e9-1 — `<thead>` / `<tbody>`, страница снова собирается.
- `Icon`, `Logo`, `PhoneFrame` — `$derived` вместо захвата начального prop.

### Изменено

- `Mark` — `display:block`, чтобы знак в AppBar не сжимался по линии текста.

## [0.0.10] — 2026-08-28

Сверка этапов 0–3 библиотеки UI: табы CircleBar совпадают с макетом,
константы вынесены из instance-скрипта, аватар больше не угадывается
из имени.

### 2026-08-28 — Сверка этапов 0–3: CircleBar, AdminBar, PhoneFrame

Этап 3 собрал шапки, но табы круга жили на глобальных `.tab` и
`data-state`, а не на классах из `screens.html`. Стили протекали на всю
страницу и дублировали `ui.css`. Аватар брался из первой буквы identity —
на экране дня и в списке участников это уже не буква имени.

### Исправлено

- `CircleBar` — табы Bits UI рендерятся как `.tabs span.on`, без
  `:global(.tab)`. `ui.css` понимает и `button`, если child snippet нет.
- `CIRCLE_TABS` и `ADMIN_NAV` — `<script module>`: в Svelte 5 с runes
  `export const` из instance-скрипта нельзя.
- Кружок identity только при явном `avatar`. Иначе «12 июля» и «22»
  получали бы ложный `.av`.
- `CircleBar` больше не рисует четыре таба на экранах, где их нет
  (настройки, день). Prop `tabs`, по умолчанию включён; иначе отступ
  12 px, как в макете.

### Изменено

- `PhoneFrame` — prop `height` вместо `:global` на e9-1; класс цвета не
  ставится для terracotta (в макете это значение `.ph` по умолчанию).
- `Icon` — `viewBox="0 0 24 24"` на внешнем svg, чтобы спрайт из файла
  масштабировался так же, как инлайн в `screens.html`.
- `/dev/spike/b` — те же табы на `.on`, без утечки `:global(.tab)`.

## [0.0.9] — 2026-08-28

Библиотека UI-компонентов: этап 3 — каркас экрана (PhoneFrame, шапки),
smoke-test экранов «Список кругов», «Хронология», «Новый круг» и панели
администратора.

### 2026-08-28 — Библиотека UI-компонентов: этап 3 «Chrome»

После этапа 2 оставались только разметка и CSS-классы из макетов. Теперь
оболочка экрана — шесть Svelte-компонентов в `chrome/`; сверка идёт с
`#e2-1`, `#e3-1`, `#e2-3` и `#e9-1` в `docs/screens.html`.

### Добавлено

- `web/src/lib/components/chrome/PhoneFrame.svelte` — `.ph`, props `color`,
  `shell`, `dark`, `wide`.
- `web/src/lib/components/chrome/StatusBar.svelte` — `.sb` + `.sig`.
- `web/src/lib/components/chrome/AppBar.svelte` — знак, поиск, настройки.
- `web/src/lib/components/chrome/CircleBar.svelte` — back, заголовок, identity,
  четыре таба (Хронология · Дни · Сетка · Карта) на Bits UI.
- `web/src/lib/components/chrome/BackBar.svelte` — back, заголовок, опционально
  `right` или поле поиска.
- `web/src/lib/components/chrome/AdminBar.svelte` — знак, навигация панели,
  подпись сервера.
- `web/src/routes/dev/smoke/e3-1` — лента круга с CircleBar.
- `web/src/routes/dev/smoke/e2-3` — форма «Новый круг» с BackBar.
- `web/src/routes/dev/smoke/e9-1` — панель администратора с AdminBar.

### Изменено

- `web/src/routes/dev/smoke/e2-1` — переведён на PhoneFrame, StatusBar, AppBar.
- `docs/plans/ui-library.plan.md`: этап 3 отмечен выполненным.

## [0.0.8] — 2026-08-28

Библиотека UI-компонентов: этап 2 — спрайт иконок, знак и логотип как
переиспользуемые компоненты, smoke-test экранов «Список кругов» и «Возвращение».

### 2026-08-28 — Библиотека UI-компонентов: этап 2 «Иконки и бренд»

После этапа 1 оставались только CSS-классы и сырой спрайт из спайка. Теперь
знак, логотип и иконки — отдельные Svelte-компоненты; сверка идёт с `#e2-1` и
`#e1-5` в `docs/screens.html`.

### Добавлено

- `web/src/lib/components/Mark.svelte` — знак из `assets/mark.svg`, цвет через
  `currentColor`, класс `.mk` по умолчанию.
- `web/src/lib/components/Logo.svelte` — лок из `assets/logo.svg`, высота 48 px
  по умолчанию (пропорция 2.908 : 1).
- `web/src/routes/dev/smoke/e2-1` — список кругов: знак в AppBar, иконки поиска
  и настроек.
- `web/src/routes/dev/smoke/e1-5` — экран «Вернуться»: логотип 48 px по центру.

### Изменено

- `web/src/lib/components/Icon.svelte` — тип `IconName` для 21 иконки спрайта.
- `docs/plans/ui-library.plan.md`: этап 2 отмечен выполненным.

## [0.0.7] — 2026-08-28

Библиотека UI-компонентов: этап 1 — полный CSS телефона из макетов, карта
цветов кругов и smoke-test экрана приглашения.

### 2026-08-28 — Библиотека UI-компонентов: этап 1 «Дизайн-токены и CSS»

После спайка в `ui.css` оставались только классы, нужные e3-1. Теперь перенесены
все семантические классы внутри `.ph` — формы вступления, выбор цвета, панель
администратора, лайтбокс. Имена классов не менялись: сверка идёт один в один с
`docs/screens.html`.

### Добавлено

- `web/src/lib/theme/colors.ts` — `CircleColor` union и `CIRCLE_COLORS`:
  `{ name → css var, tint }` для восьми цветов кругов.
- `web/src/routes/dev/smoke/e1-1` — статичная разметка экрана «Вас пригласили»
  для визуальной сверки с `#e1-1`.

### Изменено

- `web/src/lib/styles/ui.css` — полный блок телефона из `screens.html` (строки
  62–324): `.icard`, `.codebox`, `.sws`, `.admbar`, `.lb` и остальное, чего не
  хватало после спайка.
- `scripts/extract-ui-assets.py` — выгрузка CSS до секции лайтбокса, а не до
  форм вступления.
- `docs/plans/ui-library.plan.md`: этап 1 отмечен выполненным.

## [0.0.6] — 2026-08-28

Документы продукта: тэглайн, решение по UI-библиотеке, дни и квота на макетах,
а следом серверы — выбор при создании круга, учётка без приглашения и окно
правок, которое не переписывает прошлое. Кода в этой версии не менялось.

### 2026-08-28 — Выбор сервера, учётка без приглашения, окно правок только вперёд

Три правки, каждая из которых меняет не картинку, а модель. Круг заводится на
конкретном сервере — значит, сервер стоит первым решением формы, и рядом с ним
сказано, что данные лежат незашифрованными. Учётку можно завести без приглашения
— значит, у сервера появляется режим доступа, и решает его тот, чей сервер. Окно
правок прирастает к записи в момент публикации — значит, круг не переписывает
прошлое ни в одну сторону. Экранов стало 57.

### Добавлено

- `docs/wynd-servers-and-registration.md` — новый уточняющий документ: имя
  инстанса, два пути к учётке, три режима доступа, выбор сервера при создании
  круга и прямая речь о незашифрованном хранении.
- `docs/screens.html`: пять экранов. **1.6 Без приглашения** — единственное
  место в продукте, где вводят адрес. **1.7 Приглашение на сервер** — ссылка и
  QR, не ведущие ни в один круг. **1.8 Сервер не принимает новых** — нарисованный
  тупик. **4.7 Правка записи** — строка с собственным окном записи, появляется
  только при расхождении с окном круга. **9.6 Доступ** — четвёртый раздел панели:
  имя сервера, режим регистрации, серверная ссылка с QR, список учёток.
- `docs/screens.html`: строка «Сервер» на **2.3** и **2.4** — первое решение
  формы, выше названия, с предупреждением о незашифрованном хранении. Текст на
  обоих экранах один и тот же: дневник о сервере ничего своего не говорит.
  «Без приглашения» пунктиром на карте переходов.
- `docs/plans/server.plan.md`: доменные правила про имя инстанса, два пути к учётке и
  режимы `open | invite | closed`; в Этапе 3 — инвайты на сервер, публичный
  `GET /api/v1/instance`, регистрация без круга; в Этапе 2 — снимок окна правок
  у единицы контента; в Этапе 6 — admin API доступа.
- `docs/plans/ui-library.plan.md`: `ServerRow` — строка сервера со значком облака;
  `AdminNav` — четвёртый пункт.

### Изменено

- `docs/screens.html`: у сервера появилось **имя** — «Дом Ани», «У Славы».
  В 1.1, 1.5, 2.2, 6.2, 6.7, 7.1, 7.2 и в шапке панели имя стоит впереди адреса;
  на первом запуске (9.3) оно задаётся в одной карточке с доменом.
- `docs/screens.html`: окно правок — «применяется ко всему сразу, включая
  прошлое» → «подействует только на новые записи» в 2.3 и 6.1, в обзорном списке
  инвариантов и в подписях.
- `docs/screens.html`: из панели администратора убраны обещания «никакого
  доступа к журналам» и «администратор не читает журналы». Панель содержимого
  не показывает, но гарантией это не притворяется.
- `docs/screens.html`: «приглашение и есть регистрация» перестало быть
  инвариантом — в разделе «Вход», в 1.5, 6.4 и 7.2. Приглашение остаётся главным
  путём, но не единственным. На 1.5 внизу — «Прийти без приглашения»: вторая
  дверь для тех, кого никто не звал.
- `docs/wynd-event-log-immutability.md`: раздел о снимке окна правок в момент
  публикации, симметрия в обе стороны, следствия для хранения и интерфейса.
- `docs/plans/server.plan.md`: инвариант «смена окна действует на старые записи» →
  два инварианта о том, что не действует, включая комментарий к старой записи.
- Счётчики экранов: 52 → 57 в `screens.html` и `ui-library.plan.md`.

### Решено

- **Сервер выбирается при создании круга.** Перенос круга на другой сервер
  предусмотрен — идентификаторы под него заложены, экрана и реализации пока нет,
  — но круг живёт там, где заведён, и поэтому строка стоит выше названия.
  Добавления сервера из этой формы нет намеренно: цепочка «создание →
  регистрация → почта → возврат» слишком длинная, чтобы возвращаться в
  недособранную форму.
- **Хранение названо незашифрованным в трёх местах** — где заводят учётку, где
  заводят круг и где администратор смотрит на свой диск. Продукт не обещает за
  хозяина сервера обратного, но и не пугает участника на каждом экране: «тот, у
  кого сервер, прочитает всё» сказано администратору в панели, а человеку,
  заводящему круг, — только что данные незашифрованы.
- **У сервера есть имя.** Адрес набирают руками ровно в одном месте; помнить,
  чей компьютер стоит за доменным именем, человек не обязан.
- **Учётка без круга разрешена, но не всем.** Режим инстанса выбирает
  администратор, по умолчанию — «по приглашению». Закрытый сервер говорит об
  этом прямо: «не получилось» без причины хуже отказа.
- **Окно правок прирастает к записи в момент публикации.** Круг может стать
  строже или мягче — сказанное живёт по тем условиям, при которых было сказано.
  Показывается это один раз, на экране правки, и только при расхождении.
- **Плитка с буквой принадлежит кругу.** Сервер обозначается значком облака и
  плитки не получает нигде.
- **Экраны входа названы ситуацией, а не механикой.** «Без приглашения», а не
  «учётка на сервере»: слово «учётка» остаётся там, где инфраструктура показана
  честно, — в настройках приложения.

### 2026-08-27 — Дни, квота и неправимая хроника — на макетах

Три уточняющих документа описали механику, которой на макетах не было: у записи
три времени, день — общая сущность круга, окно правок не распоряжается служебной
хроникой, а кончившееся место — забота владельца круга, не администратора.
Экранов стало 52. Пятого уровня навигации не появилось.

### Добавлено

- `docs/wynd-days-spec.md`, `docs/wynd-circle-quota-management.md`,
  `docs/wynd-event-log-immutability.md` — уточняющие документы; на них ссылаются
  макеты, план сервера и футер `screens.html`.
- `docs/screens.html`: семь экранов. **3.8 Задним числом** — запись, отнесённая
  к прошлой дате, и приглашение назвать день. **3.9 Круг под архивацией** —
  баннер цикла в ленте. **5.1 Дни** и **5.2 День** — четвёртая вкладка и экран
  дня с обложкой и общим названием. **6.8 Место кончилось** — отсечка по графику
  объёма. **6.9 Сроки архивации** — дедлайн и напоминания.
  **6.10 Архив участника** — своё унести, раскладка на выбор.
- `docs/screens.html`: строка запросов на расширение квоты в **9.1 Хранилище** —
  единственное, что администратор знает о круге, это его размер.
- `docs/plans/server.plan.md`: **Этап 7 «Квота и архив»** — события цикла,
  `cutoff_locked_at`, график объёма, сборка самодостаточного HTML-архива, письма,
  задание на удаление. Прежние этапы 7 и 8 сдвинулись на 8 и 9.
- `docs/plans/ui-library.plan.md`: `DayCard`, `DayGrid`, `DayHeader`, `EntryDateMark`,
  `ArchiveBanner`, `VolumeChart`, `DangerNote`, `QuotaRequestRow`.

### Изменено

- `docs/screens.html`: раздел 5 — **«Три взгляда» → «Четыре взгляда»**; вкладка
  «Дни» добавлена в панель круга на всех экранах, где она есть. Прежние 5.1–5.3
  стали 5.3–5.5.
- `docs/screens.html`: в **4.1 Новая запись** — поле «Отнести к дате» с подсказкой
  из EXIF; в **6.2** и **6.7** — строка «Архив и очистка».
- `docs/plans/server.plan.md`: события хроники разделены на пользовательские и служебные;
  добавлены `entry_date`, `captured_at`, таблица-проекция `days`, события
  `day_titled` и `day_cover_set`, отделяемый профиль под удаление по GDPR.
- `docs/plans/ui-library.plan.md`: 45 экранов → 52, `CircleBar` — четыре таба; эталон
  спайка e3-1 собран с тремя и требует догона.
- `CHANGELOG.md`: убран дубль заголовка `[Unreleased]`.

### Решено

- **Правится только сказанное.** Окно правок распоряжается записями,
  комментариями и реакциями. Вход, уход, роли, квота, отсечка не правятся и не
  удаляются никогда — даже при окне «без ограничения».
- **День общий, а не авторский.** Название и обложку задаёт любой, у кого есть за
  этот день запись; владельца у дня нет и в интерфейсе он не подписан именем.
- **Безымянный день подписан датой**, а не словами «без названия».
- **Дни — четвёртая вкладка, а не пятый уровень.** Навигация остаётся плоской.
- **Отсечку выбирают по графику объёма**, а не наугад: видно, сколько
  освободится.
- **Кончившееся место — дело владельца круга.** Администратор видит размер и
  решает по квоте; что удалить — не его вопрос.

### 2026-08-23 — Библиотека компонентов: решаем спайком

«Взять готовую библиотеку» — не одно решение, а три, и ответы у них противоположные:
поведение и доступность берём готовыми, визуальный слой уже оплачен работой по марке,
прикладные компоненты не продаёт никто. Открытым остался один вопрос — покупать ли
вместе с поведением ещё и Tailwind с пре-стилизованными компонентами. Отвечаем
замером, а не спором.

### Добавлено

- `docs/stack.html`: раздел **«Библиотека UI-компонентов»** — разбор трёх слоёв,
  две ветки (shadcn-svelte против Bits UI со своим CSS) и подраздел
  «Почему Б — не запасной вариант».
- Замеры по `docs/screens.html`: кегли 12.5px (×80), 11.5px (×42), 13.5px (×12),
  17px (×11); половинные значения — в 136 объявлениях `font-size` из ~162; самый
  частый `gap` — 10px; отступы вида `8px 13px`, `4px 18px 14px`. Шкала Tailwind
  кратна 4px, кегли целые — дизайн лежит вне неё.
- `docs/stack.html`: врезка о масштабе выигрыша — время уходит в синхронизацию
  и состояние, а не в компоненты; выбор библиотеки снимает недели отладки
  доступности и больше ничего.

### Изменено

- `docs/plans/ui-library.plan.md`: Этап 0 «Подготовка» → **«Спайк: две ветки на одном
  экране»**. Экран e3-1 собирается дважды, обе версии живут рядом на `/dev/spike/`,
  сравнение по таблице замеров.
- `docs/screens.html` в плане библиотеки — больше не эталон, а визуализация:
  вёрстка пересматривается, информационная архитектура (8 цветов кругов, различие
  «оболочка / круг», устройство ленты) — нет.
- `npm create svelte@latest` → `npx sv create` в обоих документах: официальный
  CLI сменился.

### Решено

- **Поведение и доступность — готовыми при любом исходе.** Focus trap, возврат
  фокуса, Esc, `inert`, позиционирование, клавиатура, ARIA. Своё здесь не экономия,
  а отладка на чужих устройствах.
- **Правило решения спайка:** точность попадания в типографику и стоимость
  overlay-слоя весят больше бандла и строк кода. Первое — про марку, второе —
  про грабли. Если ветка А не выигрывает явно хотя бы по одному из двух, берём Б.
- **Отвергнуты Skeleton, Flowbite-Svelte, daisyUI** — зависимость со своим мнением
  о том, как выглядит продукт; **Carbon, SMUI, Konsta UI** — корпоративный,
  материальный и нативно-iOS облик; **пакеты иконок** — свой спрайт на 22 иконки
  уже есть и меньше по размеру.

### 2026-08-22 — Тэглайн

Вместо «Приватная сеть кругов» — две строки под логотипом: обещание и перевод
имени Wynd.

### Изменено

- `docs/logo.html`: тэглайн — **Наш журнал. Наши воспоминания.** и **Наш тихий переулок.**
  Запасной вариант первой строки: **Журнал для наших воспоминаний.**
- `docs/wynd.html`: та же пара в шапке образа продукта.

## [0.0.5] — 2026-08-26

Спайк UI-библиотеки: экран e3-1 собран дважды; победила ветка Б — Bits UI
и свой CSS на токенах из макетов.

### 2026-08-26 — Библиотека UI-компонентов: этап 0 «Спайк»

До вёрстки ~40 компонентов — замер на самом плотном экране. shadcn-svelte
не попал в типографику без борьбы с Tailwind; Bits UI + `ui.css` совпали с
макетом и дали меньший бандл.

### Добавлено

- `web/` — SvelteKit + TypeScript, `@sveltejs/adapter-static`, `ssr: false`.
- `web/src/lib/styles/tokens.css`, `ui.css` — токены и семантические классы
  из `docs/screens.html`.
- `web/static/icons.svg` (21 иконка) и `Icon.svelte`.
- `web/src/routes/dev/spike/b` — e3-1 (Хронология); табы на Bits UI.
- `web/src/routes/dev/spike` — таблица сравнения веток.
- `scripts/extract-ui-assets.py` — выгрузка спрайта и CSS из `screens.html`.

### Решено

- **Ветка Б — Bits UI + свой CSS.** Типографика 12.5 / 11.5 / 13.5 px и
  `gap: 10px` без округления; CommentBar — разметка `.comp`, не обёртка.
  shadcn-svelte удалён после спайка.
- **Golos Text** на спайке — CDN в `+layout.svelte`; каталог
  `web/static/fonts/` для self-host перед продакшеном.

### Изменено

- `docs/plans/ui-library.plan.md`: этап 0 отмечен выполненным, таблица замеров
  заполнена.
- `docs/stack.html`: итог спайка и выбор ветки Б.

## [0.0.4] — 2026-08-26

SQLite-хранилище: миграции вверх-only и интерфейс под будущий PostgreSQL
без второго драйвера.

### 2026-08-26 — Серверный слой: этап 1 «Store»

Один драйвер — `modernc.org/sqlite`. WAL, `busy_timeout`, `foreign_keys`.
Схема пока пустая: доменные таблицы придут с хроникой.

### Добавлено

- `internal/store` — интерфейс `Store` (`Open`, `OpenMemory`, `Ping`, `Version`,
  `Close`); реализация на SQLite.
- Движок миграций: встроенные SQL, только вверх, таблица `schema_migrations`.
- `internal/store/migrations/0001_init.sql` — пустая схема.
- Тесты на in-memory и на файле: открыть, смигрировать, закрыть повторно без
  повторного применения, проверить pragma.

### Изменено

- `cmd/wynd/main.go` — при старте открывает `wynd.db`, накатывает миграции,
  пишет номер схемы в лог.
- `docs/plans/server.plan.md`: этап 1 отмечен выполненным.

## [0.0.3] — 2026-08-26

Первый исполняемый код: Go-бинарник слушает порт 7676, отдаёт health и заглушку
фронтенда, печатает bootstrap-URL в лог.

### 2026-08-26 — Серверный слой: этап 0 «Каркас»

Минимальный каркас по `docs/plans/server.plan.md`: один бинарник, каталог данных,
конфиг из файла и env, graceful shutdown.

### Добавлено

- `go.mod`, `cmd/wynd/main.go` — HTTP на `:7676`, `GET /health`, graceful shutdown.
- `internal/config` — `config.json` в каталоге данных, переопределение через
  `WYND_DATA_DIR`, `WYND_LISTEN`, `WYND_PUBLIC_URL`; каталоги `blobs/`, `keys/`,
  файл `wynd.db`.
- `web/build/index.html` + `web/embed.go` — заглушка фронтенда через `go:embed`.
- `LICENSE` — AGPL-3.0.
- `.gitignore` — артефакты сборки и локальный каталог `data/`.

### Изменено

- `docs/plans/server.plan.md`: этап 0 отмечен выполненным.

## [0.0.2] — 2026-08-21

Планы двух независимых треков (сервер и UI-библиотека) и экраны проверки инстанса.
Исполняемого кода по-прежнему нет.

### 2026-08-21 — Серверный слой

Часть, которая не привязана к интерфейсу и нужна в любом случае: Go-бинарник,
хроника, отрезки видимости, REST. Экраны и PWA — отдельный трек.

### Добавлено

- `docs/plans/server.plan.md` — план серверного слоя: 9 этапов (0–8), от каркаса
  до приёмки без браузера. У каждой задачи — чекбокс прогресса и рекомендуемая
  модель агента. Ядро — хроника с материализованным снимком и отрезками видимости;
  фронтенд, офлайн и сжатие медиа вне scope.

### 2026-08-21 — Библиотека UI-компонентов

Макеты в `docs/screens.html` остаются эталоном; следующий шаг — перенести
дизайн-систему в Svelte-компоненты `web/src/lib/` по [stack.html](docs/stack.html).

### Добавлено

- `docs/plans/ui-library.plan.md` — план библиотеки: 9 этапов (0–8), ~40 компонентов,
  6 layout-шаблонов, dev-каталог `/dev/ui` для сверки с макетами. У каждой задачи —
  чекбокс прогресса и рекомендуемая модель агента.

### 2026-08-20 — Проверка инстанса

Админу нужен ответ на вопрос «всё ли я настроил правильно». Продукт ломается не сам:
молча ломаются HTTPS, прокси, почта и пуши, а выясняется это через неделю, когда
кто-то не смог войти.

### Добавлено

- `docs/screens.html`: два экрана панели администратора — **проверка инстанса** (9.4)
  и **разбор поломки** (9.5). Экранов стало 45.
- Проверка: домен и HTTPS снаружи, редирект, сертификат и автопродление, четыре
  заголовка прокси, потолок тела запроса, буферизация SSE, таймаут загрузки, SMTP
  и DKIM, VAPID и тестовый пуш, манифест и service worker, часы, место, суточная
  рутина, бэкап.
- Разбор поломки: готовый конфиг прокси (nginx · Caddy · Traefik) с выделенными
  строками, которых не хватает. Обещание образа продукта «при любом сомнении печатает
  готовый конфиг» стало экраном.
- Навигация в шапке панели: Хранилище · Проверка · Сжатие. Первый запуск теперь
  говорит, что после сохранения панель прогонит проверку.

### Решено

- **Проверка «снаружи» — из браузера админа, а не через сторонний пробник.** Панель
  просит браузер сходить на публичный адрес и сравнивает с тем, что видит сервер:
  Wynd никуда не звонит. Если браузер окажется в одной сети с сервером, на экране
  сказано, что проверка вышла изнутри.
- **Статус — формой, а не цветом:** галочка, «!» в кружке, крестик залитым чернилами.
  Красного нет — он занят цветом круга, как и в опасной зоне настроек.
- **Строка объясняет последствие, а не код ошибки.** Не «FAIL: X-Forwarded-For», а
  «три попытки на код считаются всем сразу»; не «413», а «фотография не загрузится»;
  не «DKIM missing», а «письмо с кодом уедет в спам, а пароля у участников нет».
- **Обе поломки прокси показаны одной страницей** — они чинятся одной вставкой.

### Не нарисовано

Отдельного экрана обслуживания и бэкапа в панели нет: суточная рутина и последний
бэкап показаны строками в проверке, а настоящая настройка бэкапа живёт вне продукта.

## [0.0.1] — 2026-08-19

Первая пронумерованная версия: образ продукта, марка, макеты интерфейса,
технический стек. Исполняемого кода пока нет.

### 2026-08-19 — Стек: технический документ

Зафиксирован стек после обсуждения: Go, SQLite (PostgreSQL опционально),
SvelteKit + TypeScript, PWA/TWA, Web Push, монорепо, код на английском.

### Добавлено

- `docs/stack.html` — стек с обоснованиями, структурой монорепо, отвергнутыми
  альтернативами и ограничениями (офлайн, видео в фоне, iOS best effort).

### Решено

- **Пуши в MVP — Web Push (VAPID).** UnifiedPush отложен: сначала проверяем на TWA
  и браузере, de-Googled Android — позже при необходимости.

### 2026-08-19 — Экраны: макеты всего интерфейса

Образ продукта описывал экраны словами и девятью картинками. Теперь весь интерфейс
нарисован разметкой в масштабе 1:1 — от приглашения до панели администратора.

### Добавлено

- `docs/screens.html` — **43 экрана** в рамке `390 × 800`, сгруппированные по уровням:
  вход (5), улочка (6), круг (7), запись (6), три взгляда (3), настройки круга (7),
  приложение (4), тёмная тема (2), панель администратора (3). У каждого — что на нём
  и почему именно так.
- Карта переходов: четыре уровня, ни один экран не спрятан глубже.
- Разделы «Что решилось при прорисовке» и «Что не нарисовано».

Файл самодостаточен: ни сети, ни шрифтов, ни картинок. Знак и лок берутся из
`assets/mark.svg` и `assets/logo.svg` один в один, фотографии заменены заливками —
макет отвечает за композицию, а не за содержимое снимков.

### Решено

- **Знак в шапке — без слова.** В образе продукта в шапке списка стоял лок; гайд
  логотипа, написанный позже, запрещает набирать его ниже 48 px по высоте знака, а в
  мобильной шапке помещается 30. Лок появляется один раз — на экране возвращения.
- **Опасная зона обводится, а не краснеет.** Красной кнопки в продукте быть не может:
  терракота — один из восьми цветов кругов. Необратимое обведено рамкой в чернилах,
  второй барьер — набор имени круга рукой. Открытый вопрос образа продукта закрыт.
- **Шкала — четыре кегля.** Из `17 / 13.5 / 13.5 / 12.5 / 11.5` две ступени совпадали;
  на сорока трёх экранах развести их не понадобилось ни разу — имя автора и текст
  записи различаются весом.
- **Состояния вышедших названы словами:** «читает, не пишет» и «без доступа». Разницы
  между уходом и исключением интерфейс не делает нигде.
- **Тёмная тема** переставляет бумагу и чернила и не трогает цвета кругов. Исключение
  одно: подложка реакции получила свой тёмный оттенок, иначе становилась кнопкой.
- **«Кто уже здесь» стал экраном** — тем, что подразумевало «ещё 17» на вступлении.

### Исправлено

- **Твой отрезок перевёрнут.** Лента идёт от свежего к старому, поэтому выше точки
  обрыва — твоя часть, ниже нет ничего. В образе продукта сказано наоборот; та фраза
  написана до того, как закрепился порядок ленты.

### Не нарисовано

Зум карты, экспорт и миграция круга, видеоплеер, сканер QR. Клавиатура показана один
раз — на экране новой записи, чтобы было видно, сколько остаётся места.

### 2026-08-19 — Марка: файлы и гайды

Знак существовал в одном экземпляре — инлайновым `<svg>` в шапке образа продукта.
Теперь у него есть эталонные файлы, разобранное построение и правила применения.

### Добавлено

- `assets/icon.svg` — эталон иконки. Холст 1024, знак 491 × 756 по чернильному
  габариту, оптический центр совмещён с центром холста, safe zone Android
  соблюдена. Контур перенесён из образа продукта один в один.
- `assets/icon-small.svg` — вариант для 16, 24 и 32 px: та же ось и то же поле,
  перо поднято с 0.62–2.38 до 5.0–7.0, иначе тонкий конец линии не переживает растр.
- `assets/mark.svg` — знак без подложки, берёт `currentColor`; для четырёх
  сценариев линии в интерфейсе.
- `assets/logo.svg` — лок: знак плюс слово «Wynd» в кривых. Шрифта в файле нет.
- `assets/README.md` — что где лежит и как выводится растр.
- `docs/mark.html` — гайд по знаку: смысл, построение, поле, цвет, размеры,
  сетка иконки, применение в интерфейсе, восемь плиток «как нельзя».
- `docs/logo.html` — гайд по логотипу: построение лока, вес, поле, цвет,
  тэглайн, границы размеров, восемь плиток «как нельзя».

### Разобрано

- **Знак не нарисован, а посчитан.** Контур разложен на ось и профиль пера, и
  обе части оказались точными формулами: ось — ровно один период синусоиды
  `x = −A·sin(2π·y/H)` с амплитудой `0.32 H`, перо — линейный рост по `y`
  от 0.6163 до 2.3810 перпендикулярно оси, торцы — полукруг радиусом в половину
  пера. Контур, пересчитанный по этим правилам, совпадает с исходным
  с расхождением 0.001 при высоте 60: это исходное построение, а не подгонка.
- **Лок держится на трёх числах.** Чернильные высоты знака и слова равны
  (расхождение 0.001 %), чернильные центры совмещены (0.007 % высоты), зазор
  между чернилами — `0.35 H`. Кегль слова из этого следует, а не выбирается: `0.86 H`.
- **Слово плотнее знака.** Толщина штриха измерена двумя независимыми способами
  (дистанционное преобразование и горизонтальные пробеги, сошлись на 54–56
  единицах шрифта). Самый лёгкий штрих слова — 3.4 % высоты, самое толстое перо
  знака — 3.9 %: рифма между шрифтом и знаком в характере линии, а не в весе.
  Отсюда граница лока — 48 px по высоте знака: при уменьшении первым пропадает
  знак, а не слово.
- Кривые слова сверены с набором шрифтом: при кегле 200 px обе версии дают
  13 048 чернильных пикселей, расхождения не выходят за сглаживание.

### Исправлено

- Чернильная высота знака — **61.474**, а не 61.342. Первый расчёт брал габарит
  по узлам контура и не учитывал, что дуги торцов выходят за них. Пересчитано
  точно, иконки перегенерированы; сдвиг 0.2 %, но числа в гайдах теперь верные.
  Ширина знака в иконке — 491, не 492.
- Из `docs/mark.html` убран встроенный шрифт: лок в нём показан теми же кривыми,
  что и в `assets/logo.svg`. Источник правды один.

### 2026-08-15

- `docs/wynd.html` — образ продукта по итогам проектной сессии.

### 2026-08-14

- `docs/compass_artifact_…_text_markdown.md` — конкурентный анализ.
