# Ревью: internal/api (handlers), cmd/wynd, internal/config, web

Статус: В РАБОТЕ (файл дополняется по ходу).

## Безопасность

### S1. [high, CONFIRMED] `POST /admin/bootstrap` меняет config.json, SMTP-релей и флаг loopback ДО проверки токена — и после завершённого bootstrap тоже
- Где: `internal/api/admin.go:40-96`. Порядок шагов: `readJSON` (42) → `config.WritePublicURL` (58) → `s.applyPublicURL` (62) → `s.Mail.SaveConfig` (69) → `s.Mail.SendTest` (79) → и только потом `s.Auth.Bootstrap` (84), внутри которого `gateBootstrapToken` и проверка `bootstrapped == 1` (`internal/auth/admin.go:64, 80`).
- Маршрут публичный (`server.go:68`, обёртка только `limitBody`). `mail.SaveConfig` (`internal/mail/mail.go:99-115`) ничего не знает про токен и про `bootstrapped`.
- Сценарий: на давно работающем инстансе аноним шлёт `{"token":"x","password":"xxxxxxxx","public_url":"https://a.example","host":"evil.example","port":25,"smtp_password":"p","from":"a@evil.example"}`. Ответ будет 400/502, но к этому моменту: (а) `config.json` переписан чужим `public_url`; (б) в `instance_settings` лежит SMTP-релей атакующего; (в) сервер сходил на `evil.example:25` (SSRF). Дальше `POST /auth/code` на почту любого участника — код входа уходит через релей атакующего → захват любой участнической учётки. Вариант `public_url=http://127.0.0.1` включает `Loopback=true` у `Auth`/`Mail` (коды в лог, `code_delivery=log`).
- Исправление: первым действием хендлера после `readJSON` вызвать `s.Auth.ConfirmBootstrapToken(ctx, body.Token, s.BootstrapToken, s.clientIP(r), now)` (он же проверяет `bootstrapped`), затем провалидировать пароль (`MinPasswordLen`), и только после этого писать `config.json`/SMTP. Регрессионный тест: запрос с неверным токеном и заполненными `public_url`+SMTP → 400, `config.json` и `instance_settings.smtp_host` не изменились.

### S2. [medium, CONFIRMED] Bootstrap не атомарен: при ошибке после записи конфигурация остаётся «наполовину»
- Где: `internal/api/admin.go:57-94`. Даже с верным токеном: `WritePublicURL` и `SaveConfig` уже выполнены, а `SendTest` вернул 502 или `Auth.Bootstrap` вернул `weak_password` — в БД остаётся нерабочий релей, в `config.json` — новый адрес, инстанс не bootstrapped.
- Цена: оператор видит «ошибка», а `GET /instance` уже отдаёт `code_delivery=mail` с битым релеем; повторный bootstrap на loopback без SMTP не откатывает релей.
- Исправление: порядок «токен → валидация пароля → `mail.Probe` несохранённого конфига → `Auth.Bootstrap` → `SaveConfig` → `WritePublicURL`/`applyPublicURL` → `SendTest` (ошибка письма — в `smtp_last_error`, не в HTTP-статус)».

### S3. [low, CONFIRMED] `POST /admin/bootstrap/smtp-test` — dial произвольного host:port, защищён только токеном и общим login-лимитером
- Где: `internal/api/admin.go:98-127`. Токен проверяется ДО `Probe` (104) — анонимного SSRF здесь нет, после bootstrap маршрут закрыт (`ErrInvalid`). Остаток: владелец токена может сканировать внутреннюю сеть (это оператор — приемлемо).
- Замечание: `gateBootstrapToken` не вызывает `loginLimiter.reset` при успехе в `ConfirmBootstrapToken` — не ошибка, но успешные пробы не копят штраф, неуспешные делят ведро с `AdminLogin` (`internal/auth/admin.go:26-36,124`): перебор токена блокирует вход админа с того же IP. Оставить как есть, дописать комментарий.

### S4. [low, CONFIRMED] Гонка данных на `Server.PublicURL/Loopback` и `mail.Service.loopback`
- Где: `internal/api/admin.go:129-138`, `internal/mail/mail.go:68-70`. Поля пишутся из хендлера без синхронизации, читаются из других горутин (`handleInstance`, `SendCode`).
- Цена: `go test -race` на параллельном сценарии «PUT /admin/access + GET /instance» упадёт; на практике — редкое чтение старого значения.
- Исправление: `atomic.Bool` для loopback в `mail.Service`/`auth.Service`, `atomic.Pointer[string]` (или `sync.RWMutex`) для `Server.PublicURL`.

