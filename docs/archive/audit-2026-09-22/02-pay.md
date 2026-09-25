# 02 — Оплата (аудит безопасности, план 42, волна 2а)

Дата: 2026-09-22. Область: `internal/auth/pay.go`, `internal/api/pay.go`, `internal/api/admin_pay.go`, шлюз `RequirePaidParticipant*` (`internal/api/admin.go`), маршруты pay в `internal/api/server.go`, тесты, справочник.

Статус: завершено. Итог: высокая — 1, средняя — 2, низкая — 3, заметка — 3.

## Находки

### Уникальность pending-заявки — check-then-act вне транзакции, без ограничения в схеме
- Серьёзность: средняя
- Уверенность: подтверждено кодом (проследил `CreatePayRequest` и схему `0004_payment.sql`)
- Где: `internal/auth/pay.go:341-349` (COUNT pending), `internal/auth/pay.go:368-386` (транзакция начинается уже после проверки); схема `internal/store/migrations/0004_payment.sql:14-26` — только обычные индексы, уникального частичного индекса на `(account_id) WHERE status='pending'` нет; позднейшие миграции таблицу не трогают (только 0011 — формат времени).
- Атакующий: участник (свой аккаунт).
- Что: проверка «есть ли pending» и INSERT разделены; два параллельных `POST /pay/requests` создают две pending-заявки. Каждая из них независимо утверждается админом (`ApprovePayRequest` продлевает от текущего срока), т.е. один платёж можно «продать» дважды — админ видит две строки от одного e-mail и должен сам заметить дубль. Также обходится клиентский лимит «одна заявка».
- Как воспроизвести: параллельно два `POST /pay/requests {"blob_id": B1|B2}` от одной сессии (два разных complete-блоба владельца); `GET /admin/pay/requests` вернёт две pending-строки одного `account_id`.
- Исправление: `CREATE UNIQUE INDEX ... ON pay_requests(account_id) WHERE status = 'pending'` (+ маппинг ошибки UNIQUE → `ErrConflict`), либо перенести COUNT внутрь той же `BEGIN IMMEDIATE` транзакции, как сделано в QLT-1 для других предусловий.

### Продление «бессрочного» срока делает его конечным (задокументировано, но без предохранителя)
- Серьёзность: заметка (не атака — ошибка админа; поведение описано в `docs/reference/server-reference.md:119` «бессрочная … считается от сегодня»)
- Уверенность: подтверждено кодом
- Где: `internal/auth/pay.go:37-53` (`subscriptionExtendBase`: при `isSubscriptionUnlimitedRaw` база = `now`), использование — `GrantPayAccount` (`493-498`) и `ApprovePayRequest` (`525-532`).
- Атакующий: нет (админ против самого себя/участника).
- Что: если у участника `9999-12-31`, а админ выдаёт `days:N` (или утверждает его заявку с `days`), срок становится `now + N дней` — участник теряет бессрочный доступ; переполнения `time` нет (база сбрасывается на `now`, а не прибавляется к 9999 году), отрицательных значений нет. Дата хранится строкой `xtime.Layout` (`internal/xtime`, фиксированная ширина, UTC) и сравнивается через `parseTime` → `time.Before`/`After`, не лексикографически (кроме двух SQL-фильтров `< ?`/`> ?` в `674-675`, `837`, где фиксированная ширина это позволяет) — смешения форматов нет (миграция 0011 нормализовала старые значения; `9999-12-31T00:00:00.000000000Z` той же ширины).
- Как воспроизвести: `PUT /admin/pay/accounts/{id} {"unlimited":true}`, затем `PUT /admin/pay/accounts/{id} {"days":30}` → `subscription_expires_at` = сегодня + 30 дней.
- Исправление: если поведение оставлять — предупреждение в админ-клиенте («у участника бессрочный доступ, продление сделает его конечным»); иначе `ErrConflict` при бессрочном текущем сроке.

