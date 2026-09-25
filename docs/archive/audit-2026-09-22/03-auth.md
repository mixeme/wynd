# 03 — Аутентификация, сессии, публичные маршруты, приглашения, админ-вход (SEC-6)

Аудит безопасности Wynd 0.7.0, 2026-09-22. Область: `internal/api/{respond,admin,auth,invites,member_invites,admin_access,probe,probe_limit,instance}.go`, `internal/auth/*`, `internal/chronicle/invite_*.go`.

## Находки

### Глобальный тормоз лимитера admin/login и bootstrap запирает вход админа для всех
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/auth/limiter.go:25-33,44-49,60-68`; `internal/auth/admin.go:137-141`
- Атакующий: аноним
- Что: `failLimiter` ведёт глобальный ключ `*` с порогом 50 неудач за 15 минут; при переполнении `allow()` возвращает false для **любого** ключа на 15 минут. Аноним с 10 адресами (IPv6 /64 даёт их бесконечно) по 5 неверных паролей каждый переводит `/admin/login` и `/admin/bootstrap` в состояние 429 для настоящего администратора и держит его сколько угодно (после lockout счётчик сбрасывается и добирается заново). Это тот же лимитер, что и у bootstrap, так что до установки первичная настройка тоже блокируется.
- Как воспроизвести: 50 × `POST /api/v1/admin/login {"password":"x"}` с 10 разных IP за минуту → затем правильный пароль с любого IP → 429 `rate_limited`.
- Исправление: глобальный ключ либо убрать, либо сделать его не запретом, а замедлением (задержка ответа / повышенная стоимость bcrypt уже есть); отдельный лимитер для bootstrap.

### Неограниченный по IP `POST /auth/verify` позволяет выжечь 3 попытки чужого кода
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/auth/codes.go:148-192,301-315`; `internal/api/auth.go` (handleVerify не вызывает лимитер)
- Атакующий: аноним, знающий e-mail жертвы
- Что: `Verify` не считает попытки по IP и не привязывает код к IP/сессии запросившего. Знающий e-mail жертвы шлёт 3 × `POST /auth/verify {"email":..., "code":"000000"}` сразу после того, как жертва запросила код — `claimAttempt` исчерпывает `codeMaxAttempts=3`, и настоящий код жертвы отвергается `too_many_attempts`. Новый код жертва получит не чаще 5 раз в час (`emailRateLimit`), атакующий повторяет — целевой отказ входа. Дополнительно ответы `404 not_found`/`410 expired`/`429 too_many_attempts` против `400 invalid` сообщают анониму, есть ли сейчас у адреса живая заявка на вход.
- Как воспроизвести: жертва `POST /auth/code`; атакующий 3 × `POST /auth/verify` с неверным кодом; жертва вводит верный код → 429.
- Исправление: считать попытки verify по IP (тот же `code_request_log` или `failLimiter`), и/или отдавать единый `400 invalid` на not_found/expired/too_many_attempts.

