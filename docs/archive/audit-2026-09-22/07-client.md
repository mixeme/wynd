# 07 — Клиент SvelteKit `web/src` (SEC-6 из плана 42)

Аудит безопасности, 2026-09-22. Область: XSS, хранение токена, инвайт-ссылки, IDB/офлайн, service worker, fetch-обёртка, карта, админка, ui-guard.

Итог: критических и высоких — 0; средних — 2 (DoS ленты дубликатом упоминания; офлайн-очередь переживает выход и уходит под чужой учёткой); низких — 4; заметок — 3. XSS-стоков с пользовательскими данными не найдено.

## Находки

### Повтор упоминания в тексте записи роняет ленту у всех читателей (дубликат ключа `{#each}`)
- Серьёзность: средняя
- Уверенность: подтверждено кодом (проверено по исходнику Svelte 5.56.10 `web/node_modules/svelte/src/internal/client/dom/blocks/each.js:355-361` и по собранному `web/dist/_app/immutable/chunks/*.js`: в production `each_key_duplicate` тоже бросает `Error`)
- Где: `web/src/routes/circles/[id]/+page.svelte:552` (лента), `web/src/routes/circles/[id]/days/[date]/+page.svelte:237` (день), `web/src/routes/circles/[id]/posts/[postId]/+page.svelte:365,454,475` (запись, комментарии, очередь комментариев); источник частей — `web/src/lib/journal/mentions.ts:35-54` (`splitMentionBody`).
- Атакующий: любой участник с правом записи (против всех читателей круга); автор комментария — против всех, кто открывает запись.
- Что: ключ элемента — `part.kind + part.value`. Тело `«@Аня привет @Аня»` даёт две части `mention:@Аня`; тело `«@a x @b x »` — две одинаковых текстовых части `text: x `. Svelte при дубликате ключа бросает исключение во время рендера, `<svelte:boundary>`/`+error.svelte` в проекте нет (grep пустой) — экран ленты/дня/записи не отрисовывается, пока запись существует. Сервер текст не нормализует (упоминания — только клиентский рендер), так что запись остаётся и ломает экран всем до её удаления автором/владельцем. Сравни `web/src/lib/components/forms/TextArea.svelte:38` — там ключ `(i)`, безопасно.
- Как воспроизвести: опубликовать запись с текстом `@x @x` (или комментарий) — открыть ленту круга в собранном клиенте: консоль `Error: https://svelte.dev/e/each_key_duplicate`, содержимое ленты пустое.
- Исправление: ключ `(i)` или `` (`${i}:${part.kind}`) `` во всех пяти местах; при желании обернуть тело записи в `<svelte:boundary>`.

### Офлайн-очередь не привязана к учётке и не очищается при выходе: чужая запись уходит под новым входом
- Серьёзность: средняя
- Уверенность: подтверждено кодом
- Где: `web/src/lib/idb/db.ts:79-103` (`QueueRecord` — есть `origin`, `circle_id`, нет `account_id`), `web/src/lib/session/session.svelte.ts:65-74` (`dropParticipantSession` удаляет сессию, курсор, снимки — очередь, `media`, `pins`, `groups`, `settings.circle_meta` не трогает), `web/src/routes/settings/servers/+page.svelte:62-67` (выход), `web/src/lib/queue/queue.ts:251-293` (401 → `pending`, комментарий «после входа запись должна уйти»), `:295-314` (`drainOnce` берёт все записи очереди и шлёт через `apiFetch(item.origin, …)` — токен той сессии, которая сейчас лежит в `sessions[origin]`), `web/src/lib/api/client.ts:34-39`.
- Атакующий: пользователь A общего устройства против пользователя B (или наоборот — B невольно публикует от своего имени текст и фото A).
- Что: A пишет запись/комментарий/реакцию офлайн (или сервер недоступен), выходит из учётки; запись остаётся в IDB со состоянием `pending` вместе с телом и байтами фото. B входит на тот же сервер на этом устройстве — `initQueueDrain`/`startSync → drainQueue` отправляет запись A с токеном B: она публикуется как запись B в круге A (если B — участник того же круга; иначе 403/404 → `failed`, но содержимое остаётся в IDB и видно B в «ожидающих» экранах круга). Отдельно: после выхода на общем компьютере в IDB остаются тексты и фото неотправленных записей, весь кэш медиа (`media`, чистится только вручную в `routes/settings/app/+page.svelte:80`), закреплённые круги и имена в круге (`circle_meta`) — читается любым, кто откроет devtools.
- Как воспроизвести: (1) войти как A, включить офлайн, опубликовать запись с фото; (2) «Серверы → выйти»; (3) включить сеть, войти как B (участник того же круга); (4) запись появляется в круге от имени B.
- Исправление: в `dropParticipantSession(origin)` удалять записи очереди, медиа-кэш (`mediaKey` c префиксом `${origin}:`), `pins`/`circle_meta` этого origin; дополнительно хранить `account_id` в `QueueRecord` и в `drainOnce` пропускать/помечать `failed` записи, чей `account_id` не совпадает с текущей сессией origin.

