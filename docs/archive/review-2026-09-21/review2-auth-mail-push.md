# Ревью 2: auth / mail / push / check / proxy

Ревью «только чтение», репозиторий не менялся. Ссылки — `путь:строка`.
CONFIRMED = однозначно следует из кода; PLAUSIBLE = требует проверки (дан регрессионный тест).

---

## Коды входа и сессии

### 1. Оракул перечисления аккаунтов не ограничен rate-limit (medium, CONFIRMED)
`internal/auth/codes.go:73-92`. В `RequestCode` порядок такой: сначала
`accountByEmail`, и при `ErrNotFound` в режиме `invite`/`closed` сразу
`return err` (→ 404), и только для существующих адресов вызывается
`s.checkRate`. То есть «промах» не пишет строку в `code_request_log` и не
расходует лимит. Атакующий может перебирать адреса бесконечно с одного IP:
404 = адреса нет, 200/429 = адрес есть. Сам факт 404 в invite-режиме принят
аудитом как осознанный, но отсутствие лимита на перебор — нет.

Одно исправление: перенести `s.checkRate(ctx, in.ClientIP, email, when)` вверх,
сразу после нормализации адреса (строка 72), до `accountByEmail`, чтобы
неудачные попытки тоже били по IP-счётчику. Счётчик по e-mail при этом
по-прежнему защищает чужой ящик от флуда.

### 2. Код одноразовый только «по факту», а не атомарно (medium, CONFIRMED)
`internal/auth/codes.go:183-192, 269-271`. `claimAttempt` атомарно занимает
попытку (`UPDATE … WHERE attempts < 3` + `RowsAffected` — сделано правильно),
но сам *расход* кода — это `DELETE FROM pending_codes WHERE id = ?` внутри tx
без проверки `RowsAffected`. Два параллельных `Verify` с одним верным кодом оба
проходят `claimAttempt` (1 и 2 < 3), оба доходят до коммита и оба получают
сессию. Для `FlowRegister` вторая транзакция, скорее всего, упадёт на UNIQUE по
`accounts.email`, а вот для `FlowLogin` и `FlowInvite` — нет: выдаются две
сессии по одному коду, а для `FlowInvite` инвайт спасает только атомарный
`consumeInvite`.

Одно исправление: заменить `DELETE … WHERE id = ?` на `DELETE FROM pending_codes
WHERE id = ? AND code_hash = ?` (или просто проверять `RowsAffected` у текущего
DELETE) и при `n == 0` возвращать `ErrInvalid` с откатом транзакции — тогда
второй параллельный Verify гарантированно откатится.

### 3. Хеш кода — несолёный SHA-256 от 6 цифр (low, CONFIRMED)
`internal/auth/codes.go:386-389`. `hashCode` = `sha256(code)` без соли и без
привязки к e-mail/id. Пространство — 10^6, радужная таблица строится за
миллисекунды, так что хранение «в хеше» здесь эквивалентно хранению открытым
текстом. Само по себе это не дыра (доступ к БД уже означает компрометацию), но
формулировка «code_hash» вводит в заблуждение.

Одно исправление: хешировать `sha256(pendingID + ":" + code)` — соль уже есть в
строке (`id`), менять схему не нужно, только `hashCode` получает второй аргумент.

### 4. Генерация кода — корректна (ОК)
`internal/auth/codes.go:378-384`: `crypto/rand.Int(rand.Reader, big.NewInt(1e6))`
+ `%06d` — равномерно, без modulo bias, ведущие нули сохраняются. TTL 15 мин
(`auth.go:18`), 3 попытки (`auth.go:19`), сравнение через
`subtle.ConstantTimeCompare` (`codes.go:190`). Всё правильно.