### Оракул блокировки учётки через `/auth/register` и `/auth/code` в режиме open
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/auth/codes.go:46-49,93-95`
- Атакующий: аноним
- Что: в режиме `open` ответ единый для известных и неизвестных адресов (комментарий на стр. 42-44), но заблокированная учётка отдаёт `403 forbidden` — аноним узнаёт, что адрес зарегистрирован и заблокирован. Стоит 1 запрос из лимита 10/IP/час.
- Как воспроизвести: `POST /auth/code {"email": "<blocked>"}` → 403; любой другой адрес → 200.
- Исправление: для заблокированных отвечать 200 без отправки кода (или отправлять письмо «учётка заблокирована»).

### `chargeRate` не атомарен: параллельные запросы кода обходят лимит
- Серьёзность: низкая
- Уверенность: предположение (нужен пробный тест)
- Где: `internal/auth/codes.go:322-332`
- Атакующий: аноним
- Что: `checkRate` (SELECT COUNT) и `INSERT INTO code_request_log` — два разных вызова без транзакции; N параллельных `POST /auth/code` на один адрес все видят счётчик < 5 и все проходят. Каждый проход — письмо жертве (спам кодами) и запись `DELETE/INSERT pending_codes`. Порядок величины обхода: десятки писем вместо 5/час.
- Как воспроизвести: 50 одновременных `POST /auth/code` с одним e-mail; посчитать письма/строки `code_request_log`.
- Исправление: `INSERT ... SELECT ... WHERE (SELECT COUNT(*) ...) < ?` с проверкой RowsAffected, или `BEGIN IMMEDIATE`.

### Блокировка не отзывает push-подписки: заблокированный продолжает получать сигналы активности кругов
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/auth/admin_access.go:71-100` (блокировка: только `sessions`); `internal/auth/accounts.go:162-243` (удаление: sessions, pending joins, личные инвайты — но не `push_subscriptions`); `internal/api/notify.go:194-230`; `internal/push/push.go:187-231`
- Атакующий: заблокированная/удалённая учётка
- Что: `SetAccountBlocked` и `DeleteAccount` не трогают `push_subscriptions`, а `notifyCircle` рассылает по `CircleMemberAccountIDs` без проверки `blocked`/`deleted_at`. Полезная нагрузка — только `{circle_id, type, count}`, содержимого нет, но заблокированный по-прежнему видит факт и частоту активности в кругах, из которых его формально не исключали («Circles are left untouched»). (Открытые SSE при блокировке рвутся: `syncSSE` перечитывает сессию перед каждым опросом, `internal/api/sync.go:89-94` — API-2 закрыт.)
- Как воспроизвести: подписать push, заблокировать учётку в панели, опубликовать пост в её круге → push приходит.
- Исправление: при блокировке/удалении `DELETE FROM push_subscriptions WHERE account_id = ?`; в `notifyCircle` фильтровать заблокированных.

### Инвайты, созданные удалённым/заблокированным участником, остаются рабочими
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/auth/accounts.go:236-241` (отзываются только `target_account_id = id`); `internal/auth/invites.go:213-231` (`validateInvite` не смотрит на создателя)
- Атакующий: удалённый/заблокированный участник, раздавший ссылки заранее
- Что: участник с `can_settings` создаёт multi-инвайт круга на 7 дней, затем его блокируют или удаляют; ссылка продолжает принимать новых участников в круг, пока владелец сам не найдёт и не отозвёт её в списке. Серверные инвайты создаются только админом, для них неактуально.
- Как воспроизвести: участник B (can_settings) → `POST /circles/{id}/invites`; админ блокирует B; аноним → `POST /invites/{token}/accept` → 202.
- Исправление: при блокировке/удалении отзывать `invites WHERE created_by_account_id = ?`; при исключении из круга (`exclude`) — тоже.

### Персональное приглашение нельзя отозвать: `pending_circle_joins` бессрочна и переживает отзыв/истечение ссылки
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/auth/member_invites.go:13-54` (инвайт на 30 дней + `insertPendingCircleJoin` без срока); `internal/auth/pending_join.go:54-91` (`CompleteCircleJoin` смотрит только на `pending_circle_joins`); `internal/auth/admin_access.go:179-198` (`RevokeInvite` не трогает pending); `internal/api/member_invites.go:137-181`
- Атакующий: приглашённый участник другого круга (после того как приглашение «передумали»)
- Что: `POST /circles/{id}/member-invites` сразу кладёт строку `pending_circle_joins(target, circle)`. Строка не истекает (30-дневный TTL есть только у сопутствующей ссылки, которой клиент не пользуется — токен даже не возвращается), не удаляется при `DELETE /circles/{id}/invites/{id}`, при исключении пригласившего или смене `invite_who` на `owner`. Приглашённый может вступить в круг (`POST /circles/{id}/join`) через любое время, даже если пригласивший давно исключён, а владелец переключил приглашения «только владелец». Единственная уборка — `DeleteAccount` цели. Также `GET /circles/{id}/join-preview` бессрочно отдаёт ему название круга и имена всех участников.
- Как воспроизвести: B зовёт X в круг T; владелец T исключает B и ставит `invite_who=owner`; через год X → `POST /circles/T/join {"name":"X"}` → 200.
- Исправление: хранить `expires_at`/`invite_id` в `pending_circle_joins`, чистить при отзыве ссылки, при исключении пригласившего и в рутине; дать владельцу список ожидающих с отзывом.