### Вставленная инвайт-ссылка с чужого сервера: токен уходит на текущий сервер
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `web/src/lib/auth/links.ts:2-16` (`parseWyndLink` отбрасывает host и возвращает только путь `/invite/<token>` / `/join/<token>`), `web/src/routes/join/+page.svelte:71-92,94-111` (paste, буфер обмена, QR → `goto(path)`), `web/src/routes/invite/[token]/+page.svelte:49` и `web/src/routes/join/[token]/+page.svelte:36` (`fetchInvitePeek('', token)` — origin всегда текущий).
- Атакующий: администратор сервера A (или тот, кто читает его журнал запросов) против участника, получившего приглашение на сервер B.
- Что: пользователь, открывший SPA на сервере A, вставляет в поле адреса / читает по QR ссылку `https://B/invite/<token>`; клиент игнорирует хост и делает `GET A/api/v1/invites/<token>`. Токен приглашения в круг на B оказывается в логах A (в пути запроса). Ответ A будет 404, пользователь увидит «приглашение недействительно», не понимая причины. Держатель токена может сам присоединиться к кругу на B.
- Как воспроизвести: открыть `https://A/join`, вставить `https://B/invite/<tok>` — в access-log A появится `/api/v1/invites/<tok>`.
- Исправление: в `parseWyndLink` сравнивать `url.origin` с `window.location.origin`; при несовпадении не переходить в SPA, а показать «ссылка ведёт на другой сервер — откройте её напрямую» (или `location.assign(url)`).

### Ссылка-приглашение в круг строится от хоста SPA, а не от origin круга
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `web/src/routes/circles/[id]/settings/invite/+page.svelte:61-64,98` и `web/src/routes/circles/[id]/settings/invites/+page.svelte:40-43,50` (`inviteUrlFor` = `window.location.origin + /invite/<token>`), при этом токен создаётся на `circle.origin` (`:92`, `:46`).
- Атакующий: администратор сервера A (хост SPA) — пассивно; либо просто дефект работы мульти-серверного клиента.
- Что: клиент поддерживает несколько серверов (сессии по `origin`, круг помнит свой `origin` — `lib/circles/origin.ts`). Если участник открыл SPA на A и создаёт приглашение в круг, живущий на B, ссылка/QR получаются `https://A/invite/<tokenB>`. Получатель открывает её → `GET A/api/v1/invites/<tokenB>` → 404 «приглашение недействительно», а токен круга на B оказывается в логах A. Для админ-инвайтов (`routes/admin/access/+page.svelte:56-59`) проблемы нет — админка всегда same-origin.
- Как воспроизвести: две сессии (A = хост SPA, B — другой сервер), открыть круг на B → «Пригласить» → сравнить хост в ссылке с `circle.origin`.
- Исправление: `const base = circle.origin || window.location.origin` в обоих `inviteUrlFor` (то же для `navigator.share`).