### 5. Окна rate-limit и `retry_after_sec` (ОК с оговоркой, low)
`internal/auth/auth.go:22-24`: `ipRateLimit=10`, `emailRateLimit=5`,
`ipRateLimitWindow=1h`. Отдельной константы `emailRateLimitWindow` нет — окно
для e-mail берётся из `ipRateLimitWindow` (`codes.go:314`, `codes.go:369`), и
математика `retry` (`codes.go:356-376`: `MIN(requested_at) + window - now`,
округление вверх, минимум 1) поэтому верна. Замечание — именование: константа
с префиксом `ip` используется как общее окно; стоит переименовать в
`codeRateWindow`, иначе при разведении окон математика тихо разъедется.

### 6. Токены сессий хранятся открытым текстом (medium, CONFIRMED)
`internal/auth/sessions.go:12-18, 34-39`. Энтропия хорошая — 32 случайных байта
из `crypto/rand`, hex. Но в таблицу `sessions` кладётся сам токен, а не его
хеш, и поиск идёт `WHERE token = ?`. Любая утечка файла БД или бэкапа = готовый
Bearer для всех живых сессий на 30 дней (`auth.go:20`). Это не пересмотр
решения «Bearer-сессии» — только хранение.

Одно исправление: хранить `sha256(token)` в колонке `token` (индекс тот же,
поиск тот же `WHERE token = ?` от хеша), сам токен возвращать клиенту один раз
при создании.

### 7. Блокировка аккаунта не убивает сессии, но проверяется на каждом запросе (ОК)
`internal/auth/sessions.go:82-95`: `IsParticipantSession` на каждом запросе
подтягивает аккаунт и отдаёт `ErrForbidden` при `Blocked`. Строки в `sessions`
остаются, но использовать их нельзя. Soft-delete (`accounts.go:219-228`) в той
же транзакции и переименовывает e-mail в `deleted+<id>@wynd.local`, и удаляет
`sessions … kind = 'participant'` — это правильно.

Разделение админских и участниковых сессий сделано верно: `SessionByToken`
фильтрует `AND kind = ?` (`sessions.go:53-54`), так что участниковый токен не
подойдёт к админским ручкам и наоборот.

### 8. Смена админского пароля не отзывает админские сессии (medium, PLAUSIBLE)
`internal/auth/sessions.go:77-80` — единственный способ удалить сессию — по
токену; массового `DELETE … WHERE kind='admin'` в пакете нет (проверено
чтением). Плюс установленный ведущим факт: клиент никогда не вызывает
`POST /admin/logout`. Значит при смене пароля админа старая 8-часовая сессия
(`auth.go:21`) продолжает жить.
Регрессионный тест: сменить пароль админа, затем выполнить админский запрос со
старым Bearer — ожидать 401.
Одно исправление: в обработчике смены пароля выполнить
`DELETE FROM sessions WHERE kind = 'admin'` в той же транзакции, что и UPDATE
хеша пароля.

### 9. Нормализация e-mail: ToLower без Unicode-складывания (low, CONFIRMED)
`internal/auth/accounts.go:11-13` — `strings.ToLower(strings.TrimSpace(...))`.
Для ASCII достаточно. Но `Verify` (`codes.go:149`) применяет только
`normalizeEmail`, а `Register`/`RequestCode`/`AcceptInvite` — полноценный
`ParseParticipantEmail` (`email.go:11-26`). Асимметрия не эксплуатируется (в
`Verify` идёт точное сравнение с уже сохранённой строкой), но для турецкого `İ`
(U+0130) `ToLower` даёт `i` + комбинирующая точка, то есть адрес, отличный от
`i`-варианта, — два разных аккаунта на визуально один адрес.
Одно исправление: в `normalizeEmail` понижать регистр только у ASCII-байтов
домена и локальной части (`strings.Map` с проверкой `< utf8.RuneSelf`),
оставляя не-ASCII как есть.

---

## Приглашения

### 10. Расход приглашения — атомарен (ОК)
`internal/auth/invites.go:227-240`: `UPDATE invites SET uses = uses + 1 WHERE id = ?
AND uses < max_uses` + проверка `RowsAffected`, при нуле — `ErrInvalid`.
Это правильный паттерн, гонка двух параллельных погашений закрыта. Энтропия
токена — 24 случайных байта (`invites.go:243-249`), 192 бита, достаточно.
Отзыв — `revoked_at`, проверяется в `validateInvite` (`invites.go:211-213`),
срок — `!when.Before(inv.ExpiresAt)` (строгое сравнение, верно).