### Заявка принимает любой *собственный* complete-блоб, в том числе уже занятый записью круга
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/auth/pay.go:350-362` (проверяется только `account_id` владельца и `status='complete'`), `379-383` (`INSERT OR IGNORE INTO blob_refs`).
- Атакующий: участник.
- Что: чужой блоб приложить нельзя (`owner != accountID` → 403) — вопрос 4 закрыт положительно. Но проверки «блоб ещё ни к чему не привязан» нет: участник может приложить блоб своей записи из круга; заявка добавляет ссылку `pay_request` и тем самым держит блоб от GC после удаления записи. Утечки чужих данных нет (блоб свой). Обратного эффекта («сделать чужой блоб занятым») тоже нет — чужой блоб отсекается.
- Исправление: при желании — требовать `NOT EXISTS (SELECT 1 FROM blob_refs WHERE blob_id=?)`; безопасностной необходимости нет.

### `ApprovePayRequest`: проверка `pending` и запись в разных транзакциях, UPDATE без guard по статусу
- Серьёзность: средняя
- Уверенность: подтверждено кодом
- Где: `internal/auth/pay.go:510-519` (SELECT pending вне транзакции), `526-532` (чтение текущего срока вне транзакции), `540-542` (`UPDATE pay_requests SET status='approved' ... WHERE id = ?` — без `AND status = 'pending'`, `RowsAffected` не проверяется). Для сравнения `RejectPayRequest` (`565-578`) guard и RowsAffected имеет.
- Атакующий: нет внешнего (единственный админ; двойной клик / повтор запроса клиентом / два окна).
- Что: (а) два параллельных approve одной заявки: оба проходят SELECT pending; первый коммитит `X+N`; второй читает уже `X+N` и записывает `X+2N` — двойное продление за один платёж. (б) approve параллельно с reject: reject уже удалил файл скриншота и пометил `rejected`, затем approve без guard перезаписывает статус в `approved` и продлевает подписку — заявка «утверждена», хотя админ её отклонил; последовательный approve после reject корректно даёт 404 (SELECT со `status='pending'`). (в) Заявка мягко-удалённой учётки: `ListPendingPayRequests`/`PayRequestByID` (`252-291`) джойнят `accounts` без `deleted_at IS NULL`, а `UPDATE accounts` в approve (`545-547`) тоже без него — в отличие от `setAccountSubscriptionExpires` (`55-71`). Админ видит и «утверждает» заявки удалённых учёток; вреда нет, но несогласованно.
- Как воспроизвести: два одновременных `POST /admin/pay/requests/{id}/approve {"days":30}` → `subscription_expires_at` = now+60d, оба ответа 200/204.
- Исправление: одна транзакция `BEGIN IMMEDIATE`: `UPDATE pay_requests SET status='approved' ... WHERE id=? AND status='pending'`, при `RowsAffected==0` → `ErrNotFound`/`ErrConflict`, чтение текущего срока и запись нового — внутри той же транзакции (как QLT-3 для PATCH круга).

### После отклонения заявки блоб остаётся `complete` с удалённым файлом — его можно приложить повторно
- Серьёзность: низкая
- Уверенность: предположение (нужен пробный тест; путь удаления файла проследил, статус `blobs` не меняется — `pay.go:705-740`, комментарий «Keep the blobs row»)
- Где: `internal/auth/pay.go:705-740` (`removePayRequestBlob` удаляет файл, строку `blobs` и её `status='complete'` не трогает), `350-362` (`CreatePayRequest` принимает любой свой блоб со `status='complete'`), `660-666` (`PayBlobAllowedForAdmin` — по новой заявке `blob_deleted=0` → true).
- Атакующий: участник (мелкий DoS/путаница админа).
- Что: после reject участник может создать новую заявку с тем же `blob_id`: строка `blobs` осталась `complete`, файла нет. Админ в списке видит имя файла и размер, `GET /admin/pay/blob/{id}` → 404 (`blob.OpenBlob` делает `os.Stat`, `internal/blob/serve.go:37-39`), клиент показывает заявку без картинки — «пустая» заявка, которую админ обязан отклонить вручную. Также занятое место, если `UPL-2` считает по строкам `blobs`, а не по файлам, продолжает учитываться за участником (это его же квота — не обход).
- Как воспроизвести: заявка с блобом B → reject → `POST /pay/requests {"blob_id": B}` → 201; `GET /admin/pay/blob/B` → ошибка чтения файла.
- Исправление: в `CreatePayRequest` отвергать блоб, у которого есть `pay_requests` с `blob_deleted=1` (или при удалении файла переводить `blobs.status` в отдельное состояние и требовать `status='complete'` на файле).

### Истёкший участник не может загрузить скриншот: `/uploads` под шлюзом `paid`, а заявка требует `blob_id`
- Серьёзность: высокая (для продукта: включение шлюза запирает всех без возможности оплатить через клиент; безопасностно — отказ обслуживания собственной «дверью оплаты»)
- Уверенность: подтверждено кодом (маршруты + клиентский поток)
- Где: `internal/api/server.go:148-151` (`POST /uploads`, `PUT /uploads/{id}`, `POST /uploads/{id}/complete` — все под `paid`/`RequirePaidParticipantStream`), `internal/api/server.go:174-176` (под `participant` только `pay/status`, `pay/requests`, `pay/banner/dismiss`), `internal/auth/pay.go:327-330` (`blobID == ""` → `ErrInvalid` — скриншот обязателен), `internal/api/admin.go:348-361` (`requirePaidSession`: `required && expired && has_requisites` → 403 `payment_required`), клиент `web/src/routes/pay/+page.svelte:56-57` → `web/src/lib/journal/posts.ts:17-46` (`uploadBlob` ходит в `/uploads`).
- Атакующий: нет (админ против всех участников; но эффект — полная блокировка входа в приложение до ручного `PUT /admin/pay/accounts/{id}`).
- Что: по вопросу 1 — при `required=true` и непустых реквизитах все учётки с `subscription_expires_at IS NULL` немедленно `expired=true` (`isSubscriptionExpired`: пусто → `true`, `pay.go:749-752`), льготного периода нет; под `participant` остаются только `logout`, `pay/status`, `pay/requests`, `pay/banner/dismiss`. Но чтобы подать заявку, нужен загруженный блоб, а все три маршрута загрузки — под `paid`. Итог: истёкший участник получает 403 `payment_required` уже на `POST /uploads` и не может «сказать, что оплатил»; сценарий «включил подписку → все ждут ручного продления» гарантирован. Ситуация «бессрочно у всех до включения» не предусмотрена: `PUT /admin/pay/subscription` (`pay.go:236-250`) сроки никому не проставляет. Сам админ (`admin@wynd.local`, sentinel) ходит через `requireAdmin` и не заперт; если у него есть отдельная участническая учётка — заперт наравне с остальными.
- Как воспроизвести: `PUT /admin/pay {"requisites":"..."}`, `PUT /admin/pay/subscription {"required":true,"remind_days":7}`; от участника без срока: `GET /pay/status` → `expired:true`; `POST /uploads {...}` → 403 `payment_required`; `POST /pay/requests` без `blob_id` → 400.
- Исправление: либо отдельный маршрут загрузки скриншота под `participant` (с малым лимитом размера и без привязки к кругу), либо пропускать `/uploads*` через `participant` с ограничением «у истёкшего — только один блоб pay-назначения»; плюс при включении `required` предложить админу проставить всем текущим учёткам стартовый срок (льготный период N дней). Тестом закрепить `TestPaidGateBlocksJournalButNotPayRoutes` из архива (`docs/archive/review-2026-09-21/review-api-spec-tests.md:200`) — он там описан, но в коде отсутствует.


### `handleAdminPayBlob` отдаёт скриншот `inline`, игнорируя `Disposition` из `blob.OpenBlob`
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/api/admin_pay.go:184-189` (`Content-Disposition: inline; filename="..."`, `Content-Type: info.MimeType`), для сравнения `internal/api/blobs.go:143-145` использует `info.Disposition` (всегда `attachment`, `internal/blob/serve.go:18,52`). Нейтрализация `executableExtension` (`serve.go:56-70`) покрывает только `text/html`, `xhtml`, `svg`, `javascript`.
- Атакующий: участник против админа (ограниченно).
- Что: MIME задаёт участник при `POST /uploads` (`internal/blob/upload.go:53-58`, только длина ≤255). Скриншот с `mime_type: application/xml`/`text/xml` (XML с xhtml-namespace и `<script>`) уйдёт админу `inline` с исполняемым типом. Практический эффект в официальном клиенте нулевой: страница берёт файл через `fetch` с Bearer и вставляет `URL.createObjectURL` в `<img>` (`web/src/routes/admin/pay/requests/[id]/+page.svelte:63-70,150-155`), скрипты в `<img>` не исполняются; прямая навигация по URL без Bearer даёт 403. Остаётся расхождение политики с `handleServeBlob`, которое сработает, если когда-нибудь появится cookie-аутентификация админа или ссылка «открыть оригинал». В `filename="…"` кавычка не экранируется (`sanitizeFilename` убирает только управляющие символы, `internal/blob/filename.go:11-24`) — общее с `blobs.go:144`, CRLF невозможен (net/http заменяет).
- Как воспроизвести: `POST /uploads {"mime_type":"application/xml","filename":"x.xml",...}` → загрузить `<html xmlns="http://www.w3.org/1999/xhtml"><script>…</script></html>` → `POST /pay/requests {"blob_id":…}` → `GET /admin/pay/blob/{id}` с Bearer: `Content-Type: application/xml`, `Content-Disposition: inline`.
- Исправление: использовать `info.Disposition` (attachment) или хотя бы белый список `image/*` для скриншотов оплаты (заявка и так «скриншот»); экранировать `filename` через `mime.FormatMediaType("inline", map[string]string{"filename": ...})` в обоих обработчиках.