### TTL и `max_uses` инвайта круга не ограничены сверху
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/api/circles.go:120-127`; `internal/api/admin.go:234-241`
- Атакующий: участник с правом звать
- Что: `ttl_sec` и `max_uses` берутся из тела как есть; участник создаёт ссылку на 100 лет с миллионом использований, обходя настройку владельца «одноразовые 72 ч» (она ограничивает только `kind`, не срок/число). Рутина такие ссылки не убирает.
- Как воспроизвести: `POST /circles/{id}/invites {"kind":"single","ttl_sec":3153600000}`.
- Исправление: потолок TTL (например 30 дней) и `max_uses` (например 100) на сервере.

### Peek инвайта не проверяет остаток использований и режим `closed`
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/api/invites.go:44-94`
- Атакующий: аноним с токеном
- Что: `handlePeekInvite` отдаёт название круга, цвет и имена всех активных участников по любой неотозванной и неистёкшей ссылке, в том числе полностью использованной (`uses >= max_uses`) и при `registration_mode=closed`. Одноразовая ссылка, уже потраченная, продолжает раскрывать состав круга до истечения TTL. Токен — 192 бита случайности, перебор невозможен; риск — только повторное чтение уже засвеченной ссылки.
- Исправление: в peek применять `validateInvite` целиком.

### Исключённый может вернуться в круг по любой живой ссылке
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/chronicle/member.go:42-48` (`JoinInTx` → `rejoinTx` для `gone`); `internal/auth/invites.go:70-119`
- Атакующий: исключённый участник
- Что: исключение — статус `gone`, не запрет; `AcceptInvite`/`Verify`/`CompleteCircleJoin` не отличают исключённого от новичка. Многоразовая ссылка, которой исключённый уже пользовался, или ссылка из общего чата возвращает его в круг. По справочнику «повторный вход — новый интервал», т.е. поведение осознанное; фиксирую как заметку — владельцу стоит показывать подсказку отозвать живые ссылки при исключении.

### `PUT /probe/body` принимает от анонима до 100 МиБ на запрос, а не 2 МиБ, как заявлено
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/api/probe.go:16-28` (объявленный лимит `min(2 MiB, attachment_max)`), `internal/api/probe.go:88-101` (реальный `MaxBytesReader` = `attachment_max`, по умолчанию 104857600); `internal/api/probe_limit.go:9-16` (комментарий DEP-5: «до 2 МБ»)
- Атакующий: аноним
- Что: `handleProbeBody` ограничивает тело `cs.AttachmentMaxBytes` (100 МиБ по умолчанию, админ может поднять), а не `bodyProbeBytes`. С лимитером 12/мин/IP аноним заливает в `io.Discard` 1,2 ГБ/мин с одного адреса; при отсутствии `ReadTimeout` у `http.Server` (`cmd/wynd/main.go:153-155`, намеренно ради загрузок/SSE) один запрос может капать байтами бесконечно, удерживая goroutine и соединение; лимитер считает только старты запросов, так что за час с одного IP — 720 висящих соединений. Диск и БД не трогаются; вред — полоса и дескрипторы. Прокси с `client_body_timeout` частично прикрывает.
- Как воспроизвести: `curl -T /dev/zero -H 'Expect:' --limit-rate 1k PUT /api/v1/probe/body` × 12 — соединения живут, пока не набегут 100 МиБ.
- Исправление: `limit = s.bodyProbeBytes(ctx)` в `handleProbeBody`; для публичных маршрутов — `http.TimeoutHandler`/`ResponseController.SetReadDeadline` (например, 30 с) на всё, кроме чанков загрузки и SSE.

