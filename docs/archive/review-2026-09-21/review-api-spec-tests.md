# Ревью: OpenAPI ↔ код, сервер ↔ клиент, тесты API (Wynd)

Статус файла: В РАБОТЕ (дополняется по шагам).

## OpenAPI ↔ маршруты

Сверка выполнена вручную: `internal/api/server.go:58-185` (120 операций `HandleFunc`) против `paths:` в `openapi-participant.yaml` и `openapi-admin.yaml`.

**Результат двусторонней сверки: расхождений НЕТ.** Все 127 операций mux есть в спеках, в спеках нет лишних операций, методы совпадают (включая HEAD `/uploads/{session_id}`). Вне `/api/v1` в `cmd/wynd/main.go:119-133` зарегистрированы `GET /health`, `GET /ready`, `/api/`, `/` — в спеках их нет (см. R-2).

### R-1. Сторож маршрутов существует, но объединяет обе спеки в одно множество — INFO/LOW, CONFIRMED
- `internal/api/openapi_test.go:20-22` — `mergeOps(spec, participant)`, `mergeOps(spec, admin)`.
- Что: `TestOpenAPICoversMuxRoutes` проверяет «mux ⊆ (participant ∪ admin)» и обратно. Админский маршрут, описанный по ошибке в participant-спеке (или наоборот), тест пропустит; клиентский `schema-participant.d.ts` получит админский путь.
- Цена: низкая, сейчас нарушений нет.
- Исправление: в тесте делить mux-операции по префиксу `/admin/` и сверять каждую половину со своим yaml.

### R-2. `/health` и `/ready` не описаны ни в одной спеке — LOW, CONFIRMED
- `cmd/wynd/main.go:119-120`.
- Что: эксплуатационные эндпоинты вне `/api/v1`, сторож их не видит (regex `handleFuncRe` требует `/api/v1/`, `openapi_test.go:44`), спеки про них молчат.
- Цена: оператор/обратный прокси узнаёт про них только из README/кода.
- Исправление: оставить вне OpenAPI, но добавить одну строку в `info.description` обеих спек: «/health и /ready — вне /api/v1, см. README».

### R-3. Сторож парсит исходник regex-ом, а не реальный mux — LOW, CONFIRMED
- `internal/api/openapi_test.go:44` — `HandleFunc\("(GET|...) (/api/v1/[^"]+)"`.
- Что: маршрут, зарегистрированный через `s.Mux.Handle(` (без Func), через переменную пути или в другом файле пакета, сторож не увидит. Сейчас таких нет (проверено Grep по `*.go`).
- Цена: ложное чувство защиты при будущем рефакторинге.
- Исправление: оставить как есть, добавить в тест проверку-счётчик: `strings.Count(raw, "s.Mux.Handle") == len(mux)` — любая нераспознанная регистрация роняет тест.

## ВНЕ ПЛАНА, НО ВАЖНО: побочные эффекты до проверки токена в bootstrap

