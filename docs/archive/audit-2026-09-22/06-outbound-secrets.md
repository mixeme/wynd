# 06 — Исходящие запросы, секреты, конфигурация, деплой (SEC-6)

Аудит безопасности Wynd 0.7.0, 2026-09-22. Область: каждый исходящий запрос сервера, секреты на диске/в БД/в логах, конфигурация, деплой.

Статус: просмотр завершён.

## 1. Таблица исходящих запросов сервера

Полный перечень получен grep-ом `http.Client|net.Dial|DialContext|tls.Dial|LookupIPAddr|smtp.` по `internal/`, `cmd/` (без тестов) — других мест нет.

| Место | Кто задаёт адрес | Таймаут | Редиректы | TLS / проверка серт. | Фильтр приватных адресов | Размер читаемого ответа | Что показывается вызывающему |
|---|---|---|---|---|---|---|---|
| SMTP: `mail.go:282-331 smtpClient` (`SendCode`, `SendTest`, `SendPlain`, `notify`, `Probe`) | админ (настройки) / держатель bootstrap-токена (`admin.go:87,149`) | 15 с ctx + `SetDeadline` | н/п | 465 — implicit TLS; иначе STARTTLS **обязателен** кроме loopback-хоста; `ServerName=host`, без `InsecureSkipVerify` (**SEC-5 ✓**) | нет (легитимно: локальный релей) | строки протокола (`net/smtp`) | админу/держателю токена: `502 smtp_failed` + `detail` = текст ошибки, включая `host:port` и первую строку ответа сервера (см. Н-2) |
| Push: `push.go:243-267 deliver` через `endpoint.go:26-33` | **участник** (`POST /push/subscribe`) | 10 с (`http.Client.Timeout`, покрывает тело) | запрещены (`ErrUseLastResponse`) (**SEC-4 ✓**) | стандартная | при **подписке**: только `https`, IP и результат DNS вне private/loopback/link-local/CGNAT (**SEC-4 ✓**); при **доставке** — нет (см. Н-1) | тело читается целиком в `io.Discard`, без лимита (см. Н-4) | участнику — ничего; в лог — статус и ошибка транспорта (с URL endpoint) |
| `check/redirect.go:11-38 ProbeHTTPRedirect` (`POST /admin/check`) | админ (`public_url`) | 10 с | запрещены | plain `http://` | нет | тело не читается | админу: код 301/308 или «нет» |
| `check/cert.go:23-64 ProbeTLS` (`POST /admin/check`) | админ (`public_url`) | 10 с dial | н/п | `tls.Dial` с проверкой; ошибка → `ProbeError` (админу — обобщённо) | нет | сертификаты | админу: `NotAfter`, Issuer CN/O, полнота цепочки |
| `check/domain.go:66 LookupIPAddr`, `:25-37` UDP-dial `8.8.8.8:80` | админ (`public_url`) / константа | ctx запроса / 3 с | н/п | н/п | нет | DNS-ответ | админу: IP-адреса |
| `check/ntp.go:12-39` UDP `pool.ntp.org:123` | константа | 2 с | н/п | нет (NTP без аутентификации; ответ не сверяется с запросом) | н/п | 48 байт | админу: дрейф часов |
| `push/endpoint.go:61-66 LookupIPAddr` (при подписке) | участник | 2 с | н/п | н/п | результат фильтруется | DNS | ничего (при отказе — `invalid`) |

`admin/proxy/{kind}` — исходящих запросов **нет**: `kind` ∈ {nginx, caddy, traefik}, шаблоны — константы в коде (`admin_check.go:62-161`), путей/файлов не читает.

## 2. Находки

### Н-1. DNS-rebinding / нерезолвящееся имя обходят `validateEndpoint` — доставка пуша во внутреннюю сеть
- Серьёзность: низкая
- Уверенность: подтверждено кодом (проверка только в `Subscribe`, в `deliver` клиент без своего `DialContext`)
- Где: `internal/push/endpoint.go:43-73`, `internal/push/push.go:107,243-267`, `internal/push/endpoint.go:26-33`
- Атакующий: участник (оплаченный)
- Что: фильтр приватных адресов выполняется один раз при подписке по результату DNS; имя, которое в момент подписки не резолвится (`:64-65` — принимается) или резолвится в публичный адрес, при доставке может указывать на `127.0.0.1`/`10.x`. Сервер отправит POST с зашифрованным (aes128gcm) телом и заголовком `Authorization: vapid …` на внутренний адрес. Тело атакующим не управляется, редиректы запрещены; остаётся 1-битный оракул (404/410 удаляет подписку — участник увидит по повторному `subscribe`/по отсутствию пушей) и «постучать» во внутренний HTTPS-сервис. Раздел B закрывает только список разрешённых сервисов, не проверку при доставке.
- Как воспроизвести: домен с TTL 0, первый ответ A=1.2.3.4, следующий A=127.0.0.1; `POST /api/v1/push/subscribe {endpoint:"https://d.example/x", ...}` → создать запись в круге → в логе `notify*: push …: Post "https://d.example/x": … connection refused` (соединение к loopback состоялось).
- Исправление: у `newDeliveryClient` задать `Transport.DialContext`, который резолвит хост сам, отвергает непубличные адреса и подключается к проверенному IP (передавая `ServerName` для TLS); `validateEndpoint` при подписке оставить как раннюю проверку.