### `X-Forwarded-For` читается только из первой строки заголовка
- Серьёзность: низкая
- Уверенность: предположение (зависит от прокси)
- Где: `internal/api/respond.go:156-169`
- Атакующий: аноним за доверенным прокси, который добавляет XFF отдельной строкой (HAProxy `option forwardfor` без `if-none`, Apache при некоторых настройках)
- Что: `r.Header.Get` возвращает первую строку `X-Forwarded-For`; если клиент прислал свою, а прокси добавил вторую строку с настоящим адресом, сервер видит только подделку и берёт её крайний правый адрес как ключ лимитера — клиент выбирает себе корзину и обходит лимиты `/auth/*`, `/admin/login`, `/probe/*`. В комплектных `deploy/proxy/{nginx.conf,Caddyfile,traefik.yaml}` прокси перезаписывает/склеивает заголовок, поэтому с ними атака не проходит; но `WYND_TRUSTED_PROXIES` допускает любой прокси. Также в качестве ключа принимается любая строка (не только IP): при прокси, добавляющем порт, каждый запрос получал бы отдельную корзину.
- Как воспроизвести: HAProxy с `option forwardfor` → два заголовка `X-Forwarded-For` → 20 × `POST /auth/code` с разными первым заголовком — 429 не наступает.
- Исправление: `strings.Join(r.Header.Values("X-Forwarded-For"), ",")`; каждый кандидат прогонять через `net.ParseIP` (с отрезанием порта/скобок), непарсящиеся отбрасывать.

### Ошибка в `public_url` при bootstrap обнаруживается после того, как установка уже зафиксирована
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/api/admin.go:63-72,112-118`; `internal/config/public_url.go:14-23,54-58`
- Атакующий: нет (надёжность установки)
- Что: перед транзакцией адрес только нормализуется (`NormalizePublicURL`), решение «нужен ли SMTP» принимается по нему, а `ValidatePublicURL` выполняется внутри `WritePublicURL` уже после `Auth.Bootstrap`. Адрес вида `https://host/path` или `ftp://…` даёт ответ 500 при уже созданном админе и `bootstrapped=1`; повтор — `400 invalid`. Установщик видит ошибку, хотя пароль уже действует. Порядок из раздела B плана 42 («→ `WritePublicURL` → `SendTest`») формально соблюдён, но валидацию стоит вынести в начало.
- Исправление: `config.ValidatePublicURL` до `ConfirmBootstrapToken`-проверок пароля; в ответ 400.

### `auth.Service.loopback` пишется из обработчика без синхронизации
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/auth/auth.go:68,97-100,116`; `internal/api/admin.go:168-170`
- Что: QLT-4 закрыт для `Server` и `mail.Service` (atomic), но `auth.Service.SetLoopback` пишет обычное поле, которое читает `Instance()` из каждого `GET /instance`. Гонка данных без последствий для безопасности; отмечаю для полноты.

### `code_request_log` без индекса по `email`
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/auth/codes.go:340-344`; `internal/store/migrations/0001_schema.sql:325` (индекс только `(client_ip, requested_at)`); `internal/jobs/routine.go:18,43,81-83` (чистка строк старше 24 ч)
- Что: `COUNT(*) ... WHERE email = ?` — полный проход таблицы на каждый публичный `POST /auth/{register,code}` и `/invites/{token}/accept`. Таблица чистится раз в сутки, так что аноним с 1 000 адресов даёт до 240 000 строк, и каждый следующий запрос кода читает их все. Самоусиливающееся замедление, не отказ.
- Исправление: индекс `(email, requested_at)`.

## Проверено, в порядке