### Одинаковые имена участников роняют экран «Кто уже здесь» и полоску людей
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `web/src/routes/invite/[token]/+page.svelte:110` и `web/src/routes/circles/[id]/join/+page.svelte:202` (`{#each peek.members … (member.name)}`), `web/src/lib/components/forms/PeopleStrip.svelte:16` (`(person.name)`); сервер отдаёт `identityName` без проверки уникальности (`internal/chronicle/invite_peek.go:68,103-110`).
- Атакующий: участник круга (выбирает себе имя, совпадающее с чужим).
- Что: два участника с одинаковым отображаемым именем → дубликат ключа → та же ошибка `each_key_duplicate`, список участников по инвайту / экран присоединения не отрисовывается. Та же природа у других ключей из пользовательских данных: `lib/components/data/ReactionBar.svelte:31` (`group.names + group.icon`), `routes/circles/[id]/settings/identity/+page.svelte:172` (`row.effective_at`).
- Как воспроизвести: два участника ставят имя «Аня», третий открывает `/invite/<token>?members=1`.
- Исправление: ключи по индексу или по `account_id` там, где он есть (в `InvitePeekMember` его нет — добавить индекс `i`).

### Push-подписка не снимается при выходе: устройство продолжает получать сигналы учётки
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `web/src/lib/push/push.ts:58-65` (`unsubscribePush` объявлена, но не вызывается ни в одном месте — grep пустой), `web/src/lib/session/session.svelte.ts:65-74` и `web/src/routes/settings/servers/+page.svelte:62-67` (выход — без отписки); сервер: `internal/api/auth.go:89-95` (`handleLogout` только отзывает сессию), `internal/push/push.go:165-172` (подписки привязаны к `account_id`, не к сессии).
- Атакующий: следующий пользователь общего/утерянного устройства — пассивно.
- Что: после «выйти» подписка Web Push остаётся и в браузере, и в `push_subscriptions` на сервере; сервер продолжает слать на это устройство сигналы учётки (`internal/api/notify.go:155-159,184-188`: `circle_id`, тип, счётчик; `internal/jobs/pay.go:52-55` — название сервера и дата подписки). Содержимое минимальное, но факт активности в кругах и срок оплаты утекают на устройство, с которого пользователь вышел. Смежно: `initPush` (`push.ts:68-81`) отдаёт одну и ту же подписку (созданную с VAPID-ключом первого сервера) всем серверам — для остальных доставка будет отвергнута push-сервисом.
- Как воспроизвести: войти на устройстве с включёнными уведомлениями, выйти через «Серверы», с другого устройства написать в круг — на первом приходит push.
- Исправление: в `dropParticipantSession`/`logout` вызывать `unsubscribePush(origin)` до отзыва токена; на сервере при `RevokeSession` (или при удалении последней сессии учётки) чистить `push_subscriptions` учётки — или хранить `session_id` в подписке.

### Карта: плитки OpenStreetMap — IP участника и район его фотографий уходят третьей стороне
- Серьёзность: заметка (принятый архитектурный выбор; в документации явно не зафиксирован)
- Уверенность: подтверждено кодом
- Где: `web/src/routes/circles/[id]/map/+page.svelte:84-89` (`https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png`).
- Атакующий: третья сторона (оператор тайлов) / наблюдатель сети.
- Что: при открытии карты браузер участника запрашивает плитки у `tile.openstreetmap.org`: OSM видит IP участника, `Referer` = origin сервера Wynd (благодаря `Referrer-Policy: strict-origin-when-cross-origin`, `web/serve.go:72`; сам путь/id круга не уходит) и координаты просматриваемой области (z/x/y), то есть район, где сняты фото круга. `referrerPolicy` у слоя не задан — заголовка сервера достаточно. В `docs/plans/42-code-review.plan.md:171` (LIC-4) отмечена только атрибуция/поддомены; приватностный аспект в справочнике клиента не описан.
- Как воспроизвести: открыть `/circles/<id>/map` с DevTools → Network: запросы на `*.tile.openstreetmap.org`.
- Исправление: зафиксировать в `docs/reference/client-reference.md` как принятый риск; опционально — настраиваемый URL тайлов (свой прокси) в админке.

### `http://` для удалённого сервера принимается без предупреждения
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `web/src/lib/auth/origin.ts:17-34` (`parseServerInput` подставляет `https://` только когда схема не указана; явный `http://host` проходит), `web/src/routes/join/+page.svelte:34-53`.
- Атакующий: наблюдатель сети.
- Что: если пользователь введёт `http://example.org`, код входа и Bearer-токен пойдут открытым текстом. Клиент об этом не предупреждает. По умолчанию — `https`, loopback (`http://127.0.0.1`) для разработки нужен, поэтому полный запрет нежелателен.
- Исправление: для не-loopback `http://` показывать предупреждение или отклонять (`isLoopbackPublicURL` уже есть).