### X-1. `POST /admin/bootstrap` пишет public_url и SMTP-конфиг ДО проверки токена и флага bootstrapped — HIGH (security), CONFIRMED чтением кода
- `internal/api/admin.go:40-90`: порядок действий — `config.WritePublicURL` + `s.applyPublicURL` (строки 57-62), `s.Mail.SaveConfig` (69-78), `s.Mail.SendTest` (79) и только потом `s.Auth.Bootstrap` (84), где проверяются токен и `bootstrapped == 1` (`internal/auth/admin.go:64-82`). Маршрут публичный (`server.go:68`, обёртка `public` = только `limitBody`, `server.go:57`). `WritePublicURL` (`internal/config/public_url.go:25`) и `SaveConfig` (`internal/mail/mail.go:99-113`) собственных сторожей не имеют.
- Для сравнения `handleBootstrapSMTPTest` (`admin.go:104`) сначала зовёт `ConfirmBootstrapToken` — то есть правильный порядок в соседнем обработчике есть, здесь он потерян.
- Что: аноним без токена на уже настроенном инстансе может (а) перезаписать SMTP host/username/password/from → коды входа уходят через чужой SMTP = захват аккаунтов; (б) переписать `public_url` в `config.json` и переключить `Loopback` (loopback-URL → `code_delivery=log`, ослабленные проверки); (в) заставить сервер слать тестовые письма. Ответ при этом 400 — атака «невидима» по статусу.
- Тесты этого не ловят: `security_test.go:315-325` (`TestBootstrapWeakPassword`) и `admin_test.go:291,314` проверяют только код ответа, не неизменность SMTP/public_url.
- Цена: полная компрометация входа по почте на любом инстансе, доступном из сети.
- Исправление: первой строкой после `readJSON` в `handleBootstrap` вызвать `s.Auth.ConfirmBootstrapToken(...)` (как в `admin.go:104`) и выйти при ошибке; добавить тест `TestBootstrapRejectedLeavesSMTPAndPublicURLUntouched` (см. список тестов, №1).
- Оговорка: не запускал, вывод сделан по чтению кода; стоит подтвердить одним curl на loopback-инстансе.

## OpenAPI ↔ поля

Главный вывод: спеки — это скелет маршрутов, а не контракт. В `openapi-participant.yaml` нет ни одной схемы в `components` (строки 695-699 — только `securitySchemes`), в `openapi-admin.yaml` — тоже (438-442). Ответы описаны свободным текстом в `description` («Posts with media, comments, reactions»), тела запросов описаны лишь у 6 операций participant и 3 операций admin. Поэтому «сравнение полей» в большинстве случаев = «в спеке поля нет вообще».

### F-1. У ~25 операций participant-спеки нет обязательного блока `responses` — MEDIUM, CONFIRMED
- `openapi-participant.yaml:523-672`: `head/put /uploads/{session_id}`, `/uploads/{session_id}/complete`, `/blobs/{blob_id}`, `patch/delete posts`, все comments/reactions/days title/cover, `/circles/{circle_id}` get/patch/delete, members, identity, transfer, exclude, quota, archive*, notify_prefs*, push/subscribe*, quota_requests — только `summary`.
- Что: по OpenAPI 3.x `responses` обязателен; документ формально невалиден, любой строгий линтер (spectral, redocly) его отвергнет. `openapi-typescript` терпит, поэтому не замечено.
- Цена: спека непригодна для генерации клиента/моков/контрактных тестов.
- Исправление: добавить каждой такой операции минимальный `responses: {"200": {description: OK}}` (или 201/204 по факту) и path-параметры; в CI — `npx @redocly/cli lint`.

### F-2. `PATCH /circles/{circle_id}/posts/{post_id}`: в спеке нет поля `media[]` — MEDIUM, CONFIRMED
- Спека `openapi-participant.yaml:553-564`: `body`, `entry_date`, `cover_blob_id`. Код `internal/api/journal.go:29-34` (`editPostBody`): ещё `media *[]mediaBody` (`blob_id`, `kind`, `captured_at`, `geo_lat`, `geo_lng`, `is_cover`, `journal.go:20-27`). Поле покрыто тестами `patch_post_media_test.go`, в `schema-participant.d.ts:1394` его нет.
- Дополнительно: при переданном `media` обработчик молча игнорирует `cover_blob_id` (`journal.go:134-150`) — в спеке это не оговорено.
- Исправление: дописать `media` (array of MediaInput) в схему PATCH и фразу «при media обложка задаётся is_cover, cover_blob_id игнорируется».

### F-3. `POST /circles/{circle_id}/posts`: тело запроса не описано совсем — MEDIUM, CONFIRMED
- Спека `openapi-participant.yaml:537-548` — только path-параметр и «201 Post created». Код `journal.go:13-18` (`createPostBody`: `body`, `entry_date`, `captured_at`, `media[]`), правило «body или media обязательно» (`journal.go:65`).
- Исправление: вынести `MediaInput`, `CreatePostBody`, `PostResponse` в `components.schemas` и сослаться из POST/PATCH.