### S5. [low, CONFIRMED] Двойной bootstrap: проверка `bootstrapped` вне транзакции
- Где: `internal/auth/admin.go:74-101`. `SELECT bootstrapped` идёт до `BeginTx`, `UPDATE ... SET bootstrapped = 1` без условия `WHERE bootstrapped = 0`.
- Сценарий: два параллельных запроса с верным токеном оба проходят проверку, оба коммитят; пароль и имя — того, кто закоммитил последним, оба получают 200. Нужен верный токен, поэтому low.
- Исправление: `UPDATE instance_settings SET ... WHERE id = 1 AND bootstrapped = 0`, `RowsAffected()==0` → `ErrInvalid`; SELECT убрать.

## Ошибки и граничные случаи

### E1. [high, CONFIRMED] Открытый SSE-поток делает каждое выключение «грязным»: 10 с ожидания, затем `log.Fatalf` без `st.Close()` и `WaitNotify`
- Где: `internal/api/sync.go:63-83` (цикл выходит только по `r.Context().Done()`), `cmd/wynd/main.go:135-168` (нет `BaseContext`/`RegisterOnShutdown`; `srv.Shutdown(ctx)` → ошибка → `log.Fatalf("shutdown: %v")` на 166).
- `http.Server.Shutdown` не отменяет контексты активных запросов. Пока у кого-то открыта вкладка, `Shutdown` ждёт все 10 с и возвращает `context deadline exceeded`; `log.Fatalf` делает `os.Exit(1)` — отложенный `st.Close()` (53-57) и `apiSrv.WaitNotify` (168) не выполняются, systemd видит код 1.
- Исправление: в `main.go` завести `baseCtx, baseCancel := context.WithCancel(context.Background())`, `srv.BaseContext = func(net.Listener) context.Context { return baseCtx }`, вызвать `baseCancel()` сразу после сигнала (до `Shutdown`); ошибку `Shutdown` логировать через `log.Printf` и продолжать к `WaitNotify` и `st.Close()`. Тест: открыть SSE, послать отмену базового контекста — хендлер возвращается < 100 мс.

### E2. [medium, CONFIRMED] SSE не перепроверяет сессию: заблокированная/удалённая/разлогиненная учётка и истёкшая подписка продолжают получать события
- Где: `internal/api/sync.go:52-84`. Сессия и `requirePaidSession` проверяются один раз в middleware (`admin.go:275-283`). `SyncEvents` (`internal/chronicle/sync.go:27-46`) фильтрует только по `membership_spans` — конец членства отрабатывает корректно, а `accounts.blocked`, `sessions` revoke и `ErrPaymentRequired` — нет.
- Сценарий: админ блокирует учётку (`POST /admin/accounts/{id}/block`) — открытая вкладка продолжает получать `summary` и `payload` новых событий, пока не закроется соединение.
- Исправление: в цикле `syncSSE` перед каждым опросом после паузы вызывать `s.Auth.IsParticipantSession(ctx, token)` + `s.requirePaidSession(r)`; при ошибке — `return` (клиент переподключится и получит 403). Тест: открыть SSE, заблокировать учётку, создать пост — событие в поток не приходит, поток закрыт.

### E3. [medium, CONFIRMED] SSE без heartbeat и без write-deadline
- Где: `internal/api/sync.go:69-82`, `cmd/wynd/main.go:135-142` (`WriteTimeout` нет — намеренно).
- (а) В тихом круге в поток не пишется ничего: прокси с read timeout 300 с (`internal/proxy.ReadTimeoutSeconds`) рвёт соединение каждые 5 минут. (б) Ошибка `fmt.Fprintf` (71) игнорируется; клиент, переставший читать, блокирует запись навсегда — горутина и опрос SQLite висят до TCP keepalive.
- Исправление: `rc := http.NewResponseController(w)`; перед каждой записью `rc.SetWriteDeadline(time.Now().Add(30*time.Second))`; при ошибке записи — `return`; раз в 25 с простоя писать комментарий `: ping\n\n`.

### E4. [low, CONFIRMED] Нет потолка одновременных SSE-потоков на учётку
- Где: `internal/api/sync.go:52`. Каждый поток = запрос к SQLite раз в 2 с. 20 вкладок = 10 запросов/с на холостом ходу. Для домашнего инстанса терпимо.
- Исправление: `map[accountID]int` под мьютексом в `Server`, потолок 8 потоков на учётку, сверх — 429 `rate_limited`.