### 11. `consumeMemberInvite` — мёртвый код, который всегда возвращает nil (medium, CONFIRMED)
`internal/auth/member_invites.go:177-194`. Обе ветки (`n == 0` и `n != 0`)
возвращают `nil`, то есть функция не может сообщить, что живого персонального
приглашения не было. Вызовов у неё в пакете `auth` нет — проверено Grep по
`consumeMemberInvite`: единственное вхождение — само определение. Функция
неэкспортируемая, значит это мёртвый код, и при этом он выглядит как охрана
(«погасить персональный инвайт»), которой на деле нет.
Одно исправление: удалить `consumeMemberInvite` целиком — погашение уже делает
`consumeInvite` в `codes.go:240`.

### 12. Чужой персональный инвайт «съедает» лимит писем жертвы (low, CONFIRMED)
`internal/auth/invites.go:70-108`. `AcceptInvite` проверяет только срок, отзыв и
`uses < max_uses` (`validateInvite`), но не сверяет `inv.TargetAccountID` с
адресом заявителя. Привязка проверяется позже — в `Verify`
(`codes.go:237-239`, `inv.TargetAccountID != acc.ID → ErrForbidden`), и это
действительно закрывает захват круга: чужой аккаунт инвайт не погасит.
Но до этого `AcceptInvite` уже успевает выдать код и записать строку в
`code_request_log` на *чужой* адрес: имея утёкшую персональную ссылку, можно
жечь квоту `emailRateLimit=5/ч` произвольного ящика и слать на него письма.
Одно исправление: в `AcceptInvite` после `accountByEmail` добавить
`if inv.TargetAccountID != "" && (err == ErrNotFound || acc.ID != inv.TargetAccountID) { return ErrForbidden }`
— отказ до `issueCode`.

### 13. Soft-delete не чистит `pending_circle_joins` и персональные инвайты (medium, CONFIRMED)
`internal/auth/accounts.go:199-229`. Транзакция удаления аккаунта делает
`LeaveInTx` по кругам, обнуляет `identities.account_id`, переименовывает
e-mail, ставит `deleted_at`/`blocked`, удаляет участниковые сессии — но строки
`pending_circle_joins` с этим `account_id` и живые `invites` с
`target_account_id = <id>` остаются. `MemberInviteTargets`
(`member_invites.go:113-150`) после этого продолжает отдавать id удалённого
аккаунта, то есть в админке круга участник висит как «приглашён» навсегда.
Одно исправление: добавить в ту же транзакцию
`DELETE FROM pending_circle_joins WHERE account_id = ?` и
`UPDATE invites SET revoked_at = ? WHERE target_account_id = ? AND revoked_at IS NULL`.

### 14. `DeleteAccount`: проверка `chronicle == nil` после `BeginTx` (low, CONFIRMED)
`internal/auth/accounts.go:199-206` — транзакция открывается, и только потом
проверяется `s.chronicle == nil`. Работает (сработает `defer Rollback`), но
порядок неверный: дешёвая проверка предусловия должна быть до открытия
транзакции, как это сделано в `codes.go:244`.
Одно исправление: перенести проверку `s.chronicle == nil` выше `BeginTx`.

---

## Push

### 15. SSRF: endpoint подписки не валидируется вообще (high, CONFIRMED)
`internal/push/push.go:101-107` — `Subscribe` только делает `TrimSpace` и
проверяет непустоту `Endpoint`; ни схема, ни хост не проверяются. Дальше
`deliver` (`push.go:241-265`) отправляет на этот URL POST через
`webpush.SendNotificationWithContext`. Ни в `push.go`, ни в
`internal/api/notify.go:281-300` (обработчик `handlePushSubscribe`) нет ни
требования `https`, ни отсечения приватных/loopback/link-local адресов, ни
списка разрешённых push-сервисов.