### F-4. Комментарии, реакции, заголовок/обложка дня: ни тел, ни ответов — MEDIUM, CONFIRMED
- Спека `openapi-participant.yaml:568-594` — одни `summary`. Код: `textBody{body}` (`journal.go:36-38`), `reactionBody{emoji}` (40-42), `dayTitleBody{title}` (44-46), `dayCoverBody{post_id, blob_id}` (48-51).
- Исправление: четыре схемы запросов в `components.schemas` + `required`.

### F-5. `POST /circles`: тело не описано — LOW, CONFIRMED
- Спека `openapi-participant.yaml:87-91`. Код `circles.go:44-49`: `name`, `owner_name`, `edit_window_sec?`, `color?` (по умолчанию `ochre`, окно — безлимит). Ответ `{id,name,color}` (`circles.go:78-80`) в спеке — «Circle created».
- Исправление: описать тело и ответ схемой.

### F-6. `POST /circles/{id}/invites` и `POST /admin/invites`: enum `kind` не соблюдается кодом — LOW, CONFIRMED
- Спека: `kind: enum [single, multi]` (`openapi-participant.yaml:120-122`, `openapi-admin.yaml:184-186`). Код `circles.go:105-108`: любое значение, кроме `"multi"` (в т.ч. `"foo"`, пусто), молча становится `single`; `max_uses < 1` → 1; `ttl_sec <= 0` → 72 ч; верхней границы `ttl_sec` нет (`circles.go:124-127`).
- Имена полей совпадают (`kind`, `max_uses`, `ttl_sec`). Ответ `{id, token, expires_at, ...}` (`circles.go:136-139`) в спеке — «Invite token».
- Исправление: в коде вернуть `ErrInvalid` при `kind ∉ {"", single, multi}`; в спеке указать `default` для всех трёх полей.

### F-7. Совпадения (проверено, расхождений нет)
- `POST member-invites`: `account_id` required — `member_invites.go:34` ↔ спека 155-164.
- `POST /circles/{id}/join`: `name` required, `body` optional — `member_invites.go:133-134` ↔ спека 191-202.
- `PUT read_cursor`: `seq` required integer — `circles.go:215` ↔ спека 254-263.
- `POST admin/pay/requests/{id}/approve` и `PUT admin/pay/accounts/{id}`: `days`, `unlimited` — спека `openapi-admin.yaml:355-365, 410-420` (обработчик сверю ниже, см. F-9).

### F-8. `POST /admin/bootstrap` и `/admin/bootstrap/smtp-test`: тело не описано — MEDIUM, CONFIRMED
- Спека `openapi-admin.yaml:29-43` — «200 OK». Код `admin.go:~12-24` (`bootstrapBody`): `token`, `instance_name`, `password`, `public_url`, `host`, `port`, `username`, `smtp_password`, `from`. Условная обязательность (SMTP обязателен, если `public_url` не loopback, `admin.go:51-56`) нигде не документирована.
- Исправление: схема `BootstrapBody` с `required: [token, instance_name, password]` и описанием условия SMTP.

### F-9. Admin pay approve / grant: поля совпадают, но взаимоисключение не описано — LOW, CONFIRMED
- Код `internal/api/admin_pay.go:104-106, 138-140`: `days int`, `unlimited bool` ↔ спека `openapi-admin.yaml:355-365, 410-420`. Имена совпадают; в спеке нет `required`/`oneOf` (что будет при `days=0, unlimited=false`?).
- Исправление: в спеке `oneOf: [{required:[days]}, {required:[unlimited]}]` + `minimum: 1` у `days`.