**Публичные маршруты (`internal/api/server.go:64-77`).**
- `GET /instance` (`instance.go`) — имя, версия, режим регистрации, `bootstrapped`, `code_delivery`, публичный VAPID-ключ, настройки сжатия. Ничего секретного; в БД не пишет.
- `GET /probe`, `GET /probe/sse` — без лимитера, но обработчики возвращаются немедленно (один JSON / один SSE-кадр), соединение не удерживается. `/probe/stream` (2 с) и `/probe/body` — под `limitProbe` 12/мин/IP (`probe_limit.go`), карта ключей выметается при >1024 записей. Отражённые заголовки `X-Forwarded-For`/`X-Real-IP` — диагностика 9.8, чужого не раскрывают.
- `POST /auth/register|code` — лимит 10/IP/час и 5/e-mail/час **до** поиска учётки (AUTH-2 закрыт, `codes.go:32,75`), `limitBody` 1 МиБ, `ParseParticipantEmail` через `net/mail`. Sentinel `admin@wynd.local` отвергается во всех трёх входах (`Register`, `RequestCode`, `AcceptInvite`, `Verify`), сравнение после нормализации регистра. 404 на неизвестный адрес в режиме invite — принято (раздел B).
- `POST /auth/verify` — код 6 цифр из `crypto/rand`, хеш `sha256(id:code)`, 3 попытки через атомарный `UPDATE ... WHERE attempts < 3` (`claimAttempt`), TTL 15 мин, один живой код на адрес (`DELETE FROM pending_codes WHERE email`), `subtle.ConstantTimeCompare`, расход `DELETE` с `RowsAffected` внутри транзакции с созданием сессии (AUTH-1 закрыт, `codes.go:271-281`). Личный инвайт при Verify повторно сверяется с `TargetAccountID` (`codes.go:237`).
- `GET /invites/{token}` — токен 24 байта `crypto/rand` (192 бита), поиск по `UNIQUE` индексу; перебор невозможен. Отдаёт только имена (без `account_id`, e-mail, аватаров) — `invite_peek.go:11-16`. `POST /invites/{token}/accept` — `validateInvite` (режим closed, revoked, expiry, `uses < max_uses`) → лимит кодов → проверка `TargetAccountID` **до** отправки письма (AUTH-4 закрыт, `invites.go:103-110`). Расход `UPDATE ... WHERE uses < max_uses` с `RowsAffected` (`consumeInvite`).
- `POST /admin/bootstrap` — `ConfirmBootstrapToken`: лимитер по IP → константное сравнение 256-битного токена из `keys/bootstrap` (0600, никогда не пустой — `config/bootstrap.go`) → флаг `bootstrapped`; пароль ≥ 8 рун; `mail.Probe` несохранённого релея; одна транзакция с `UPDATE ... WHERE bootstrapped = 0` + `RowsAffected` (SEC-1/SEC-1a/SEC-1b закрыты, `auth/admin.go:93-121`); `SendTest` после коммита не откатывает. Тест `TestBootstrapConcurrentOnlyOneSucceeds`.
- `POST /admin/bootstrap/smtp-test` — тот же `ConfirmBootstrapToken`: на установленном инстансе — `400 invalid` независимо от токена (`auth/admin.go:53-55`); до установки нужен токен. Открытого релея/SSRF для анонима нет; с токеном — это уже установщик.
- `POST /admin/login` — bcrypt `DefaultCost`, лимитер 5 неудач/15 мин на IP с lockout 15 мин, сброс при успехе; до bootstrap — `403` без утечки. Пустой пароль — `invalid` без обращения к лимитеру.

**Сессии (`internal/auth/sessions.go`).** 32 байта `crypto/rand`, в БД `sha256(token)` (AUTH-4/0010 закрыты), участник 30 дней, админ 8 ч, без продления; выборка по `token_hash AND kind` — участнический токен не проходит `requireAdmin`, админский отвергается `IsParticipantSession` и повторно `RejectAdminJournal`. `IsParticipantSession` проверяет `blocked` и `deleted_at` при каждом запросе. `bearerToken` — только заголовок `Authorization`, `?token=` нигде не читается (grep по `Query().Get` в `internal/api`). Смена и сброс пароля админа удаляют все админские сессии в той же транзакции (AUTH-3, `storeAdminPassword`). SSE перечитывает сессию и подписку перед каждым опросом (API-2, `sync.go:89-94`). Просроченные сессии и коды убирает рутина.