### Комментарий заявки и текст реквизитов/баннера без предметного лимита длины
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/api/pay.go:30-38` (`comment` — только `limitBody` 1 MiB, `internal/api/respond.go:18,79-86`), `internal/auth/pay.go:373-376` (сохраняется как есть после `TrimSpace`); `internal/api/admin_pay.go:26-39` и `50-61` (`requisites`, `donate.text` — тоже только 1 MiB, админ). У записей есть `chronicle.ErrTooLong`, у заявки аналога нет.
- Атакующий: участник (мелкий DoS на админ-панель/БД).
- Что: один участник может положить в `pay_requests.comment` ~1 MiB текста; `GET /admin/pay/requests` отдаёт все pending-комментарии целиком; после каждого reject участник создаёт новую заявку (строки `rejected` не удаляются) — рост БД ограничен только скоростью реакции админа. Реквизиты и текст баннера отображаются как текст (`RequisitesCard.svelte:19` — `{text}`, `circles/+page.svelte:358` — проп), `{@html}` нет — XSS нет.
- Как воспроизвести: `POST /pay/requests {"blob_id":…,"comment":"<1 000 000 символов>"}` → 201.
- Исправление: лимит комментария (например 2000 символов → `ErrInvalid`/`too_long`), лимит реквизитов и текста баннера (например 4000).

### Верхняя граница `days` в approve/grant отсутствует: год > 9999 не парсится и трактуется как «истёк»
- Серьёзность: заметка
- Уверенность: подтверждено кодом (переполнение — по семантике `time.AddDate` и `time.Parse`; пробный тест не обязателен)
- Где: `internal/auth/pay.go:490-498`, `507-508,531-532` (`days < 1` — единственная проверка), `749-761` (`parseTime` ошибка → `expired=true`), `26-35` (`isSubscriptionUnlimitedRaw` при ошибке парсинга → false).
- Атакующий: нет (ошибка админа / кривой клиент).
- Что: `days` ≥ ~2.9 млн даёт год > 9999; `xtime.Format` печатает год шире четырёх знаков, `time.Parse(RFC3339Nano)` такую строку не читает → участник считается истёкшим, а `ListPayReminderCandidates` (`837`, лексикографическое `>`) ведёт себя непредсказуемо. Также `PUT /admin/pay/donate` не валидирует `until` (`pay.go:200`): неразборчивая строка → `parseDonateUntil` !ok → баннер активен бессрочно (`770-773`).
- Исправление: `days` ≤ 3660 (10 лет), иначе `ErrInvalid`; `until` — проверять `YYYY-MM-DD` при сохранении.

## Ответы на вопросы задания

1. **Включение шлюза при пустых сроках.** Шлюз = `required && expired && has_requisites` (`internal/api/admin.go:357`). Пока реквизиты пусты — никто не заперт (мягкий режим). Как только реквизиты заданы, все учётки с `subscription_expires_at IS NULL` немедленно `expired` (`pay.go:749-752`), льготного периода нет, `PUT /admin/pay/subscription` никому сроков не проставляет. Админ (sentinel `admin@wynd.local`) ходит через `requireAdmin` и не заперт; его отдельная участническая учётка — заперта наравне со всеми. Под `participant` остаются `POST /auth/logout`, `GET /pay/status`, `POST /pay/requests`, `POST /pay/banner/dismiss` (`server.go:125,174-176`); всё остальное, **включая `/uploads*`**, — под `paid`. Отсюда находка «истёкший не может загрузить скриншот» (высокая).
2. **Продление от 9999-12-31.** База сбрасывается на `now` (`subscriptionExtendBase`), срок становится конечным; переполнения/отрицательных нет; задокументировано. Хранение — строка `xtime.Layout` (фиксированная ширина, UTC, наносекунды), сравнение в Go через `time` после `parseTime`; два SQL-сравнения строк (`674-675`, `837`) корректны при фиксированной ширине. Верхняя граница `days` отсутствует (заметка).
3. **Параллельные заявки.** Уникальность pending — check-then-act без индекса (средняя). Два approve — двойное продление, approve параллельно с reject — перезапись `rejected` в `approved` (средняя). Последовательный approve после reject — 404, корректно. Approve заявки заблокированной учётки — продлевает, блокировка остаётся (норма); мягко-удалённой — продлевает и показывает в списке (заметка внутри находки про approve).
4. **Скриншот.** Чужой блоб приложить нельзя (`owner != accountID` → 403, `pay.go:360-362`); собственный, уже занятый записью — можно (заметка). `handleAdminPayBlob` отдаёт только блобы, привязанные к `pay_requests` с `blob_deleted=0` (любого статуса) — `PayBlobAllowedForAdmin`. Заголовки: `Content-Type` из клиентского MIME (с нейтрализацией html/svg/js в `OpenBlob`), `Content-Disposition: inline` вместо `attachment` (низкая), `nosniff`, `Content-Length`.
5. **Утечки.** `GET /pay/status` — только своё (срок, своя pending-заявка, реквизиты, имя инстанса, баннер); чужих e-mail/id нет. `GET /admin/pay/*` — e-mail и `account_id` участников, только админу. Реквизиты и текст баннера показываются как текст (`RequisitesCard.svelte:19`, `{text}`), `{@html}` в pay-страницах нет.
6. **Баннер.** `POST /pay/banner/dismiss` пишет `pay_donate_dismissed_version = pay_donate_version` только для `sess.AccountID` (`api/pay.go:52`, `pay.go:585-594`) — чужой сбросить нельзя. Версия — счётчик в `instance_settings`, растёт при включении баннера или смене текста/срока/dismissible при включённом показе (`pay.go:209-216`); `Dismissed = dismissedVersion >= version && version > 0`.
7. **Лимиты.** Все pay-маршруты проходят `limitBody` (через `requireAdmin`/`RequireParticipant`, 1 MiB). Специального лимитера нет — как и у остального API (лимитер только у `/probe*`). Свободный текст (`comment`, `requisites`, `donate.text`) без предметного лимита (низкая).
8. **Тесты.** См. ниже.

## Покрытие тестами

Есть: `internal/auth/pay_test.go` — `TestDonateUntilDateExpires`, `TestDonateVersionBumpsOnlyWhenShown`, `TestPayReminderLastDayIsOne`, `TestGrantPayAccountUnlimitedNotExpired`, `TestGrantPayAccountDaysFromEmptyFuturePast` (пустой срок → expired, продление от now/будущего/прошлого); `internal/auth/pay_gate_test.go` и `internal/api/admin_pay_test.go` — list/by-id/grant при выключенной подписке → `invalid`; `internal/jobs/pay_test.go` — очистка скриншота не удаляет строку `blobs`.

Нет ни одного теста на: `CreatePayRequest` (чужой блоб → 403, повторная pending → 409, без реквизитов/подписки → 400), `ApprovePayRequest`/`RejectPayRequest` (в т.ч. approve после reject, удаление файла при reject, сохранение файла при общем блобе с записью), шлюз `RequirePaidParticipant` на HTTP-уровне (403 `payment_required` на `/circles`, доступность `/pay/*`), `GET /admin/pay/blob/{id}` (блоб вне заявок → 403), `POST /pay/banner/dismiss`, гонки (двойной POST/approve). Три теста из плана 42 (`docs/plans/42-code-review.plan.md:377`) не написаны; в архивной рецензии (`docs/archive/review-2026-09-21/review-api-spec-tests.md:200-201`) сценарии расписаны. К ним стоит добавить `TestExpiredParticipantCanUploadPayScreenshot` — сейчас он бы упал и зафиксировал находку про `/uploads`.

## Проверено, в порядке

- `RequirePaidParticipant`/`RequirePaidParticipantStream` (`admin.go:311-346`): порядок сессия → `RejectAdminJournal` → шлюз; админская сессия на участнических маршрутах отсекается; `Stream`-вариант без `limitBody` только на PUT-чанке (`server.go:150`), остальные загрузочные маршруты под `paid` с лимитом.
- Все `admin/pay/*` под `requireAdmin` (`server.go:107-120`); участнические pay-маршруты под `participant` с `limitBody` (`174-176`).
- `CreatePayRequest`: владелец блоба и `status='complete'` проверяются (`pay.go:350-362`); без реквизитов или при выключенной подписке — `ErrInvalid` (`335-340`); `blob_refs` добавляется в той же транзакции, что и заявка.
- `RejectPayRequest`: guard `status='pending'` + `RowsAffected` (`565-578`); удаление файла через `blob.IsFileNeeded` внутри транзакции — блоб, общий с записью круга, не теряет файл (`705-740`; BLB-1/BLB-2 закрыты и в этом пути). Рутина `CleanupExpiredPayScreenshots` использует тот же путь.
- `PayBlobAllowedForAdmin` (`660-666`) — только блобы из `pay_requests` с `blob_deleted=0`; блоб вне заявок → 403; `OpenBlob` требует `status='complete'` и наличие файла → иначе 404.
- `ListPayAccounts`/`PayAccountByID`/`GrantPayAccount` исключают sentinel `admin@wynd.local` и мягко-удалённых (`416-473`); `setAccountSubscriptionExpires` (`55-71`) — `deleted_at IS NULL` + `RowsAffected`.
- `ListPayReminderCandidates` (`822-857`) — `blocked=0`, `deleted_at IS NULL`, не sentinel; повтор письма исключён через `pay_reminder_sent_for = subscription_expires_at`, сбрасываемый при любом продлении.
- `SetPaySubscriptionSettings` валидирует `remind_days` из {1,3,7} (совпадает с CHECK в схеме).
- `IsParticipantSession` (`sessions.go:91-104`) отсекает заблокированных — заблокированный не подаст заявку и не сбросит баннер.
- `PayStatus` не раскрывает чужих данных; `Reminder` не ставится при `Pending`/`Expired`/бессрочном (`651-656`, `792-812`).
- Клиент: реквизиты, текст баннера, комментарий, имя файла — текстовые вставки Svelte; скриншот у админа — `<img src=blob:>` (`admin/pay/requests/[id]/+page.svelte:151`, `admin/pay/subscription/+page.svelte:69-71`).
- Формат времени: `xtime.Layout` фиксированной ширины (TIME-1 закрыт), миграция 0011 нормализовала `subscription_expires_at`, `pay_requests.created_at/resolved_at`.
- Раздел B плана 42 (закрытые развилки) находками не затрагивается: sniffing MIME не предлагается (только `attachment`/белый список для скриншотов).

## Что остаётся на волну 7

- Вынос в `internal/pay` (ARC-2, план 42 раздел B «Оплата в `auth`»): в `auth` остаётся только `requirePaidSession` через узкий интерфейс вида `PaidStatus(ctx, accountID, now) (required, expired, hasRequisites bool)`. `PayStatus` сейчас делает 4 запроса на **каждый** `paid`-запрос (`loadPaySettingsRow`, `loadInstance`, аккаунт, pending-заявка) — шлюзу нужны два поля; облегчённая проверка или кэш `instance_settings`.
- Единый стиль записи в БД: `CreatePayRequest`, `ApprovePayRequest`, `GrantPayAccount` — «предусловия в той же транзакции» (образец QLT-1/QLT-3); сейчас три разных стиля.
- `scanPayRequestRow` + два одинаковых SELECT в `ListPendingPayRequests` и `PayRequestByID` — один запрос с параметром.
- `handleAdminPayHub` собирает ответ вручную, остальные отдают структуры целиком; `PaySettings` смешивает три подсистемы (реквизиты, баннер, подписка) в одной строке `instance_settings`.
- `handleAdminPayBlob` дублирует `handleServeBlob` (открытие файла, заголовки) — общий `serveBlobInfo(w, info)`.
- `removePayRequestBlob` и `gcBlobIfUnreferenced` (`internal/blob/serve.go:179-203`) — два пути удаления файла блоба с разной семантикой («строку оставить» vs «строку удалить»); задокументировано, но стоит свести к одному API `blob.Store`.
- OpenAPI (`web/src/lib/api/schema-*.d.ts`) не описывает `payment_required` как отдельный вариант 403 — клиент различает по телу (`client.ts:97-98`).

Статус: завершено.