### F-10. `GET /pay/status`: спека перечисляет 5 полей, код отдаёт 15 — MEDIUM, CONFIRMED
- Спека `openapi-participant.yaml:677-679`: «required, expires_at, pending, banner, dismissed». Код `internal/auth/pay.go:117-132`: ещё `expired`, `pending_at`, `pending_comment`, `pending_blob_filename`, `requisites`, `has_requisites`, `reminder`, `reminder_days_left`, `instance_name`; `banner` = `{text, dismissible}` (135-138).
- Исправление: схема `PayStatus` в `components.schemas`, сгенерированная 1:1 из структуры.

### F-11. `POST /pay/requests`: тело не описано — LOW, CONFIRMED
- Код `internal/api/pay.go:30-33`: `blob_id`, `comment`; ответ `{id}` (43). Спека 681-686: «201 request id».
- Исправление: схема тела с `required: [blob_id]`.

### F-12. `GET /admin/accounts`: summary спеки противоречит коду — LOW, CONFIRMED
- Спека `openapi-admin.yaml:112`: «email and circle count only». Код `internal/auth/admin_access.go:11-17` (`AccountSummary`): `id`, `email`, `circle_count`, `created_at`, `blocked`.
- Исправление: убрать «only» из summary и описать схему `AccountSummary`.

### F-13. `PUT /admin/storage/quota`, `default_quota`, `circles/{id}/quota`, `compression`: тела не описаны — MEDIUM, CONFIRMED
- Код `internal/api/admin_storage.go:13-33`: `storageQuotaBody{quota_bytes?, quota_disk_percent?}` — ровно одно из двух (`admin_storage.go:105-110`), `quota_bytes >= 1` (117); `defaultQuotaBody{default_circle_quota_bytes?}`; `circleQuotaBody{custom, quota_bytes?}`; `compressionBody{photo_max_px, photo_quality, video_max_height, video_bitrate_kbps, attachment_max_bytes}`.
- Спека `openapi-admin.yaml:52-89`: только summary «absolute bytes or disk percent, not both» и «200 OK». Ответ `GET /admin/storage` в спеке перечисляет 5 ключей, код отдаёт 6 (нет `default_circle_quota_bytes`, `admin_storage.go:66-73`); поля `circles[]` (`id,name,color,posts,media_bytes,quota_bytes,quota_custom,owner_email`, 76-85) не описаны.
- Исправление: четыре схемы тел + `StorageResponse`; у quota — `oneOf`.

### F-14. Поиск: параметр `author` принимается там, где спека его не объявляет — LOW, CONFIRMED
- `internal/api/search.go:61-69` — `parseSearchFilters` общий для трёх обработчиков и всегда читает `author`. Спека объявляет `author` только у `/circles/{id}/search` (`openapi-participant.yaml:416-419`); у `/search` summary прямо говорит «no author» (426), но `SearchAll` получает `filters.Author` и применит его в `filterSQL` (`internal/search/search.go:222-225`). У `/search/authors` сервис сам обнуляет `Author` (`search.go:68`) — там корректно.
- Ответы `{"hits": [...]}` / `{"authors": [...]}` (`search.go:25,42,58`) в спеке — «Search hits», структура `Hit` не описана. `limit`: спека `default: 50`, код — «`<=0` или `> defaultLimit` → defaultLimit» (`internal/search/search.go:118-120`), максимум не объявлен.
- Исправление: в `handleGlobalSearch` обнулять `Author` перед вызовом (одна строка) — поведение совпадёт со спекой; в спеке добавить `maximum` для `limit`.

### F-15. Snapshots (`feed`, `grid`, `map`, `days`, `days/{date}`), `quota`, `circle detail`: ответы — свободный текст — MEDIUM, CONFIRMED
- Спека `openapi-participant.yaml:268-337, 596-628`: «Posts with media, comments, reactions», «Photo tiles», «Day summaries». Это самые толстые ответы API (клиент на них построен), и именно они не типизированы.
- Исправление: описать `FeedPost`, `GridTile`, `MapPin`, `DaySummary`, `CircleDetail`, `CircleQuota` в `components.schemas` — с них начинать, если делать спеку контрактом.