### Токен приглашения хранится в `sessionStorage` и остаётся в истории вкладки
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `web/src/lib/auth/pending.ts:19-38` (`wynd:pending-auth` c `inviteToken`), `web/src/lib/auth/invites.ts:53-68` (`wynd.inviteJoin.<circleId>`), `web/src/routes/invite/[token]/+page.svelte:93-99` (`goto` с токеном в пути без `replaceState`), `web/src/routes/auth/code/+page.svelte:73-76`, `web/src/routes/admin/bootstrap/+page.svelte:21,127` (`?token=` bootstrap в истории; после `completeBootstrap` — `goto('/admin/check')` без `replaceState`, токен одноразовый).
- Атакующий: локальный пользователь общего компьютера.
- Что: токен инвайта лежит в `sessionStorage` (живёт до закрытия вкладки; `clearPendingAuth` вызывается после успешного кода — `auth/code/+page.svelte:73`; `wynd.inviteJoin.<id>` очищается при завершении присоединения — `circles/[id]/join/+page.svelte:162`). История браузера хранит `/invite/<token>` — `replaceState` не применяется. `Referer` наружу не утекает: `web/serve.go:72` ставит `Referrer-Policy: strict-origin-when-cross-origin`. Риск ограничен: токен принадлежит самому получателю, `sessionStorage` не разделяется между вкладками; `localStorage` не используется.
- Исправление: не требуется; при желании — `goto(..., { replaceState: true })` после успешного `verifyCode`/`completeBootstrap`.

## Проверено, в порядке