### Н-2. Баннер-скан внутренней сети через SMTP-пробу (админ / держатель bootstrap-токена)
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/api/domain_errors.go:32-33`, `internal/mail/mail.go:289-291,304-308`, `internal/api/admin.go:87-90,149-158`, `internal/api/admin_smtp.go:38-63,65-76`
- Атакующий: админ; до bootstrap — аноним, знающий bootstrap-токен (лимитер по IP есть)
- Что: `mail.Probe`/`SendTest` соединяются с любым `host:port`; `smtp.NewClient` читает приветствие, и для не-SMTP сервиса ошибка `textproto.Error` содержит первую строку баннера (например `SSH-2.0-OpenSSH_9.6`), а сетевые ошибки различают `connection refused` / `i/o timeout` / `no route`. Всё это возвращается вызывающему в поле `detail` (502). Это чтение первой строки любого TCP-сервиса внутренней сети сервера с правами админа панели.
- Как воспроизвести: `PUT /admin/smtp {host:"10.0.0.5", port:22, from:"a@b"}` → `POST /admin/smtp/test {to:"a@b"}` → `detail: "mail: send failed: mail: smtp client: SSH-2.0-…"`; либо `POST /admin/bootstrap/smtp-test` с токеном.
- Исправление: в `writeDomainError` для `ErrSend` отдавать классифицированную причину (`dial`, `tls`, `starttls_required`, `auth`, `protocol`) без сырого текста сервера; сырой текст — только в лог. Полное сокрытие адреса не требуется — его ввёл сам админ.

### Н-3. Медленный push-endpoint участника глушит уведомления остальным участникам круга
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/api/notify.go:140,173,201,239` (общий `WithTimeout(10s)`), `:155-161,221-227,264-270` (последовательная доставка), `internal/push/push.go:213-226`, `internal/push/endpoint.go:17`
- Атакующий: участник круга
- Что: каждая `notify*`-горутина даёт **один** 10-секундный контекст на цикл по всем участникам и всем их подпискам; доставка последовательная и наследует ctx. Endpoint участника, который принимает TCP и молчит, съедает весь бюджет; всем, кто идёт после него в `CircleMemberAccountIDs`, `SendSignal` возвращает `context deadline exceeded` — они не получают пушей ни о записях, ни о комментариях, ни об упоминаниях, пока подписка существует. Пуш — только сигнал (клиент дотягивает данные по SSE/опросом), поэтому потеря ограничена уведомлениями.
- Как воспроизвести: участник A подписывается на `https://<свой хост>/sink`, где сервер держит соединение открытым 30 с; участник B создаёт запись; участник C (после A в списке) не получает пуш; в логе `notifyCircle: push C/…: context deadline exceeded`.
- Исправление: таймаут на **одну** доставку (свой `context.WithTimeout(ctx, 10s)` внутри `deliver`), а внешний бюджет — на весь цикл пропорционально числу адресатов или без него; при желании — доставка подписок параллельно с ограничением.