## Генерация типов и сторожа

Факты:
- Сторож спека ↔ маршруты ЕСТЬ: `internal/api/openapi_test.go:12` `TestOpenAPICoversMuxRoutes` (двусторонний, по методам). Он сравнивает только «метод + путь», не поля, не `info.version`. Его слабости — R-1, R-3 выше.
- Типы генерируются вручную: `web/package.json:13` — `api:types` = `openapi-typescript` (devDependency `^7.8.0`, строка 34) по обоим yaml → `web/src/lib/api/schema-participant.d.ts`, `schema-admin.d.ts`.
- Сверка свежести по числу путей: participant yaml — 60 путей, `schema-participant.d.ts` — 60; admin yaml — 37, `schema-admin.d.ts` — 37. На сегодня d.ts синхронны с yaml по набору путей (по полям — тоже, т.к. в yaml полей почти нет; `cover_blob_id?` на `schema-participant.d.ts:1394` соответствует yaml:563).
- Спеки бинарником НЕ отдаются: в Go-коде нет `go:embed` для yaml (есть только `web/embed.go:8` для `dist` и `internal/store/migrate.go:15` для миграций), упоминания `openapi` в `*.go` — только в `openapi_test.go`.

### G-1. Сгенерированные `schema-*.d.ts` никем не импортируются — мёртвый артефакт — MEDIUM, CONFIRMED
- Grep `schema-(participant|admin)|openapi-fetch` по всему репозиторию (без node_modules): совпадения только в `web/package.json:13` и `CHANGELOG.md:366,574,2308`. В `web/src` импортов нет.
- Что: ~1800+ строк сгенерированного кода + devDependency + ручной шаг `api:types` ничего не типизируют. Клиент описывает типы ответов руками (см. раздел B), спека на клиент не влияет, поэтому её пустота (F-1…F-15) никому не мешает и никем не замечается.
- Цена: ложное ощущение «у нас типы из OpenAPI»; каждое изменение API требует трёх ручных правок (yaml, `api:types`, ручные TS-типы), из которых проверяется тестом только первая и только на уровне пути.
- Исправление (закрытое решение): удалить оба `schema-*.d.ts`, скрипт `api:types` и зависимость `openapi-typescript`; оставить yaml как документ-оглавление маршрутов под сторожем `TestOpenAPICoversMuxRoutes`. Возвращать генерацию только вместе с реальными схемами (F-15) и `openapi-fetch`.

### G-2. Нет проверки, что d.ts свежие относительно yaml — LOW, CONFIRMED
- `scripts/test.bat:7-17` и `README.md:49-50` гоняют `go test`, `npm run check`, `check:ui`, `test` — `api:types` + `git diff --exit-code` нет нигде; CI-конфигов (`.gitea/`, `.github/`) в репозитории не найдено.
- История подтверждает дрейф: `CHANGELOG.md:366` — «OpenAPI drift: npm run api:types …» (чинили вручную постфактум).
- Исправление: снимается решением G-1; если d.ts оставлять — добавить в `scripts/test.bat` шаг `npm run api:types && git diff --exit-code -- src/lib/api`.

### G-3. `info.version` спек (0.4.4 / 0.4.3) не сверяется с VERSION (0.6.7) — LOW, CONFIRMED (известно)
- `openapi-participant.yaml:4`, `openapi-admin.yaml:4`. Два файла к тому же разошлись между собой.
- Исправление: в `openapi_test.go` добавить `TestOpenAPIVersionMatchesVERSION` — читает `../../VERSION` и regex-ом `^  version: (.+)$` оба yaml; альтернатива «убрать версию» невозможна (`info.version` обязателен).