Следствие: любой аутентифицированный участник регистрирует подписку с
`http://127.0.0.1:<порт>/...` или `http://169.254.169.254/...`, а затем любое
уведомление заставляет сервер сходить туда изнутри периметра. Тело ответа
выбрасывается (`push.go:260`), то есть SSRF «слепой», но канал утечки всё
равно есть: 404/410 приводит к удалению строки подписки (`push.go:220-222`), а
прочие коды — нет. Это готовый однобитный оракул для сканирования внутренних
портов, причём результат виден атакующему через список собственных подписок.

Одно исправление: в `Subscribe`, до записи в БД, распарсить endpoint
`url.Parse` и отклонять всё, кроме `scheme == "https"` с хостом, который
резолвится в публичный адрес (`netip.Addr.IsPrivate() || IsLoopback() ||
IsLinkLocalUnicast() || IsUnspecified()` → `ErrInvalid`).

### 16. `http.DefaultClient`: нет таймаута и разрешены редиректы (medium, CONFIRMED)
`internal/push/push.go:58` — `&Service{db: db, client: http.DefaultClient}`.
У `http.DefaultClient` нулевой `Timeout`, так что единственная граница —
`ctx` вызывающего; в `internal/jobs/pay.go:52` и `internal/api/notify.go` это
контекст запроса или фона, отдельного дедлайна на доставку нет. Плюс
`DefaultClient` по умолчанию идёт по редиректам (до 10), так что даже после
исправления №15 endpoint на публичном хосте сможет увести запрос на
`127.0.0.1` через `302`.
Одно исправление: завести собственный клиент
`&http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}`
и передавать его в `Service.client`.

### 17. «Push hijack» из `security_test.go` — что именно починено (ОК)
`internal/push/push.go:124-130`. Ветка `err == nil && existingAccount !=
in.AccountID && (existingP256dh != in.P256dh || existingAuth != in.Auth)`
возвращает `ErrForbidden`. Смысл: строка в `push_subscriptions` уникальна по
`endpoint`, и раньше повторная подписка чужим аккаунтом на уже известный
endpoint просто переписывала `account_id`, то есть уводила чужие уведомления
себе. Теперь перехват возможен только тому, кто знает `p256dh` и `auth`, а их
знает лишь браузер, создавший подписку. Комментарий на месте и объясняет
именно это — сделано правильно.

Обратная сторона (low): первый, кто зарегистрировал endpoint, им владеет.
Атакующий, знающий чужой endpoint URL, но не ключи, может занять строку своими
поддельными ключами, и тогда настоящий владелец получит `ErrForbidden` и не
сможет подписаться. Это DoS на одну подписку, не перехват.

### 18. Содержимое записей в push не утекает провайдеру (ОК)
`internal/push/push.go:24-30, 197-200, 249-255`. Полезная нагрузка — `Signal`
c `circle_id`/`type`/`count` и необязательными `title`/`body`; она шифруется
библиотекой `webpush-go` на ключах `p256dh`/`auth` подписки (aes128gcm), то
есть push-сервис видит только шифротекст. Тексты записей в `Signal` не
попадают: `internal/api/notify.go` передаёт счётчики и типы. Решение «клиент
сам дочитывает подробности» реализовано последовательно.

### 19. `Subscriber: auth.AdminSentinelEmail` — не URI и лишняя зависимость (medium, PLAUSIBLE)
`internal/push/push.go:251`. В VAPID-заголовок в качестве claim `sub`
подставляется `admin@wynd.local` — голый адрес, тогда как RFC 8292 требует
`mailto:` или `https:` URI. Строгие push-сервисы (в частности Apple Web Push)
отвечают на такой `sub` кодом 400, и подписка будет молча отваливаться —
причём 400 не попадает в `isStaleSubscription`, так что строка останется в БД
и будет ломаться вечно.
Побочно: ради одной строковой константы пакет `push` импортирует
`internal/auth` (`push.go:15`) — зависимость не туда, `push` не должен ничего
знать про аккаунты.
Регрессионный тест: поднять `httptest`-сервер как push-эндпоинт, вызвать
`deliver` и проверить, что claim `sub` в JWT заголовка `Authorization`
начинается с `mailto:`.
Одно исправление: ввести в `push` собственную константу
`vapidSubscriber = "mailto:admin@wynd.local"` и убрать импорт `internal/auth`.