### Н-4. Тело ответа push-сервиса читается без лимита
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/push/push.go:262`
- Атакующий: участник (контролирует endpoint)
- Что: `io.Copy(io.Discard, resp.Body)` — ограничено лишь 10-секундным таймаутом клиента; за 10 с злонамеренный endpoint может заставить сервер скачать сотни мегабайт (память не растёт — `Discard`, но канал занят).
- Исправление: `io.CopyN(io.Discard, resp.Body, 64<<10)` (дренаж нужен только для переиспользования соединения).

### Н-5. Копия `wynd.db` в бэкапе создаётся с правами SQLite по умолчанию (0644 & ~umask), а `keys/bootstrap` — с 0640 вместо 0600
- Серьёзность: средняя
- Уверенность: подтверждено кодом в части отсутствия `Chmod`; фактический режим (ожидаемо 0644 при umask 022) — предположение, нужен пробный тест на Linux
- Где: `internal/backup/backup.go:90-111` (`VACUUM INTO` → `Rename`, без `os.Chmod`), `:197-215 copyFile` (`0o640` для всех, включая `keys/bootstrap`), `:206`
- Атакующий: локальный пользователь на хосте, читающий каталог бэкапа (или получатель бэкапа при переносе)
- Что: `wynd.db` содержит SMTP-пароль, приватный ключ VAPID, хэши сессий, e-mail участников, `reset_token` админа — раздел B принял хранение открытым текстом **при условии** прав `0600` на файл и каталог бэкапа. Сам код бэкапа это условие нарушает: файл, созданный `VACUUM INTO`, получает права SQLite по умолчанию (0644 до umask); каталог назначения создаётся 0750 только если его не было (`:45`), в существующий каталог (`/var/backups`, точка монтирования, 0755) файл ляжет читаемым для всех. Bootstrap-токен копируется из 0600 в 0640.
- Как воспроизвести: `umask 022; wynd backup /tmp/wb; stat -c %a /tmp/wb/wynd.db /tmp/wb/keys/bootstrap` → ожидается `644` и `640`.
- Исправление: после `Rename` — `os.Chmod(destPath, 0o600)`; в `copyFile` — наследовать режим источника (`info.Mode().Perm()`) либо 0600 для `keys/*` и `wynd.db`; каталог назначения — `os.Chmod(destDir, 0o700)` при создании. Записать в `server-reference.md` требование к правам каталога бэкапа.

### Н-6. `wynd.db` создаётся с правами 0640, а не 0600
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/config/config.go:126` (`os.OpenFile(dbPath, O_CREATE|O_WRONLY, 0o640)`), `internal/config/config.go:120` (каталоги 0750)
- Атакующий: локальный пользователь в группе `wynd` (или в группе процесса при запуске без выделенного пользователя — `dev/data` в рабочем каталоге разработчика)
- Что: раздел B (DEC-1) требует `0600` на `wynd.db` как условие принятого риска «секреты открытым текстом». Код задаёт 0640 — группа читает БД со SMTP-паролем и ключом VAPID. `-wal`/`-shm` наследуют 0640. В systemd-юните `UMask` не задан (по умолчанию 0022 — не сужает 0640).
- Как воспроизвести: свежий старт → `stat -c %a $WYND_DATA_DIR/wynd.db` → `640`.
- Исправление: `0o600` в `ensureDataLayout` + `UMask=0077` в `wynd.service`; при старте — `os.Chmod(dbPath, 0o600)` для уже существующих файлов (одноразовое выравнивание).

### Н-7. Валидация `public_url` в bootstrap выполняется после коммита установки
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/api/admin.go:63-66` (`NormalizePublicURL` без `ValidatePublicURL`), `:94-110` (коммит `Auth.Bootstrap` + SMTP), `:112-118` (`WritePublicURL` валидирует и падает)
- Атакующий: нет (ошибка порядка операций, влияет на оператора)
- Что: `public_url` вида `https://x/path` или `https://user@x` проходит до `WritePublicURL`, где отвергается уже после того, как админ и SMTP записаны; клиент получает ошибку, повтор — «уже установлено»; `public_url` остаётся прежним (loopback по умолчанию), и инстанс продолжает работать с `loopback=true` (ссылки в письмах на 127.0.0.1). Также `loopback` (`:67`) вычисляется от невалидированного URL.
- Исправление: вызывать `config.ValidatePublicURL(body.PublicURL)` до `ConfirmBootstrapToken`/`Probe`, как остальные предусловия.

### Н-8. URL push-endpoint попадает в лог при ошибке транспорта
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/api/notify.go:160,189,226,269` (`%v` ошибки `http.Client`), `internal/push/push.go:258-259`
- Атакующий: читатель логов (оператор/адм. журнала)
- Что: ошибка `Post "https://fcm.googleapis.com/fcm/send/<token>": …` содержит полный endpoint — capability-URL браузера участника (с ключами `p256dh/auth` его нет, зато есть VAPID-ключ инстанса, так что оператор мог бы слать участнику пуши и после отписки). Ценность низкая, но это ПД-подобный идентификатор в логе.
- Исправление: в `deliver` оборачивать ошибку транспорта в собственную с хостом endpoint без пути (`url.Parse(endpoint).Host`).

### Н-9. `/ready` отдаёт текст ошибки БД анониму
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `cmd/wynd/main.go:249-264`
- Атакующий: аноним
- Что: при сбое `Ping` в ответ уходит `err.Error()` — сообщение драйвера SQLite (может включать путь к файлу БД). Маршрут вне `/api/`, без лимитера; сама проверка дёшева (2 с таймаут, пул соединений).
- Исправление: отдавать только `{"status":"unavailable"}`, текст — в лог.

## Просмотренные файлы (журнал)

### internal/check/*.go, internal/api/admin_check.go, internal/proxy/timeout.go

- `check.go`: `RunChecks` — чистые функции над `Input`; сетевые части: `checkDomain` (DNS + UDP-dial к `8.8.8.8:80` для определения egress IP, без отправки данных), `checkClocks` → `ntp.go`.
- `domain.go:25-37`: `serverEgressIP` — `net.Dialer{Timeout:3s}` UDP `8.8.8.8:80`; UDP-dial не отправляет пакеты, только выбирает маршрут. `checkDomain:66` — `LookupIPAddr(host)` где host из `public_url` (задаёт админ/оператор). В ответ показываются только IP-адреса.
- `cert.go:23-64`: `ProbeTLS` — `tls.DialWithDialer` 10 с на `host:443`, `ServerName: host`, **без** `InsecureSkipVerify` (валидация системными корнями; при ошибке валидации — `ProbeError` = текст ошибки Go, который админу не показывается, `cert.go:76` даёт «не удалось прочитать сертификат»). Из ответа показываются: `NotAfter`, `Issuer.CommonName`, `Issuer.Organization`, полнота цепочки. Тела нет.
- `redirect.go:11-38`: `ProbeHTTPRedirect` — `http.Client{Timeout:10s, CheckRedirect: ErrUseLastResponse}` (редиректы не следуются), `GET http://<host>/api/v1/instance`. Тело **не читается** (только `resp.StatusCode`), закрывается. Показывается только код 301/308 или 0. Хост — из `public_url`.
- `ntp.go:12-39`: UDP к `pool.ntp.org:123`, 2 с, 48 байт запрос/ответ. Ответ не сверяется с запросом (Originate Timestamp) — подмена UDP-ответа даст ложный дрейф часов в админской проверке; влияние: только строка в отчёте. Заметка.
- `admin_check.go:18-35`: `handleAdminCheck` под `requireAdmin` (см. server.go). `readJSON` для `External` — админский ввод; поля отчёта возвращаются админу переработанными (`ClientIP` парсится `net.ParseIP`, сырые строки XFF/XRI в `Detail` не попадают).
- `admin_check.go:62-83`: `handleAdminProxySnippet` — `kind` ∈ {nginx, caddy, traefik}, иначе 404; шаблоны — строковые константы в коде, не файлы; путей из `kind` нет. В сниппет подставляются `ListenAddr`, `public_url` host, `AttachmentMaxBytes`.
- Сниппеты: nginx — `$proxy_add_x_forwarded_for` (добавляет к клиентскому XFF, не перезаписывает); Caddy — `header_up X-Forwarded-For {remote_host}` (перезапись); Traefik — в комментарии предлагается `forwardedHeaders.insecure: true`. Все три безопасны **только потому**, что `respond.go:151-174 clientIP` берёт самый правый адрес, не входящий в `TrustedProxies`, и вовсе игнорирует XFF, если прямой пир не доверенный. Проверено — корректно. HSTS есть во всех трёх; таймаут 300 с; `proxy_buffering off`/`flush_interval -1`/`flushInterval 100ms` для SSE.

### internal/mail/*.go

- `mail.go:282-331 smtpClient`: адрес `host:port` из настроек (админ). Таймаут: `context.WithTimeout(sendTimeout=15s)` + `conn.SetDeadline` (`:293`). TLS: порт 465 — implicit `tls.Client(..., &tls.Config{ServerName: host})`; иначе STARTTLS обязателен, кроме `isLoopbackHost` (`:317`). `InsecureSkipVerify` **нет** — проверка сертификата системными корнями. **SEC-5 подтверждено** (`:319-323`). Фильтра приватных адресов нет — и не нужен (локальный релей легитимен), редиректов у SMTP нет.
- `authenticate:333-355`: `smtp.PlainAuth` (stdlib сама отказывается слать без TLS вне localhost), `loginAuth` (`login.go:13-18`) — тот же отказ на `!server.TLS && !localhost`. Механизм выбирается по рекламе сервера; по умолчанию PLAIN.
- Ошибки: `%w: mail: dial %s` включает `host:port`; `mail: auth: %w` — текст ответа сервера (535 …), пароль/AUTH-строка в него не попадают (stdlib не включает отправленные данные). Текст ошибки уходит в `smtp_last_error` (`recordSMTPTest:193`) → админу в `/admin/check` («не ушло: …»). Приемлемо.
- `buildMessage:424-447`: `headerSafe` (CR/LF → пробел) на From/To/Subject — **SEC-5 подтверждено**; From через `net/mail.Address.String()` (корректное квотирование), Subject — Q-encoding при не-ASCII. `To` пишется как есть после `headerSafe` (не через `netmail.Address`) — при адресе с не-ASCII символами заголовок будет сырым UTF-8; функционально, не безопасность. `Content-Type: text/plain; charset=UTF-8` — **HTML нет**, экранирование имён не требуется. Тело — через `smtp.Client.Data()` (DotWriter: точки экранируются, LF→CRLF).
- Что уходит на почтовый релей: код входа (`SendCode:167`), проверочный текст, напоминание об оплате с `instanceName` (`pay.go:17-23`), архив: `cutoffDate`, deadline, `downloadURL` (`archive.go:18-23, 34-39`); уведомления — см. `internal/api/notify.go` ниже. Содержимого записей/имён участников в почте из этого пакета нет.
- Loopback-фолбэк: `SendCode` при `!configured && loopback` → `fallback.SendCode` (`auth.LogCodes`, лог); `archive.go:50` печатает в лог `to=<email> subject=… <body>` — **только** при `loopback && !configured`. Loopback = `config.IsLoopback(public_url)` (`loopback.go:10-21`): хост `localhost` или loopback-IP. На публичном инстансе `public_url` — публичный хост → фолбэк не включится; чтобы включить, нужно переписать `public_url` (админ/оператор). ОК.
- `messageID:411` — `UnixNano` + 8 случайных байт; ничего не раскрывает.
- `SaveConfig:119-135`: пароль сохраняется как есть (открытый текст в БД — закрытая развилка, DEC-1).

### internal/config/*.go

- `config.go:37-101 Load`: читает `WYND_DATA_DIR` (пусто → `dev/data` относительно cwd — `filepath.Abs`), `WYND_LISTEN`, `WYND_PUBLIC_URL` (перекрывает `config.json`; логируется факт перекрытия с обоими URL — не секрет), `WYND_TRUSTED_PROXIES` (`LookupEnv` — пустая строка явно обнуляет список). `ValidatePublicURL` (`public_url.go:29-51`): только http/https, есть хост, без пути/запроса/якоря/userinfo. Хорошо.
- `ensureDataLayout:113-131`: каталоги `0o750`, `wynd.db` создаётся заранее пустым файлом `0o640` — SQLite при последующем открытии сохранит режим; `-wal`/`-shm` SQLite создаёт с режимом основного файла. `umask` процесса дополнительно ограничивает. См. находку «`wynd.db` 0640» ниже.
- `writeFileConfig:133-148` — **мёртвый код** (нет вызовов), пишет `fileConfig` без `TrustedProxies`. Живой путь — `WritePublicURL` (`public_url.go:54-82`) — читает существующий файл целиком и сохраняет `trusted_proxies`. ОК.
- `atomic.go:12-42 writeFileAtomic`: `os.CreateTemp` (0600) → write → fsync → `Chmod(perm=0640)` → `Rename`. Временный файл удаляется в `defer`. `config.json` секретов не содержит (listen, public_url, trusted_proxies).
- `bootstrap.go:12-40`: токен — 32 байта `crypto/rand` hex, файл `keys/bootstrap` `0o600`; при существующем непустом файле переиспользуется. Где печатается — см. `cmd/wynd/main.go`.

### internal/api/admin.go (bootstrap), admin_smtp.go, domain_errors.go, auth/admin.go

- `admin.go:131-160 handleBootstrapSMTPTest`: аноним с bootstrap-токеном; `ConfirmBootstrapToken` (`auth/admin.go:26-57`) — лимитер по IP (`loginLimiter`), `ConstantTimeCompare`, отказ при `bootstrapped=1`. Затем `mail.Probe` на `host:port` из тела запроса. Тот же путь — `handleBootstrap:87`.
- `admin.go:63-66`: `publicURL = config.NormalizePublicURL(body.PublicURL)` **без** `ValidatePublicURL`; проверка происходит только в `WritePublicURL:113` — уже после коммита `Auth.Bootstrap`. При невалидном `public_url` (например `https://x/path`) админ и SMTP сохранены, ответ — ошибка, повтор — «уже установлено». Не безопасность, а порядок операций; заметка.
- `domain_errors.go:32-33`: `mail.ErrSend` → 502 с `detail: err.Error()`. Текст включает `host:port` и первую строку ответа сервера (для не-SMTP сервиса `smtp.NewClient` вернёт `textproto.Error` с баннером — например строкой SSH). См. находку «баннер-скан через SMTP-пробу».
- `admin_smtp.go:22-36 GET /admin/smtp`: возвращает host/port/username/from/configured/test_sent_at — **пароль не эхо-ится**. ОК.
- `admin_smtp.go:38-63 handleAdminSetSMTP`: пустой пароль → оставляет текущий. Побочный эффект: смена `host` без повторного ввода пароля отправит **старый** пароль на **новый** хост при следующей отправке/пробе. Решение админа, но неочевидное; заметка. Способа очистить пароль нет.
- `admin_smtp.go:65-76 handleAdminSMTPTest`: адрес получателя произвольный (админ). `handleAdminVAPID:78-89` — только `public_key`. `handleAdminPushTest:91-103` — пуш только на подписки самого админа.
- `admin.go:265-278 requireAdmin`: `limitBody` + `IsAdminSession`. Все `admin/*` из области — под ним (`server.go`, проверено grep-ом ниже).

### internal/push/*.go, internal/api/notify.go, internal/auth/notify.go

- `endpoint.go:26-33 newDeliveryClient`: `http.Client{Timeout: 10s, CheckRedirect: ErrUseLastResponse}` — **SEC-4 подтверждено**. TLS — стандартная проверка (`InsecureSkipVerify` нет). `SubscriberMailto = "mailto:admin@wynd.local"` константа (`:14`).
- `endpoint.go:43-73 validateEndpoint`: только `https`, IP-литерал → `isPublicIP`; имя → `LookupIPAddr` (2 с), любой приватный адрес в ответе → отказ; **нерезолвящееся имя принимается** (комментарий это признаёт). `isPublicIP:75-91` покрывает loopback/private/unspecified/link-local/multicast/CGNAT/0.0.0.0/8; не покрыты 192.0.0.0/24, 198.18.0.0/15, 240.0.0.0/4 (экзотика). Проверка выполняется **только при подписке**, а не при доставке — см. находку «DNS-rebinding обходит validateEndpoint».
- `push.go:243-267 deliver`: `webpush.SendNotificationWithContext` (aes128gcm по RFC 8291, VAPID по RFC 8292 — стандарт, библиотека `SherClockHolmes/webpush-go`). Ответ: `io.Copy(io.Discard, resp.Body)` **без лимита размера** (`:262`) — ограничено только 10-секундным таймаутом клиента, которым покрывается и чтение тела. Из ответа наружу — только `StatusCode` (в `deliveryError`, идёт в лог; 404/410 → удаление подписки).
- Payload (`Signal`, `push.go:23-29`): `circle_id`, `type`, `count`; `title`/`body` заполняются только в `SendTest`. Текст записей, имена, e-mail, ссылки — **не уходят**. Хорошо.
- VAPID: `EnsureKeys:66-85` — `webpush.GenerateVAPIDKeys()` (P-256), хранение в `instance_settings.vapid_private_key` открытым текстом (закрытая развилка DEC-1). `PublicKey:88-97` — только публичный; приватный ключ не логируется и не отдаётся ни одним обработчиком (grep `vapid_private_key` — только `push.go`, миграции).
- `push.go:100-162 Subscribe`: чужой endpoint с другими ключами → `ErrForbidden` (защита от угона подписки). `Unsubscribe` — только свои (`account_id = ?`).
- `notify.go:133-273`: все `notify*` — фоновые горутины с **общим** `context.WithTimeout(10s)` на цикл по всем участникам круга; доставка последовательная, `SendSignal` → `deliver` наследует ctx. Один медленный endpoint (участник контролирует адрес) съедает бюджет остальных — см. находку «Медленный push-endpoint глушит уведомления круга».
- Логи `notify.go:149,160,178,189,206,215,226,244,249,258,269`: `accountID/circleID` (не ПД) и `%v` ошибки. Ошибка транспорта от `http.Client` включает URL endpoint (`Post "https://fcm…/<token>": …`) — endpoint-URL является capability-ссылкой браузера (кто знает — может слать пуши при наличии VAPID-ключа). Утечка в лог низкой ценности; заметка.
- `auth/notify.go`: чистая логика предпочтений; `MuteUntil` при сохранении не валидируется (любая строка ≤ лимита тела), при чтении непарсимое = не заглушено. Не безопасность.

### cmd/wynd/main.go, cmd/wynd/backup.go

- `main.go:46-49, 160-162`: bootstrap-токен печатается в лог **при каждом старте**, пока `bootstrapped=0`: `bootstrap URL: <public_url>/admin/bootstrap?token=…` (stdout → journald/`docker logs`). Файл `keys/bootstrap` 0600. Приемлемая практика для первичной установки; токен после bootstrap бесполезен (`ConfirmBootstrapToken` отказывает при `bootstrapped=1`). Заметка: файл `keys/bootstrap` **никогда не удаляется** и попадает в бэкап.
- `main.go:78-83`: `codeLog.File = dev-auth-codes.log` только при `loopback`; `auth.LogCodes.SendCode` (`auth/auth.go:38-60`) пишет `auth code for <email>: <code>` в стандартный лог и файл 0600 (каталог 0700). Включается только `loopback && !configured(SMTP)` (`mail.go:160-165`). `capture_codes` как таковой в проде нет — `CaptureCodes` только в тестах. На публичном инстансе включить можно лишь сменой `public_url` на loopback админом/оператором (`applyPublicURL` из bootstrap или `admin_access.go:54`) — проверю в разделе «Проверено».
- `main.go:107-108`: `/health` и `/ready` — без аутентификации и лимитера. `/ready` (`:249-264`) при отказе `Ping` отдаёт `err.Error()` анониму — текст ошибки SQLite (может содержать путь к файлу БД). Низкая ценность; заметка.
- `main.go:148-156`: `ReadHeaderTimeout 10s`, `IdleTimeout 2m`, без `ReadTimeout`/`WriteTimeout` (осознанно, SSE/загрузки); `MaxHeaderBytes` по умолчанию 1 МБ.
- Логи `main.go`: schema version, data dir, listen, счётчики рутин — секретов и ПД нет. `log.Printf("pay jobs: %+v", counts)` — только числа.
- `backup.go:19,35-37`: `config.Load()` по-прежнему зовёт `ensureDataLayout` (создаёт каталоги и пустой `wynd.db`), но проверка `Size()==0` (`:35`) прерывает бэкап пустоты (STB-4 закрыт по результату, хотя комментарий `config.go:59` про «ничего не пишет» неточен).

### internal/backup/backup.go

- Состав бэкапа (`:30-68`): `config.json`, `wynd.db` (через `VACUUM INTO`), `keys/` (включая `keys/bootstrap`), `blobs/` (кроме `.uploads/` — `.part`-файлы не копируются, `:132-134`), `manifest.json` с SHA-256.
- Права: каталоги `0o750`; `copyFile:206` — `0o640` для всех файлов, в т.ч. **`keys/bootstrap` (исходно 0600 → в копии 0640)**; `writeManifest:194` — 0640. `backupDatabase:90-111` — `VACUUM INTO '<dest>.vacuum'` → `Rename`: файл создаёт SQLite с режимом по умолчанию (`SQLITE_DEFAULT_FILE_PERMISSIONS` = 0644 & ~umask), **`os.Chmod` нет** — см. находку «Копия wynd.db в бэкапе создаётся с правами SQLite по умолчанию».
- `VACUUM INTO` — путь экранируется (`'` → `''`, `:100`), путь задаёт оператор CLI. Инъекции нет.
- `touchLastBackupAt:254-274` пишет в живую БД `last_backup_at` — открытие без `journal_mode` (режим WAL хранится в заголовке файла, ок).
- Инкрементальный режим не удаляет из назначения то, что удалено из источника (в очереди волны 7). Удалённые участниками фото остаются в бэкапе — вопрос удержания данных, не дефект кода; заметка для документации.

### internal/store/*.go

- `sqlite.go:45-54`: DSN — `_pragma=busy_timeout(5000)`, `foreign_keys(1)`, `journal_mode(WAL)`, `_txlock=immediate`; `checkPragmas:72-100` **проверяет** фактические значения после открытия (fail-closed). Хорошо.
- `store.Open` не меняет права файла: `wynd.db` заранее создаётся `ensureDataLayout` (0640). Побочные `-wal`/`-shm` SQLite создаёт с режимом основного файла.
- `migrate.go:15-16`: миграции — `//go:embed migrations/*.sql`, с диска пользователя не читаются. `stripSQLComments` — **уже удалён** (в файле отсутствует). MIG-1 (пропущенная версия → отказ, `:65-69`) — на месте. Каждая миграция — в транзакции (`applyMigration:147-169`). Неизвестная версия выше максимума → отказ (`:49-51`).

### internal/jobs/routine.go

- Исходящих соединений нет. Логи — только счётчики. `storage_path` из БД соединяется с `blobsDir` без нормализации (`:138`) — эксплуатируемо только при компрометации БД; заметка. `cleanEmptyAccounts:212` переписывает e-mail на `deleted+<id>@wynd.local` — ПД стирается. ОК.

### deploy/**, .dockerignore, .gitignore, scripts/*

- `deploy/docker/Dockerfile`: три стадии; финальный образ `alpine:3.21` + `ca-certificates` + `tzdata`, пользователь `wynd` (не root, `USER wynd` `:27`), только бинарник. Секретов в образ не копируется (`COPY . .` — лишь в build-стадии, контекст фильтруется `.dockerignore`: `dev/`, `data/`, `.git`). **Нет `HEALTHCHECK`**, нет `-trimpath` (DEP-1 в очереди). `WYND_LISTEN=:7676` — слушает все интерфейсы контейнера (нужно для прокси в другом контейнере).
- `.dockerignore` исключает `web/src/` — при этом Dockerfile `:7-8` копирует `web/` и делает `npm run build`, т.е. сборка образа из чистого контекста **невозможна** (известно, DEP-1). Не безопасность.
- `deploy/docker/compose.yaml`: у `wynd` порт наружу **не публикуется** (только `caddy` 80/443) — хорошо. `WYND_TRUSTED_PROXIES: 10.0.0.0/8,172.16.0.0/12,192.168.0.0/16` — все RFC1918; безопасно ровно пока порт `wynd` не опубликован (комментарий `:10-11` это фиксирует). Заметка для документации: при добавлении `ports:` к `wynd` любой клиент из LAN сможет подделать XFF.
- `deploy/caddy/Caddyfile`, `deploy/proxy/Caddyfile`: HSTS, `header_up X-Forwarded-For {remote_host}` (перезапись — клиентский XFF не проходит), `flush_interval -1` (SSE), `read_timeout 300s`, `request_body max_size 100MB`. `X-Forwarded-Proto` Caddy выставляет сам. ОК.
- `deploy/proxy/nginx.conf`: `$proxy_add_x_forwarded_for` — дописывает `$remote_addr` к клиентскому XFF; безопасно благодаря выбору самого правого недоверенного адреса в `clientIP`. HSTS `always`, `proxy_buffering off`, 300 с, 100m. ОК.
- `deploy/proxy/traefik.yaml`: в комментарии `:7-11` предлагается `forwardedHeaders.insecure: true` — Traefik тогда **сохраняет** клиентский XFF и дописывает свой адрес; при `clientIP` «правый недоверенный» — безопасно; но совет `insecure: true` избыточен (Traefik и без него ставит XFF) и вводит в заблуждение. Заметка. Также в Traefik-сниппете HSTS через `stsSeconds` — ОК.
- `deploy/systemd/wynd.service`: `User/Group=wynd`, `NoNewPrivileges`, `PrivateTmp`, `ProtectSystem=strict`, `ProtectHome`, `ReadWritePaths=/var/lib/wynd`. **Нет**: `UMask=0077`, `ProtectKernelTunables/Modules/Logs`, `ProtectControlGroups`, `ProtectClock`, `ProtectHostname`, `ProtectProc=invisible`, `PrivateDevices`, `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX`, `RestrictNamespaces`, `RestrictRealtime`, `RestrictSUIDSGID`, `LockPersonality`, `MemoryDenyWriteExecute`, `CapabilityBoundingSet=`, `SystemCallFilter=@system-service`. Зафиксировано как текущее состояние для DEP-3.
- `deploy/install.sh`: `set -euo pipefail`, требует root; **нет `curl | sh` и загрузок** — бинарник берётся из `--bin`, `--from-source` или рядом со скриптом; Caddy — через `apt-get` (подписанные репозитории). Контрольных сумм нет (нечего проверять — источник локальный). Каталоги `0750` под `wynd`, `config.json` `0640`. Перезаписывает `/etc/caddy/Caddyfile` целиком (`:109`) — снесёт чужие сайты (DEP-2). `$PUBLIC_URL` и `$host` подставляются в JSON/`sed` без экранирования — вход оператора. Bootstrap-URL с токеном печатается в терминал (`:254`).
- `.gitignore`: `/dev/`, `/data/`, `/data-*/`; живые SMTP-учётки для тестов ищутся в `dev/credentials/` (`smtp_fixtures_test.go:34-65`) — под игнором. В `internal/mail/testdata/` — только `*.example.*`. ОК.
- `scripts/*.bat`, `binary-stale.ps1`, `extract-ui-assets.py`: локальные dev-обёртки (сборка, запуск, упаковка QA-архива), без загрузок и секретов.

### Зависимости

`go.mod` (прямые): `github.com/SherClockHolmes/webpush-go v1.4.0` (сеть+крипто: HTTP к push-сервисам, ECDH/HKDF/AES-GCM, VAPID JWT — через косвенную `github.com/golang-jwt/jwt/v5 v5.2.2`), `github.com/google/uuid v1.6.0`, `golang.org/x/crypto v0.56.0` (крипто: bcrypt для пароля админа), `modernc.org/sqlite v1.36.1` (БД, CGO-free). Косвенные: `dustin/go-humanize`, `mattn/go-isatty`, `ncruces/go-strftime`, `remyoudompheng/bigfft`, `golang.org/x/exp`, `golang.org/x/sys v0.47.0`, `modernc.org/{libc,mathutil,memory}`. Сеть в stdlib: `net/http`, `net/smtp`, `crypto/tls`.

`web/package.json` (runtime `dependencies`): `bits-ui ^1.3.19`, `exifr ^7.1.3` (парсер EXIF — обработка недоверенных файлов на клиенте), `idb ^8.0.2`, `leaflet ^1.9.4` (сеть: плитки OSM), `leaflet.markercluster ^1.5.3`, `mediabunny ^1.58.1` (демуксер/кодеки медиа — недоверенный ввод на клиенте), `qrcode ^1.5.4`. `devDependencies`: `@sveltejs/kit ^2.16.0`, `svelte ^5`, `vite ^6`, `vitest ^3.2.4`, `@vite-pwa/sveltekit ^1.1.0`, `typescript ^5`, `eslint ^9.18`, `prettier ^3.4.2`, `jsdom ^30`, `fake-indexeddb ^6`, `openapi-typescript ^7.8` (к удалению, CPX-1), типы `@types/*`. Криптографических зависимостей на клиенте нет (Web Push — через браузерный `PushManager`).


## 3. Ответы на вопросы задания (сводно)

**2. Почта.** `buildMessage` (`mail.go:424-447`): CR/LF заменены пробелом во From/To/Subject (SEC-5 ✓); From — через `net/mail.Address`; тело — `text/plain; charset=UTF-8`, HTML нет, экранирование имён не требуется. From/To — из настроек и адресата. Ссылки в письмах строятся от `public_url` (архив: `downloadURL` формируется в `internal/jobs/archive.go` от `publicURL`, который `main.go:131` берёт свежим через `apiSrv.PublicURL()`); `public_url` меняют только админ панели (`admin_access.go:49-59`, `WritePublicURL` с валидацией) и оператор (`WYND_PUBLIC_URL`/`config.json`) — участник не может. В письмах уходит: код входа, название инстанса, дата отсечки/срока архива, ссылка на архив, тексты уведомлений об оплате — **содержимого записей и имён участников в почте нет**. Пароль SMTP: `GET /admin/smtp` не отдаёт (`admin_smtp.go:28-35`); в ошибках SMTP пароль/AUTH-строка отсутствуют (stdlib не включает отправленное); `mail.Probe` — dial → (STARTTLS) → AUTH → QUIT без письма и без записи в БД (`mail.go:226-241`).

**3. Push.** Payload — `{circle_id, type, count}` (+ `title/body` только в тесте админа); шифрование — RFC 8291 через `webpush-go`; VAPID-ключи в `instance_settings` открытым текстом (DEC-1, принято), не логируются, приватный не отдаётся ни одним обработчиком (grep `vapid_private_key`: только `push.go` и миграция). `GET /admin/push/vapid` — только `public_key`. `POST /admin/push/test` — на подписки самого админа.

**4. Секреты в логах.** Полный grep `log.Print*/Fatal*/Fprint(os.Stderr` по `internal/`, `cmd/` — в разделе журнала. Токены сессий — не печатаются; коды входа — только `auth.LogCodes` при `loopback && !SMTP` (stdout + `dev-auth-codes.log` 0600); `CaptureCodes` — только в тестах. На публичном инстансе loopback-режим включается лишь сменой `public_url` на localhost администратором, и при настроенном SMTP коды всё равно идут почтой (`mail.go:160-165`), а очистить SMTP-хост нельзя (`saveConfig:120`). Bootstrap-токен — в stdout при каждом старте до bootstrap (`main.go:161`) и в `keys/bootstrap` 0600. Пароль SMTP, ключ VAPID — не печатаются. E-mail участников — в логе только `archive.go:50` (loopback-фолбэк). Тела запросов, XFF — не логируются. `config.go:82` печатает оба URL (не секрет).

**5. Файлы на диске.** `config.json` — 0640, только `listen/public_url/trusted_proxies`, атомарная запись (temp 0600 → chmod → rename). `wynd.db` — 0640 (Н-6), создаётся `ensureDataLayout` до `store.Open`. `blobs/`, `keys/` — 0750. `.part` — `blobs/.uploads/` (в бэкап не попадают). Бэкап — Н-5. `WYND_*` читает только `config.Load` (`config.go:38-88`); пустой `WYND_DATA_DIR` → `dev/data` от cwd.

**6. Деплой.** См. журнал: Dockerfile — non-root, без HEALTHCHECK; compose — порт wynd не наружу, RFC1918 в trusted; systemd — базовый hardening без UMask/SystemCallFilter и пр. (DEP-3); install.sh — без загрузок по http, без `curl|sh`; прокси-сниппеты — HSTS, 300 с, 100 МБ, буферизация выключена; клиентский XFF: Caddy перезаписывает, nginx дописывает, Traefik по умолчанию перезаписывает — всё безопасно из-за `clientIP` «правый недоверенный».

**7. Store.** `busy_timeout(5000)`, `foreign_keys(1)`, `journal_mode(WAL)`, `_txlock=immediate` в DSN и **проверяются** после открытия (`checkPragmas`). Миграции — из `embed.FS`, транзакционно, отказ на пропуске и на неизвестной версии. `stripSQLComments` уже удалён.

**8. Зависимости.** Перечислены в журнале (раздел «Зависимости»). Сетевые/криптографические: `webpush-go` (+`golang-jwt/jwt/v5`), `golang.org/x/crypto`, stdlib `net/http`, `net/smtp`, `crypto/tls`; на клиенте — `leaflet` (плитки), `exifr`/`mediabunny` (парсеры недоверенного ввода).

## Проверено, в порядке

- **SEC-4** выполнен: `push/endpoint.go:26-33` — свой клиент, 10 с, редиректы запрещены; `validateEndpoint:43-73` — только https, фильтр private/loopback/link-local/CGNAT; `SubscriberMailto` константа, импорта `auth` в `push` нет.
- **SEC-5** выполнен: `mail.go:309-325` — STARTTLS обязателен вне loopback; `headerSafe:372-378` на From/To/Subject; `loginAuth.Start` отказывает без TLS (`login.go:14`).
- **SEC-1** (порядок bootstrap) — токен → пароль → `Probe` → одна транзакция → `WritePublicURL` → `SendTest`; сбой письма не откатывает (`admin.go:47-129`). Единственное отклонение — Н-7 (валидация URL поздно).
- **MIG-1** и удаление `stripSQLComments` — `migrate.go` соответствует.
- **API-4/API-5/QLT-4** — `PublicURL()` под мьютексом, `writeFileAtomic`, `ValidatePublicURL` строгий, лог о перекрытии `WYND_PUBLIC_URL`, `config.Load` не создаёт `config.json`.
- `GET /admin/smtp` не эхо-ит пароль; ни один ответ API не содержит `smtp_password`/`vapid_private_key`.
- `clientIP` (`respond.go:151-174`): XFF учитывается только от доверенного пира, берётся самый правый недоверенный адрес — подмена бакета лимитера клиентским XFF невозможна при любом из трёх сниппетов прокси.
- `ProbeTLS`, SMTP TLS — без `InsecureSkipVerify`; `ProbeHTTPRedirect` не читает тело и не следует редиректам.
- `admin/proxy/{kind}` — шаблоны в коде, `kind` из белого списка, иначе 404.
- Все маршруты области под нужными шлюзами (`server.go:75-76` public+лимитер токена; `:97-103` admin; `:171-172` paid).
- `ConfirmBootstrapToken` — лимитер по IP, `ConstantTimeCompare`, отказ при `bootstrapped=1`.
- `Subscribe` защищает от угона чужого endpoint (`push.go:127-132`); `Unsubscribe` — только свои.
- Bootstrap-токен: `crypto/rand` 32 байта, файл 0600, каталог `keys` 0750.
- `config.json` без секретов; `writeFileAtomic` — temp 0600, удаление temp в `defer`.
- `.gitignore`/`.dockerignore` закрывают `dev/` (живые SMTP-учётки тестов), `data/`; в `testdata/` только примеры.
- Push-payload не содержит текста, имён, e-mail и ссылок; письма не содержат содержимого записей.
- `install.sh` не скачивает ничего по http и не исполняет удалённые скрипты.
- Логи `main.go`, `jobs/*`, `notify.go` — без токенов, кодов, паролей и XFF; e-mail только в loopback-фолбэке.
- `dev-auth-codes.log` — 0600, каталог 0700, только на loopback.
- Docker-образ: пользователь `wynd`, в финальной стадии — только бинарник и CA.