### F-16. `GET /circles/{circle_id}/quota`: query-параметр `cutoff_date` не объявлен в спеке — LOW, CONFIRMED
- Код `internal/api/archive.go:172` читает `cutoff_date`; клиент его шлёт (`web/src/lib/circles/settings.ts:385`). Спека `openapi-participant.yaml:626-628` — без `parameters`.
- Исправление: добавить `cutoff_date` (`in: query`, `format: date`) в спеку.

## Сервер ↔ клиент: мёртвые и отсутствующие вызовы

Метод: Grep литералов путей в `web/src/lib` и `web/src/routes` (без `schema-*.d.ts` и тестов), сопоставление со 120 операциями mux. Клиент ходит через `apiJson/apiFetch` (`web/src/lib/api/client.ts:67,109`), путь склеивается с `/api/v1` в `apiPath` (`client.ts:27-30`).

**Клиентских вызовов несуществующих эндпоинтов НЕ найдено.** Все литералы путей клиента имеют пару в mux (включая HEAD `/uploads/{id}` — `web/src/lib/queue/queue.ts:62`; `archive/download` клиент вызывает по `download_url` из ответа сервера — `web/src/lib/journal/posts.ts:171-181`; `approve|reject` quota-requests — через шаблон `${action}` в `web/src/lib/admin/admin.ts`).

Серверные эндпоинты, которые клиент не вызывает никогда:

### B-1. `POST /admin/logout` не вызывается: «выход» админа не отзывает серверную сессию — MEDIUM (security), CONFIRMED
- Сервер: `internal/api/server.go:72`. Клиент: Grep `admin/logout|adminLogout` по `web/src` — 0 совпадений. `removeAdminSession` (`web/src/lib/session/session.svelte.ts:138-140`) только чистит локальное хранилище (`clearAdminSession`). Для участника аналог сделан правильно: `session.svelte.ts:67`, `web/src/lib/auth/auth.ts:223-225` зовут `/auth/logout`.
- Что: эндпоинт заведён по итогам аудита (`docs/security-audit-2026-09-03.md:36`), покрыт тестом (`security_test.go:281`), но в UI не подключён — украденный/забытый на чужой машине админ-токен живёт до истечения TTL.
- Исправление: в `removeAdminSession` перед `clearAdminSession()` вызвать `apiJson('', '/admin/logout', {method:'POST'})` в `try/catch` (зеркально `logoutSession`).

### B-2. `POST /admin/routine` не вызывается клиентом и не покрыт тестами API — LOW, CONFIRMED
- Сервер: `internal/api/server.go:96` (`handleAdminRunRoutine`). Grep `admin/routine|runRoutine` по `web/src`, `internal/**/*_test.go`, `README.md`, `docs/` — 0 совпадений.
- Что: ручной запуск ежедневной рутины (чистка сессий/кодов, вероятно и архивные дедлайны) доступен только curl-ом; недокументированный, нетестированный побочный вход в деструктивную логику.
- Исправление: удалить маршрут и обработчик (рутина и так идёт по расписанию); если нужен для отладки — оставить под build-tag `debug`.

### B-3. Диагностические `/probe*` вызываются только из админского «проверочного» экрана — INFO
- `web/src/lib/admin/external-probe.ts` — все 4 probe-маршрута используются. Не мёртвые. При этом они публичные без лимитера в таблице маршрутов (`server.go:58-62` — без обёртки `public`/`limitBody`): `/probe/stream` держит соединение ~2 с, `/probe/body` принимает тело до `body_probe_bytes` (`internal/api/probe.go:95`) анонимно — PLAUSIBLE вектор дешёвой нагрузки; вне задач этого ревью, отмечено для ревьюера безопасности.

Итог по B: мёртвых эндпоинтов 2 из 120 (`/admin/logout`, `/admin/routine`), фантомных клиентских вызовов 0.

## Тесты: пробелы и минимальный список