**Стрим-варианты шлюзов (`internal/api/admin.go:265-361`).** `RequireParticipant` = `limitBody(RequireParticipantStream)`; `RequirePaidParticipantStream` дублирует логику `RequireParticipantStream` + `requirePaidSession` без `limitBody` — единственный потребитель `PUT /uploads/{id}` ограничивает тело сам. Проверки идентичны, расхождений нет.

**Блокировка/удаление (`auth/admin_access.go:71-100`, `auth/accounts.go:162-243`).** Блокировка удаляет участнические сессии; `Verify` заблокированного — `forbidden`. Soft-delete: `LeaveInTx` по всем активным кругам, `identities.account_id = NULL`, e-mail заменяется, сессии, `pending_circle_joins` и личные инвайты на учётку — в одной транзакции (AUTH-4 закрыт). Владелец круга не удаляется (409), sentinel — 404.

**Инвайты.** Серверный инвайт создаёт только админ; `ListCircleInvites` не показывает личные ссылки; `RevokeCircleInvite` проверяет принадлежность кругу. Личный инвайт: токен не возвращается клиенту, `AcceptInvite`/`Verify` требуют совпадения `TargetAccountID` — участник A не может использовать ссылку B. `CreateMemberInvite` отзывает прежние живые личные ссылки на ту же пару. `invite-candidates` и `member-invites` — под `RequireCanInvite` (`invite_settings.go:74-90`, учитывает `invite_who=owner`); кандидаты — только `account_id` и имя из общего круга, e-mail не раскрывается. `join-preview` — только при своей строке `pending_circle_joins`. `MemberInviteTargets` вызывается до проверки прав, но результат наружу не уходит.

**Админ.** `requireAdmin` — токен + `IsAdminSession` (kind, срок). `PUT /admin/access`: режим валидируется (`SetRegistrationMode`), `public_url` — `WritePublicURL` → `ValidatePublicURL` (http/https, хост, без пути/запроса/учётных данных; API-5 закрыт); temp+rename. Аноним `public_url` не задаёт, подмены ссылок в письмах нет.

**`clientIP`/`trustedProxies` (`respond.go:88-182`).** Пустой список = только loopback; XFF учитывается только от доверенного пира; берётся крайний правый недоверенный адрес; все доверенные → первый; `RemoteAddr` без порта и `[::1]` без порта обрабатываются; IPv6 с зоной не парсится → считается недоверенным и используется как есть (ключ лимитера, безвредно). Тесты `TestForwardedForIgnoredFromUntrustedPeer`, `TestForwardedForRightmostUntrusted`.

**Коды в лог (`auth.go:27-60`, `mail.go:150-169`).** `LogCodes` используется только когда `loopback && !configured`; loopback выводится из `public_url` (`localhost`, `127.x`, `::1`) и меняется только админом/bootstrap. `File` в `cmd/wynd/main.go:79` пуст — только stdout. `CaptureCodes` — тестовый тип, в бинарнике не подключён.

**Покрытие тестами (`security_test.go`, `invites_test.go`, `member_invites_test.go`, `admin_test.go`).** Есть: XFF, лимиты по IP и e-mail, `Retry-After`, отсутствие перебора в open, потолок попыток verify, lockout админа, JSON-cap, logout, слабый пароль bootstrap, конкурентный bootstrap, smtp-test до/после, peek и отложенное вступление, single-only, список/отзыв ссылок, личный инвайт из другого круга. Нет тестов на: сгорание чужих попыток verify, глобальный lockout `*`, `probe/body` лимит, бессрочность `pending_circle_joins`, инвайты заблокированного создателя, push после блокировки.