- **XSS.** `{@html}` — ровно три вхождения: `routes/+layout.svelte:41` (`pwaInfo.webManifest.linkTag` — строка, сгенерированная плагином на сборке), `routes/admin/access/+page.svelte:238` и `routes/circles/[id]/settings/invite/+page.svelte:155` — SVG от `QRCode.toString(url, {type:'svg'})`: библиотека выводит только `<path d="…">`, кодируемая строка в разметку не попадает. `innerHTML`/`insertAdjacentHTML`/`document.write`/`eval`/`new Function`/`srcdoc`/`outerHTML`/`bind:innerHTML` — нет вхождений в `web/src`.
- Рендер текста записей/комментариев: `splitMentionBody` → `{part.value}` (экранирование Svelte), упоминания — `<span class="men">`, не ссылки; автолинковки URL нет, `href` с пользовательскими данными нет (единственные `<a href>` — статичные ссылки на GitHub). `a.href = url` только для blob: URL с `download` (`lib/media/objectUrl.ts:33-36`, `lib/journal/posts.ts:184-186`, `lib/admin/admin.ts:270-272`); `downloadArchive` (`posts.ts:170-188`) пропускает серверный `download_url` через `apiPath`, за пределы origin уйти не может.
- `src={…}` изображений/видео — всегда `URL.createObjectURL` от байтов, полученных `apiFetch(origin, '/blobs/<id>')` (`lib/media/objectUrl.ts:6-25`), или от локального файла (`compose`, `AvatarCrop`); origin берётся из контекста круга/сессии, не из данных записи. Подсунуть произвольный origin через данные записи нельзя.
- `style=`/`style:background` с цветами: все цвета проходят через таблицу `CIRCLE_COLORS` (`lib/theme/colors.ts`) с проверкой `in CIRCLE_COLORS` (`lib/circles/circles.ts:127`, `routes/circles/[id]/+layout.svelte:125-126,150-151,190`, `routes/admin/+page.svelte:85`, `routes/admin/people/[id]/+page.svelte:51`); CSS-инъекция через `color` невозможна. `SearchResultRow.svelte:39,57` `background-image:url({thumbUrl})` — blob: URL.
- Leaflet `divIcon({ html })` (`routes/circles/[id]/map/+page.svelte:103-115`) — скрытый `innerHTML`-сток, но подставляется только blob: URL из `getMediaUrl`; `bindPopup`/`bindTooltip` с текстом пользователя не используются.
- Админка: пароль админа и SMTP-пароль живут только в `$state` формы (`routes/admin/login/+page.svelte:13`, `routes/admin/general/+page.svelte:31,79` — обнуляется после сохранения, `routes/admin/bootstrap/+page.svelte:34`), в IDB — только токен (`admin_session`). Реквизиты/SMTP/проверки/сниппеты прокси рендерятся текстом (`lib/components/forms/RequisitesCard.svelte:19`, `lib/components/admin/CodeBlock.svelte:27-31`). `public_url` в клиенте — только поле ввода и `displayHost` (`routes/admin/general/+page.svelte:142,193`), в URL запросов не подставляется. Браузерные пробы (`lib/admin/external-probe.ts`) идут `fetch` без `Authorization` на origin админ-сессии (`lib/admin/admin.ts:234-247`). Dev-маршруты (`routes/dev/+layout.ts`) вне `vite dev` отвечают 404.
- Security-заголовки SPA: `web/serve.go:69-74` — `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, CSP `frame-ancestors 'none'; object-src 'none'; base-uri 'self'`; `index.html`/`sw.js`/`_app/*` — `Cache-Control: no-cache` (`:51-63`).
- **Токен и origin.** Сессии: `lib/idb/db.ts` — IndexedDB `wynd` (v2), store `sessions` по ключу `origin`, `admin_session` по ключу `'admin'`; токены не в `localStorage`. Logout участника (`session.svelte.ts:65-74`) зовёт `POST /auth/logout`, затем удаляет сессию, курсор и снимки origin; logout админа (`:143-150`) — `POST /admin/logout` + очистка (AUTH-3 закрыт). `reconcileStoredSessions`/`reconcileAdminSession` — при 401/403 токен сбрасывается; `sync.ts:177-180` — то же по ошибке потока.
- Origin инвайта: `routes/join/[token]`, `routes/invite/[token]` работают только с `origin=''` (текущий хост); через ссылку подсунуть чужой origin нельзя. `?next=`/`redirect` в клиенте нет — открытого редиректа после входа нет (`auth/code/+page.svelte:96` всегда `/circles`). `routes/circles/new/+layout.svelte:44-51`: `?origin=` из URL принимается только если совпадает с одной из сохранённых сессий. `lib/circles/origin.ts:32-61`: origin круга ищется по списку сессий, неизвестный круг → `/circles`.
- **Fetch-обёртка** `lib/api/client.ts`: токен участника берётся по `origin` из `sessions` — уходит только на выбранный origin (`apiPath` всегда строит `<origin>/api/v1/<path>`); админ-токен — для путей `/admin*`, все такие вызовы идут на `''`/`admin_session.origin` (`auth.ts:218`, `admin.ts:7-12`). `credentials` не задаётся (default `same-origin`, cookies не используются). Редирект `fetch`: по спецификации Fetch (Chrome/Firefox/Safari) `Authorization` снимается при смене origin — поведение платформы, не дефект клиента. 401/403 (кроме `payment_required`) → `isSessionRejected` → сброс сессии; 403 по кругу → инвалидация снимков круга (`client.ts:86-91`).
- **IDB/офлайн.** Снимки, медиа, пины, курсоры — ключи с префиксом `origin` (`db.ts:251-261`), два сервера на одном устройстве не смешиваются (CLI-1 закрыт: `invalidateSnapshots` отделяет вид снимка). Одна сессия на origin, поэтому разделение по учётке внутри origin отсутствует — см. находку про очередь.
- **Service worker**: `web/vite.config.ts:15-51` — `generateSW`, прекэш только собранных ассетов (`client/**/*.{js,css,…,woff2}`), `runtimeCaching: []` (ответы `/api/` и блобы с Bearer не кэшируются), `navigateFallbackDenylist: [/^\/api/]`, `registerType: 'autoUpdate'` + `registerSW({ immediate: true })` (`routes/+layout.svelte:26-28`); на loopback SW не регистрируется и старые снимаются (`app.html:7-16`). Своего `service-worker.ts` нет, `kit.serviceWorker.register: false`. Push доступен только в secure context (`push.ts:4-11`).
- **Карта**: плитки только с `tile.openstreetmap.org`, `Referer` урезан заголовком до origin (см. заметку выше); маркеры — blob: URL.
- **`web/scripts/ui-guard.mjs`** — правила консистентности UI (импорты компонентов, CSS кнопок); правила против `{@html}`/`innerHTML` **нет** (grep по `html` в `web/scripts/*` пустой).