### Инвентарь `internal/api/*_test.go` (14 файлов, 57 `func Test`)
| Файл | Test | Назначение |
|---|---|---|
| `api_test.go` | 7 | + общие фикстуры `setupFreshAPI`/`setupAPI`/`registerSession`/`allowCircleMultiInvites` (25, 63, 175, 202). Instance, register→verify, accept invite, невалидный JSON, валидация поста, media-only пост при полной квоте, HTML-blob не исполняется |
| `acceptance_test.go` | 3 | Три сквозных сценария (23, 100, 173) + 9 общих хелперов (`doJSON`, `doGET`, `jsonStr`, `acceptInvite`, `syncEvents`, `uploadBytes`, `downloadArchive`, `assertOfflineZIP`, `eventTextRemainsDB`, строки 272-416) |
| `security_test.go` | 13 | Лимитеры, XFF, перебор кодов, lockout админа, cap тела/текста, logout, push hijack, слабый пароль bootstrap, заголовки. Единственный файл, где у тестов есть doc-комментарии с инвариантом |
| `admin_test.go` | 10 | storage+check, proxy snippet, block, SMTP test not configured, смена пароля, public URL, bootstrap x3; сюда же затесался `TestNotifyPrefsMentionsAlwaysOn` (236) — участник, не админ |
| `admin_wave_f_test.go` | 9 | Удаление/детали/список аккаунтов, квоты (default/circle/pending), TTL инвайта |
| `invites_test.go` | 4 | peek + отложенный join, запрет multi, list+revoke инвайтов круга, peek серверного инвайта |
| `member_invites_test.go` | 1 | Персональный инвайт из другого круга |
| `admin_pay_test.go` | 1 | `/admin/pay/accounts` закрыт при выключенной подписке |
| `patch_post_media_test.go` | 1 | PATCH `media[]` |
| `comment_avatar_test.go` | 1 | Аватар автора в ответе комментария |
| `probe_test.go` | 2 | probe-эндпоинты, 413 |
| `notify_test.go` | 2 | `waitNotify` idle / ctx |
| `domain_errors_test.go` | 2 | Маппинг ошибок почты в HTTP |
| `openapi_test.go` | 1 | Сторож спека ↔ mux |

### Ранжирование непокрытых обработчиков по риску
1. **Деструктив + authz владельца**: `handleDeleteCircle`, `handleTransferOwnership`, `handleExcludeMember`, `handleSetMember` — ошибка = потеря круга/захват прав; проверка «только owner» не закреплена ни одним HTTP-тестом.
2. **Деньги/доступ**: 10 обработчиков `admin_pay.go` + `handlePayStatus/CreatePayRequest/DismissPayBanner` — approve/grant открывают платный доступ; `paid()`-гейт (`server.go:118+`) проверяется только косвенно. `handleAdminPayBlob` отдаёт чужие скриншоты.
3. **Чужой контент**: `handleEditComment/DeleteComment` (автор vs не-автор vs окно правки), `handleSetReaction/DeleteReaction`.
4. **Утечка через чтение**: поиск x3 (видимые spans, не-член круга), `handleGrid/Map/DayDetail`, `handleCircleMembers/IdentityHistory/CircleDetail/CircleQuota` (quota — «owner» по спеке).
5. **Админ-настройки с секретами**: `handleAdminSetSMTP` (пароль не возвращается в GET), `handleAdminVAPID/PushTest`, `handleAdminSetStorageQuota/Compression`, `handleAdminListInvites/RevokeInvite`, quota-requests approve/reject.
6. **Протокол/состояние**: `syncSSE/syncNDJSON`, `handleUploadStatus`, `handleSetReadCursor`, `handleMoveDeadline`, `handleClearDayTitle/Cover`, notify prefs, `handlePushUnsubscribe`.

