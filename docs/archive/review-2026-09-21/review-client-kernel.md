# Ревью клиентского ядра Wynd (`web/src/lib` без components/layouts/styles)

Статус: В РАБОТЕ (файл дописывается по мере чтения модулей).

## Ошибки и граничные случаи

### E1. high · CONFIRMED · `invalidateSnapshots(origin, { circleId })` без `kind` ничего не удаляет
- `web/src/lib/idb/db.ts:461-470`; вызовы: `web/src/lib/api/snapshots.ts:40-48` (`invalidateCircleSnapshots` без kind), `api/client.ts:89`, `sync/sync.ts:116`, `queue/queue.ts:214`, `journal/posts.ts:65,93,102,116,131,143,157,168`, `journal/days.ts:73,82,97,106`, `circles/settings.ts:228,242,364,411,424,437`.
- Ключ снимка `${origin}:${kind}:${id}`. Без `kind` префикс = `${origin}:`, значит `idPart` = `${kind}:${id}` (например `feed:<uuid>`), а сравнивается он с `circleId` (`idPart === circleId || idPart.startsWith(circleId + ':')`). Совпасть может только если kind равен circleId — никогда.
- Сценарий: участника исключили из круга → 403 → `client.ts:89` «чистит» снимки круга, но feed/grid/map/days/day остаются в IDB и дальше читаются офлайн (`loadFeed` отдаёт кэш при любом не-access сбое). То же после публикации из очереди, правки/удаления записи, SSE-события: кэш круга не инвалидируется, офлайн/при сбое сети показывается снимок со старым содержимым (удалённая запись «воскресает»).
- Тестов на `invalidateSnapshots` с `circleId` нет (в `session.test.ts:78` проверяется только вариант без опций).
- Исправление: в `invalidateSnapshots` при отсутствии `kind` разбирать ключ после `${origin}:` как `${kind}:${id}` — отрезать первый сегмент до `:` и сравнивать `circleId` с остатком (`rest === circleId || rest.startsWith(circleId + ':')`); добавить vitest на fake-indexeddb: положить `feed`, `day` (`<id>:<date>`), `circles/_list` и чужой круг, вызвать без kind, проверить, что ушли ровно feed и day нужного круга.

### E2. medium · CONFIRMED · Коллизия префиксов origin при инвалидации
- `web/src/lib/idb/db.ts:461,466`: `key.startsWith(`${origin}:`)`. Для `origin = 'https://a.example'` под префикс попадают и ключи `https://a.example:8443:…` (другой сервер на том же хосте — ровно случай «loopback с другим портом — другой сервер» из справочника).
- Сценарий: выход с `http://localhost` (или SSE-событие с него) стирает снимки `http://localhost:7676`. Только потеря кэша, но офлайн у второго сервера пропадает лента.
- Исправление: хранить ключ снимка как массив `[origin, kind, id]` (IDB поддерживает составные ключи) при следующем подъёме версии БД; до этого — после совпадения префикса проверять, что следующий сегмент входит в список `SnapshotKind`.

### E3. medium · CONFIRMED · `getDb()` кэширует отклонённый промис и не обрабатывает `blocked`/`blocking`/`terminated`
- `web/src/lib/idb/db.ts:189-210`.
- (а) Если `openDB` отклонён (приватный режим Firefox/старый Safari, `QuotaExceededError`, сбой диска), `dbPromise` навсегда остаётся rejected: любой вызов ядра (включая `initSession` → `getAppSettings`, `session.svelte.ts:108`) падает до перезагрузки вкладки, повтора нет.
- (б) Нет `blocking()` → при выпуске v3 старая вкладка держит соединение, новая вкладка висит на `openDB` бесконечно (нет и `blocked()` с подсказкой «закройте другие вкладки»). Нет `terminated()` → после принудительного закрытия БД браузером (iOS при нехватке места) все операции кидают `InvalidStateError` до перезагрузки.
- Исправление: в `getDb` передать `blocking() { db.close(); dbPromise = undefined; }`, `terminated() { dbPromise = undefined; }`, а `.catch` на `openDB` сбрасывает `dbPromise = undefined` и пробрасывает ошибку.

### E4. medium · CONFIRMED · `mediaStoreBytes()` поднимает весь кэш медиа в память
- `web/src/lib/idb/db.ts:440-448`: `db.getAll('media')` — все ArrayBuffer разом, чтобы сложить `byteLength`. Кэш не ограничен (вытеснения нет, см. E5), поэтому на экране 7.3 «кэш» при нескольких ГБ роликов вкладка на телефоне падает по памяти.
- Исправление: считать курсором (`openCursor` + `continue`), по одной записи за шаг.

### E6. low · CONFIRMED · `listQueueItems()` читает в память все файлы очереди при каждой отрисовке оверлея
- `web/src/lib/idb/db.ts:355-365`, `367-373`: N+1 `get` и полные `QueueFile.data` (ролики целиком) ради `body`/`file_count`. Вызывается на каждое `notify()` очереди и на каждый заход в ленту.
- Исправление: вынести `files[].data` в отдельный store `queue_files` (ключ `[queueId, index]`) при следующей версии БД; список очереди читает только метаданные.

## Безопасность клиента

(дополняется)

## Типы API и дрейф

(дополняется)

## Архитектура, дубли, заплатки

(дополняется)

## Legacy, заглушки, мёртвый код

(дополняется)

## Тесты

(дополняется)

## Комментарии и читаемость

(дополняется)

## Расхождения с документацией

(дополняется)

## Сделано хорошо

- SSE идёт через `fetch`-поток с `Authorization: Bearer` (`sync/sync.ts:132-135`), токен не попадает в URL и логи прокси.
- Файлы очереди хранятся как `ArrayBuffer`, а не `Blob`/`File` (`idb/db.ts:31-36`) — обходит известные проблемы Safari с Blob в IDB.