### 20. VAPID-приватный ключ лежит в БД открытым текстом (low, документируемый риск)
`internal/push/push.go:79-84, 267-275` — `vapid_private_key` пишется и читается
из `instance_settings` как есть. Вместе с SMTP-паролем (см. №25) это значит,
что любой бэкап SQLite содержит оба секрета. Для домашнего инстанса это
приемлемо, но должно быть явно записано в «Ключевые решения» как принятый риск
с требованием к правам на файл БД, а не оставаться умолчанием.

### 21. `EnsureKeys` перезаписывает ключи без охраны в `WHERE` (low, CONFIRMED)
`internal/push/push.go:67-86`: сначала `loadKeys`, потом `UPDATE
instance_settings SET vapid_public_key = ?, vapid_private_key = ? WHERE id = 1`
— без `AND (vapid_private_key IS NULL OR vapid_private_key = '')`. Проверка и
запись разнесены, между ними нет транзакции. Для одного процесса это не
стреляет, но паттерн ровно тот же, что уже дал проблему в `auth.Bootstrap`
(проверка флага вне транзакции, UPDATE без охраны в `WHERE`).
Одно исправление: дописать в `UPDATE` условие
`AND COALESCE(vapid_private_key, '') = ''` и считать `RowsAffected` признаком
того, что ключи сгенерированы именно этим вызовом.

---

## Почта

### 22. STARTTLS необязателен: письмо с кодом входа уходит открытым текстом (high, CONFIRMED)
`internal/mail/mail.go:289-296`:
```go
if !useImplicitTLS(port) {
    if ok, _ := client.Extension("STARTTLS"); ok {
        if err := client.StartTLS(...); err != nil { ... }
    }
}
```
Если сервер не анонсировал STARTTLS в ответе на EHLO — код молча продолжает
разговор по открытому каналу и отправляет письмо. Это классический
STARTTLS-stripping: активный посредник вырезает строку `250-STARTTLS` из
EHLO, и Wynd без единого предупреждения отдаёт в открытом виде тело письма,
то есть шестизначный код входа.

Учётные данные при этом защищены — и `smtp.PlainAuth`, и собственный
`loginAuth` (`internal/mail/login.go:13-18`) отказываются стартовать при
`!server.TLS` вне localhost. Но это спасает только пароль SMTP, а не код
входа, и только когда `Username` вообще задан: при анонимном релее
(`authenticate` выходит на `mail.go:305-307`) не срабатывает ничего.

Одно исправление: после блока STARTTLS проверить фактическое состояние
соединения и прервать отправку, если шифрования нет:
`if _, isTLS := client.TLSConnectionState(); !isTLS && !localhost(host) {
return fmt.Errorf("%w: mail: STARTTLS required", ErrSend) }`.

### 23. `encodeHeader` пропускает CR/LF в ASCII-заголовке (medium, PLAUSIBLE)
`internal/mail/mail.go:354-361`: если все руны `<= 127`, строка возвращается
как есть. CR (13) и LF (10) — ASCII, поэтому тема вида
`"Wynd\r\nBcc: victim@example.com"` попала бы в заголовки письма дословно;
`buildMessage` (`mail.go:382-383`) вставляет результат в `Subject:` без
дополнительной проверки, а `textproto.DotWriter` делает только dot-stuffing и
CRLF не режет.

Сегодня это не эксплуатируется: все темы — литералы в коде
(`mail.go:146,164`, `archive.go:17,33`, `pay.go:16`), а пользовательские
данные (`instanceName`, даты, ссылки) идут только в тело. То есть находка
латентная: первая же тема вида `"Wynd: " + circleName` открывает инъекцию.