### E5. [low, CONFIRMED] `cursor` с мусором молча превращается в 0 → полный реплей
- Где: `internal/api/sync.go:21`. `strconv.ParseInt` ошибка отброшена; `?cursor=abc` отдаёт всю хронику вместо 400.
- Исправление: непустой и неразобранный `cursor` → `chronicle.ErrInvalid`.

### E6. [medium, CONFIRMED] Фоновые задачи шлют письма со старым `public_url` до рестарта
- Где: `cmd/wynd/main.go:102,113` — `cfg.PublicURL` захвачен строкой при старте; `PUT /admin/access` и bootstrap меняют только `apiSrv.PublicURL` (`internal/api/admin.go:129-138`).
- Сценарий: установка на loopback → bootstrap задаёт `https://wynd.example` → письма архивного цикла (`jobs.RunArchiveJobs(..., publicURL, ...)`) несут ссылку `http://127.0.0.1:7676/...` до перезапуска.
- Исправление: передавать в `maybeRunRoutine` геттер `func() string` (метод `apiSrv.CurrentPublicURL()` под тем же мьютексом, что в S4).

### E7. [low, CONFIRMED] Остановка не ждёт фоновую рутину
- Где: `cmd/wynd/main.go:104-116,159`. `close(maintDone)` не дожидается идущего `maybeRunRoutine` (контекст `Background`), после чего `st.Close()` закрывает БД под purge/удалением файлов.
- Исправление: рутине — контекст, отменяемый по сигналу; `sync.WaitGroup` на горутину, `Wait()` перед `st.Close()`.

### E8. [medium, CONFIRMED] `config.json` пишется не атомарно; env молча перекрывает то, что сохранила панель
- Где: `internal/config/public_url.go:52` и `internal/config/config.go:136` — `os.WriteFile` (truncate + write) без temp-файла и rename. `config.go:78-80` — `WYND_PUBLIC_URL` перекрывает файл.
- Сценарии: (а) падение/отключение питания между truncate и write → пустой `config.json` → `parse config` → сервер не стартует (`main.go:41`). (б) Docker с `WYND_PUBLIC_URL`: `PUT /admin/access` пишет новый адрес в файл, отвечает 200, после рестарта env возвращает старый — без единой строки в логе. (в) два параллельных `WritePublicURL` (read-modify-write без блокировки) теряют запись.
- Исправление: один хелпер `writeFileAtomic(path, payload, 0o600)` (temp в том же каталоге → `Sync` → `os.Rename`) под пакетным `sync.Mutex`; в `Load` при заданном `WYND_PUBLIC_URL`, отличном от файла, писать `log.Printf("config: WYND_PUBLIC_URL overrides config.json")`, а `GET /admin/access` отдавать `public_url_locked: true`.

### E9. [low, CONFIRMED] `public_url` не валидируется
- Где: `internal/config/public_url.go:13-22`. `NormalizePublicURL` принимает любую строку с `://` (`ftp://x`, `javascript://`, `https://` без хоста, строку с путём и query). Значение уходит в письма, QR и проверки.
- Исправление: `url.Parse`, схема только `http`/`https`, непустой `Host`, пустые `Path`/`RawQuery`/`Fragment`; иначе `ErrInvalid`. Функция должна возвращать `(string, error)`.

### E10. [low, CONFIRMED] `wynd backup` создаёт пустую БД и config при неверном `WYND_DATA_DIR`
- Где: `cmd/wynd/backup.go:19` → `config.Load()` → `writeFileConfig` (`config.go:58`) и `ensureDataLayout` (`config.go:102-120`, создаёт `wynd.db` нулевой длины).
- Сценарий: cron без env запускает `wynd backup /mnt/b` из другого cwd → создаётся `./dev/data/wynd.db`, «бэкап» пустой базы завершается успехом.
- Исправление: разделить `config.Load()` (чистое чтение, без записи) и `config.EnsureLayout(cfg)`; второй звать только из `runServer`. В `runBackup` при отсутствии `wynd.db` — `log.Fatalf`.