### Минимальный список тестов (15)
1. `TestBootstrapRejectedLeavesSMTPAndPublicURLUntouched` — на bootstrapped-инстансе POST `/admin/bootstrap` с неверным токеном и чужими `host/from/smtp_password/public_url` → 400 И `GET /admin/smtp` + `config.json` не изменились (закрывает X-1).
2. `TestOwnerOnlyCircleActionsForbiddenForMember` — табличный: участник без прав шлёт `DELETE /circles/{id}`, `POST transfer`, `POST exclude`, `PUT members/{id}`, `GET quota`, `PUT archive/deadline` → все 403, круг и владелец в БД прежние.
3. `TestDeleteCircleRequiresNameConfirmation` — owner с неверным именем → 400, круг жив; с верным → 200, `GET /circles` без него, у второго участника `GET feed` → 403/404.
4. `TestTransferOwnershipSwapsRights` — после transfer старый owner получает 403 на `DELETE circle`, новый — 200 на `PUT members/{id}`; transfer не-участнику → 4xx.
5. `TestExcludeMemberCutsAccess` — исключённый получает 403 на `feed`/`posts`/`sync` не отдаёт новых событий; owner не может исключить сам себя.
6. `TestNonMemberCannotReadCircleSurfaces` — табличный: посторонний аккаунт → 403/404 на `feed, grid, map, days, days/{date}, search, search/authors, members, identity, circle detail`.
7. `TestCommentEditDeleteOnlyByAuthor` — не-автор PATCH/DELETE → 403, текст в `feed` прежний; автор PATCH → новый текст в `feed`; DELETE → комментария нет.
8. `TestReactionSetReplaceDelete` — PUT emoji A, PUT emoji B → в `feed` одна реакция B от аккаунта; DELETE → 0; чужой DELETE не снимает мою.
9. `TestSearchRespectsMembershipAndFilters` — `/search` находит пост своего круга и НЕ находит пост чужого; `has_photo=1`, `from/to`, `author` сужают выдачу; пустой `q` → 400; `/search/authors` возвращает имя автора.
10. `TestPaidGateBlocksJournalButNotPayRoutes` — подписка включена, у аккаунта истекла: `GET /circles` → 402 `payment_required`; `GET /pay/status` → 200 с `required:true, expired:true`; `POST /pay/requests` → 201; повторный → конфликт/`pending:true`.
11. `TestAdminPayApproveGrantsAccessRejectDoesNot` — approve `{days:30}` → `GET /circles` участника 200, `expires_at` ≈ now+30d; reject → по-прежнему 402; approve с `days:0, unlimited:false` → 400; grant `unlimited:true` → `expires_at:null`.
12. `TestAdminPayRoutesRequireAdminAndBlobScoped` — табличный по 10 маршрутам `/admin/pay*`: без токена и с токеном участника → 401/403; `/admin/pay/blob/{id}` для blob, не привязанного к pay-request → 404.
13. `TestAdminSetSMTPNeverEchoesPassword` — PUT `/admin/smtp` → 200; `GET /admin/smtp` не содержит `smtp_password`/значения пароля; PUT без `host` → 400 и прежний конфиг.
14. `TestAdminQuotaRequestApproveRejectAndInviteRevoke` — owner создаёт quota request → в `GET /admin/quota_requests`; approve → `quota_bytes` круга в `/admin/storage` вырос; reject второго → не вырос; `DELETE /admin/invites/{id}` → `GET /invites/{token}` 404/410; `PUT /admin/storage/quota` с обоими полями → 400.
15. `TestSyncSSEAndNDJSONMatchJSON` — один и тот же `cursor` с `Accept: application/json`, `text/event-stream`, `application/x-ndjson` даёт одинаковый набор `seq`; плюс `HEAD /uploads/{id}` после частичного PUT → `Upload-Offset` = отправленным байтам, чужой аккаунт → 403/404; `PUT read_cursor` с `seq` меньше текущего не уменьшает курсор (unread в `GET /circles`).