Заголовок `To:` (`mail.go:380-381`) пишется вообще без обработки, но здесь
спасает стандартная библиотека: `smtp.Client.Mail`/`Rcpt` вызывают
`validateLine` и падают на CR/LF ещё до `DATA`, так что письмо не уходит.
Защита случайная — она держится на том, что конверт формируется из той же
строки. `From:` собран правильно, через `netmail.Address.String()`
(`mail.go:344-352`), который сам квотирует и кодирует display-name.

Регрессионный тест: вызвать `buildMessage("a@b.c", "d@e.f", "X\r\nBcc: z@z.z",
"body")` и проверить, что в результате нет подстроки `Bcc:`.
Одно исправление: в начале `encodeHeader` заменить
`strings.NewReplacer("\r", " ", "\n", " ")` по строке до всех остальных
проверок — тогда и ASCII-, и QEncoding-ветка безопасны.

### 24. TLS-конфигурация и 15-секундный дедлайн (ОК)
`internal/mail/mail.go:277, 291` — `&tls.Config{ServerName: host}`:
`InsecureSkipVerify` нигде не выставляется (проверено Grep по пакету),
`ServerName` задан из настроенного хоста, `MinVersion` не указан, то есть
берётся клиентское умолчание Go — TLS 1.2. Порт 465 корректно распознаётся как
implicit TLS (`mail.go:328-330`) и в этом случае STARTTLS не пробуется.
Дедлайн: `context.WithTimeout(ctx, sendTimeout)` в `sendMessage`
(`mail.go:224`) и в `Probe` (`mail.go:210`) плюс `conn.SetDeadline(deadline)`
(`mail.go:273-275`) — покрыт и сам диалог, а не только установка соединения.
Это сделано аккуратно.

### 25. SMTP-пароль в `instance_settings` открытым текстом (low, документируемый риск)
`internal/mail/mail.go:82-84` (чтение) и `mail.go:106-110` (запись) — колонка
`smtp_password` хранится как есть. Значит пароль от почтового релея лежит в
каждом бэкапе БД и виден любому, кто получил файл. Шифровать его в рамках 0.x
смысла мало (ключ пришлось бы держать рядом), но это ровно тот случай, который
надо один раз записать в «Ключевые решения» как осознанно принятый риск —
сейчас решения по нему в справочнике нет.

### 26. Логирование кода на loopback: права верные, но флаг гонится (medium, CONFIRMED)
Файловая часть сделана правильно: `internal/auth/auth.go:48-51` — каталог
`0o700`, файл `0o600`, `O_APPEND`. Строго ли невозможен этот путь вне
loopback: `SendCode` уходит в fallback только при
`!configured(cfg) && s.loopback` (`mail.go:140-145`), и на не-loopback
инстансе вернётся `ErrNotConfigured` — логика верна.
Но `s.loopback` меняется через `SetLoopback` (`mail.go:68-70`) без
синхронизации при живом сервере, так что запрос, выполняющийся в момент
переключения, может прочитать устаревшее `true` и записать код в лог вместо
отправки. Это та же несинхронизированная запись, что уже зафиксирована
ведущим.
Плюс отдельная мелочь: `mail.New` по умолчанию подставляет
`auth.LogCodes{Logger: log.Default()}` с пустым `File` (`mail.go:56-58`), то
есть коды идут в общий stdout/journald сервера, а не в файл с правами 0600.
Одно исправление: заменить поле `loopback bool` на `atomic.Bool`, а
`SetLoopback`/чтения — на `Store`/`Load`.

### 27. `archive.go` печатает тело письма в общий лог (low, CONFIRMED)
`internal/mail/archive.go:49-52` — на loopback без SMTP `notify` делает
`log.Printf("wynd mail (loopback): to=%s subject=%q\n%s", ...)`, то есть адрес
и полный текст письма попадают в stdout. Здесь это архивные уведомления со
ссылкой на персональный архив — ссылка в логе живёт дольше, чем письмо.
Одно исправление: логировать только `to` и `subject`, тело опускать.