### E11. [low, CONFIRMED] Раздача блоба: нет `Cache-Control`, имя файла в `Content-Disposition` собирается конкатенацией
- Где: `internal/api/blobs.go:143-148`. `sanitizeFilename` (`internal/blob/filename.go:11-24`) не убирает `"` и `\`, кириллица уходит сырыми байтами — заголовок `filename="фото "1".jpg"` невалиден. Блоб неизменяем по id, но каждый показ ленты качает его заново (клиент тянет через `fetch` с Bearer).
- Исправление: `mime.FormatMediaType("attachment", map[string]string{"filename": info.Filename})` (даёт RFC 2231 `filename*=`), плюс `Cache-Control: private, max-age=31536000, immutable`; отдачу заменить на `http.ServeContent(w, r, "", time.Time{}, f)` — даёт Range и корректный HEAD.

### E12. [low, CONFIRMED] Неизвестные пути `/api/*` и чужие методы отвечают text/plain, не JSON
- Где: `cmd/wynd/main.go:127` + `internal/api/server.go:188-191`. SPA-fallback до `/api/` не доходит (хорошо), но `ServeMux` сам пишет `404 page not found` / `405 Method Not Allowed` как text/plain; клиентский `res.json()` падает с SyntaxError вместо `not_found`.
- Исправление: в `Server.ServeHTTP` — `h, pattern := s.Mux.Handler(r); if pattern == "" { writeJSON(w, 404, {"error":"not_found"}); return }`.

### E13. [medium, CONFIRMED] Хешированные бандлы `_app/immutable/*` отдаются с `no-cache`, а у embed.FS нет ни `Last-Modified`, ни `ETag`
- Где: `web/serve.go:58-61`. У файлов `embed.FS` нулевой `ModTime` → `http.FileServer` не ставит `Last-Modified`; `ETag` никто не ставит. `no-cache` без валидатора = полная перекачка всех чанков на каждой загрузке, когда service worker ещё не установлен/обновляется. Обоснование в комментарии («hash can stay stable») для `_app/immutable/` неверно: имя — хеш содержимого.
- Исправление: `strings.HasPrefix(name, "_app/immutable/")` → `Cache-Control: public, max-age=31536000, immutable`; `_app/version.json` и прочее — `no-cache`.

### E14. [low, CONFIRMED] SPA отдаёт листинг каталогов embed.FS
- Где: `web/serve.go:25-33`. `fs.Stat(root, "_app")` успешен для каталога → `http.FileServer` рисует листинг `/_app/`, `/fonts/`, `/_app/immutable/`.
- Исправление: `if info, err := fs.Stat(root, name); err == nil && !info.IsDir()`; каталог — в SPA-fallback.

### E15. [medium, CONFIRMED] `DELETE .../posts/{post_id}` — четыре несвязанных шага вне транзакции
- Где: `internal/api/journal.go:173-194`. `DeletePost` коммитится, затем отдельно `DeletePostMedia`, `RemoveRefsFor`, `ReleaseBlobs`. Сбой любого шага после первого → клиент получает 500, хотя запись уже удалена; строки `post_media` остаются, а `gcBlobIfUnreferenced` (`internal/blob/serve.go:190-200`) считает их ссылкой → файлы не освобождаются никогда, квота круга не возвращается.
- Исправление: один метод `Server.deletePostWithMedia(ctx, ...)` по образцу `editPostReplaceMedia` (`journal.go:473-533`): одна `tx` на `DeletePostInTx` + `DeletePostMediaInTx` + `RemoveBlobRef(tx)`, после коммита — `ReleaseBlobs`, ошибка которого только логируется (ответ 200).

### E16. [low, CONFIRMED] PATCH поста с `media` молча игнорирует `cover_blob_id`; без `media` — правка и обложка в двух транзакциях
- Где: `internal/api/journal.go:134-162`. Ветка `body.Media != nil` возвращает 200, не посмотрев на `CoverBlobID`. В другой ветке `EditPost` закоммичен, а `SetPostCover` вернул 4xx — клиент видит ошибку при уже применённой правке.
- Исправление: `cover_blob_id` вместе с `media` → `invalid`; в ветке без `media` — `EditPostInTx` + `SetPostCoverInTx` в одной транзакции.

### E17. [low, CONFIRMED] Проверка квоты медиа вне транзакции создания записи
- Где: `internal/api/journal.go:93-101` и `492-498`. `CheckMediaQuota` → затем отдельная `tx`. Два параллельных поста проходят проверку оба и вместе превышают квоту. Для домашнего инстанса цена мала.
- Исправление: перенести `CheckMediaQuota` внутрь `createPostWithMedia`/`editPostReplaceMedia` после `BeginTx` (версия с `tx`), транзакцию открывать `BEGIN IMMEDIATE`.

## Мёртвый код и legacy

### D1. [low, CONFIRMED] `rollbackNewPost` не вызывается
- Где: `internal/api/journal.go:535-554`. Grep по репозиторию: единственное вхождение — определение. Остаток от времени, когда пост и медиа создавались двумя шагами; сейчас это одна транзакция (`createPostWithMedia`).
- Исправление: удалить функцию.

### D2. [low, CONFIRMED] Самодельный `itoa` при импортированном `strconv`
- Где: `internal/api/respond.go:117-129`, единственный вызов — `respond.go:106`. `strconv` уже импортирован (строка 9). Буфер `[4]byte` к тому же паникует на числе длиннее 4 цифр.
- Исправление: `strconv.Itoa(bits)`, функцию удалить.

### D3. [low, CONFIRMED] `createPostWithMedia` и `editPostReplaceMedia` принимают неиспользуемые параметры
- Где: `internal/api/journal.go:445` — `circleID, accountID` не читаются (те же значения лежат в `in`).
- Исправление: убрать параметры из сигнатуры `createPostWithMedia`.

### D4. [info] `ttl_days` — вхождений в Go-коде нет; legacy-алиас уже вычищен.


## Гейты маршрутов (п. 2 задания) — промежуточные выводы

- Таблица маршрутов: `internal/api/server.go:56-186`. Все `/admin/*` кроме `bootstrap`, `bootstrap/smtp-test`, `login` — под `requireAdmin`. Все журнальные — под `RequirePaidParticipant`; `auth/logout` и `pay/*` — под `RequireParticipant` (намеренно, чтобы можно было заплатить). Пропущенных middleware в таблице нет.
- Членство/владелец/can_write в хендлерах НЕ проверяются — хендлер передаёт `sess.AccountID` в `chronicle`, гейт живёт там (`requireReader`, `requireWriter`, `RequireSettings`, `RequireOwner`). IDOR между кругами закрыт в домене: `post.CircleID != circleID → ErrNotFound` (`internal/chronicle/content.go:136,207,562`).

### G1. [medium, PLAUSIBLE] Правка и удаление своей записи/комментария не требуют `can_write`: вышедший и исключённый продолжают «писать»
- Где: `internal/chronicle/content.go:142-151` (`EditPostInTx`), `213-222` (`DeletePost`), `568-577` (`EditComment`), аналогично `DeleteComment` (604+). Используется `c.membership(...)` — он возвращает строку при любом `status` (`internal/chronicle/member.go:303-317`), а не `requireWriter` (`content.go:342-355`), которым закрыты `CreatePost`/`CreateComment`/`SetReaction`/`SetDayTitle`.
- Сценарий: участника исключили (`status=gone`), сессия учётки жива. `PATCH/DELETE /circles/{id}/posts/{post_id}` проходит: `mem.IdentityID == post.IdentityID`, окно правок `nil` (без ограничения) → исключённый переписывает или стирает ветку записи вместе с чужими комментариями (`scrubPostBranch`). Инвариант справочника «вышедший с доступом видит до выхода, не пишет» нарушен.
- Тест: участник создаёт пост, владелец делает `Exclude`, затем `EditPost`/`DeletePost`/`EditComment`/`DeleteComment` от имени исключённого → ожидается `ErrForbidden`.
- Исправление: в четырёх функциях заменить `c.membership` на `c.requireWriter(ctx, circleID, accountID, now)`.

### G2. [low, CONFIRMED] `{post_id}` в маршрутах комментария не используется
- Где: `internal/api/journal.go:226-261` — `handleEditComment`/`handleDeleteComment` читают только `circle_id` и `comment_id`. `PATCH /circles/A/posts/ЛЮБОЙ/comments/X` работает. Утечки нет (круг комментария сверяется в домене), но URL лжёт.
- Исправление: передавать `postID` в `EditComment`/`DeleteComment` и сверять `comment.PostID != postID → ErrNotFound`.

### G3. [low, CONFIRMED] `PATCH /circles/{id}` — до пяти отдельных транзакций, частичное применение
- Где: `internal/api/circle_settings.go:60-90`. Ошибка на `color` после успешного `name` → 400, имя уже изменено и событие в хронике записано.
- Исправление: один метод `Chronicle.PatchCircle(ctx, circleID, accountID, PatchCircleInput, now)` с одной транзакцией; валидация всех полей до первой записи.

