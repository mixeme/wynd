# Ревью 2: internal/chronicle, internal/store, internal/uid

Статус: в работе (файл дописывается по мере чтения).

## Мёртвый код (таблица)

| Функция | Вызовы в production (не _test.go) | Вердикт |
|---|---|---|
| `TransferOwnership` (circle.go:187) | api/circle_settings.go:196 | used-untested |
| `DeleteCircle` (members.go:267) | api/circle_settings.go:244 | used-untested |
| `SetCircleName` (members.go:185) | api/circle_settings.go:61 | used-untested |
| `SetCircleColor` (members.go:232) | api/circle_settings.go:74 | used-untested |
| `ListMembers` (members.go:30) | api/circle_settings.go:17 | used-untested |
| `reconcileDayAfterEntryDateChange` (day.go) | content.go:185 | used-untested |
| `GridSnapshot` / `MapSnapshot` / `DayPostsSnapshot` (snapshot.go:195/246/341) | api/snapshots.go:61/86/149 | used-untested |
| `MediaVolumeChart` / `MedianPostBytes` / `FreedBytesBeforeCutoff` (volume.go:15/45/99) | api/archive.go:146/151/173 | used-untested |
| `SetInviteWho` (invite_settings.go:29) | api/circle_settings.go:80 | used-untested |
| `SetMemberCanSettings` (invite_settings.go:92) | api/circle_settings.go:145 | used-untested |
| `CountCirclePosts` (day.go:547) | api/archive.go:156 | used-untested |
| `ReactionIDForPost` (content.go:688) | api/journal.go:301 | used-untested |
| `CircleOwnerAccount` (circle.go:255) | api/archive.go:73 | used-untested |
| `CircleEditWindow` (circle.go:260) | api/archive.go:78 | used-untested |
| `summaryCircleRenamed` (summary.go) | members.go:223 | used-untested |
| `summaryOwnerTransferred` (summary.go) | circle.go:246 | used-untested |
| `CountPostsForDay` (day.go:538) | нет | **мёртвый** |
| `FindCommentID` (day.go:668) | нет | **мёртвый** (в комментарии прямо «test helper», но и тесты не зовут) |
| `FindReactionID` (day.go:683) | нет | **мёртвый** (то же) |
| `LastEventType` (day.go:732) | нет | **мёртвый** |
| `ServiceEventSummary` (day.go:722) | нет | **мёртвый** |
| `EstimateArchiveMediaBytes` (archive.go:456) | нет | **мёртвый** |

Итог: 6 экспортированных методов `*Chronicle` не вызываются ниоткуда, включая тесты. Три из них (`FindCommentID`, `FindReactionID`, `ServiceEventSummary`) написаны как тестовые хелперы, но живут в production-файле `day.go` и попадают в публичный API пакета. ОДНО исправление: перенести все шесть в `internal/chronicle/export_test.go` (или удалить `CountPostsForDay`/`EstimateArchiveMediaBytes`, у которых нет даже гипотетического потребителя) — API сервера не меняется, `day.go` худеет на ~70 строк.

## Ошибки и граничные случаи

(дополняется)

### DeleteCircle: одна `DELETE FROM circles` без транзакции, утечка блобов и риск FK-отказа — high, PLAUSIBLE
`members.go:267-284`. Функция делает ровно `DELETE FROM circles WHERE id = ?` на `c.db` и полагается на `ON DELETE CASCADE`.

Инвентарь таблиц с `circle_id` (0001_schema.sql) и судьба строк:

| Таблица | Строка | Судьба при `DELETE FROM circles` |
|---|---|---|
| `circle_notify_prefs` | 42 | CASCADE |
| `comments` | 68 | CASCADE |
| `days` | 122 | CASCADE |
| `events` | 135 | CASCADE |
| `identities` | 148 | CASCADE |
| `invites` | 178 | CASCADE (у серверных `circle_id IS NULL` — не трогаются, верно) |
| `memberships` | 199 | CASCADE |
| `pending_circle_joins` | 211 | CASCADE |
| `posts` | 245 | CASCADE |
| `quota_requests` | 270 | CASCADE |
| `reactions` | 281 | CASCADE |
| `read_cursors` | 296 | CASCADE |
| `day_titles` / `day_covers` | 104 / 118 | CASCADE через `days(circle_id, entry_date)` |
| `identity_names` | 156 | CASCADE через `identities` |
| `membership_spans` | 190 | CASCADE через `memberships` |
| `post_media` | 233 | CASCADE через `posts` |
| `content_fts` (FTS5, не external-content) | 80 | зависит от срабатывания триггеров `fts_post_delete` / `fts_comment_delete` / `fts_day_delete` на каскадном DELETE — **см. ниже** |
| `blob_refs` (`ref_type`,`ref_id`) | 22 | **ОСТАЁТСЯ**: `ref_id` — это id поста/медиа, FK на него нет, каскад не достаёт |
| `blobs` | 29 | **ОСТАЁТСЯ**: FK на `accounts`, не на круг. Строка живёт, байты учитываются в квоте аккаунта |
| файлы блобов на диске (`data/blobs`) | — | **ОСТАЮТСЯ**: `DeleteCircle` вообще не трогает blob-хранилище |

Три отдельных дефекта:
1. **Утечка байт.** Все медиа удалённого круга остаются и файлами, и строками `blobs`/`blob_refs`; ни один счётчик их не освободит (на круг ссылок больше нет — найти их уже нечем). Для домашнего инстанса с квотой в процентах диска это невосстановимая утечка.
2. **Порядок каскада и FK без действия.** `posts.event_seq INTEGER NOT NULL UNIQUE REFERENCES events(seq)` (245-246), `comments.event_seq` (70), `reactions.event_seq` (283), `day_titles/day_covers.event_seq` (95/110), `day_covers.post_id REFERENCES posts(id)` (97) — **без `ON DELETE`**, то есть NO ACTION. И `events`, и `posts` каскадятся от одного и того же родителя `circles`. Порядок обработки каскадов внутри одного оператора SQLite не документирует; если строки `events` удалятся раньше строк `posts`, немедленная проверка FK даст `FOREIGN KEY constraint failed` и удаление круга упадёт (а так как транзакции нет — упадёт целиком, но это хотя бы атомарно за счёт неявной транзакции оператора). Помечаю PLAUSIBLE, а не CONFIRMED: чтением исходника порядок не определить. Регрессионный тест: создать круг с постом, комментарием, реакцией, названием дня и обложкой дня, вызвать `DeleteCircle` и проверить, что ошибки нет и что `posts`/`events`/`day_covers` пусты.
3. **FTS.** Строки `content_fts` чистятся только триггерами `AFTER DELETE ON posts/comments/day_titles`. Срабатывают ли триггеры на строках, удалённых FK-действием `ON DELETE CASCADE`, — вопрос настройки `recursive_triggers` (по умолчанию выкл.), в `sqlite.go` этот pragma не выставляется (ставятся только `busy_timeout`, `foreign_keys`, `journal_mode`). Если не срабатывают — поиск по инстансу продолжит отдавать тексты удалённого круга. PLAUSIBLE; регрессионный тест: после `DeleteCircle` выполнить `SELECT count(*) FROM content_fts WHERE circle_id = ?` и ждать 0.

ОДНО исправление: переписать `DeleteCircle` как явную транзакцию, которая до `DELETE FROM circles` (а) собирает `blob_id` круга через `post_media JOIN posts` + аватары `identity_names`, (б) удаляет `blob_refs` этих ссылок и передаёт список блобов вызывающему для удаления файлов (как это уже делает архивный purge), (в) удаляет `content_fts WHERE circle_id = ?` явным `DELETE`, (г) удаляет дочерние таблицы в порядке `reactions, comments, day_covers, day_titles, posts, days, events, …` и только потом сам круг.

### TransferOwnership: проверки вне транзакции + старый владелец мог выйти — medium, CONFIRMED
`circle.go:187-252`. Проверки `circleOwner` (188) и `membership(newOwner)` (198) выполняются на `c.db` **до** `BeginTx` (206) — классический TOCTOU: между ними владелец может смениться или цель — выйти из круга. Дальше `oldMem` перечитывается уже внутри tx (212), а `newMem` — нет, и в событие пишется `targetID: newMem.IdentityID`, прочитанный снаружи.

Отдельно: у старого владельца статус не проверяется вовсе (`membership` без `Status`-проверки, 212). У цели проверка `StatusActive` (202) есть — это правильно и закрывает `left_with_access`. Обе строки `memberships` обновляются в одной tx (230, 234) — это сделано верно.

ОДНО исправление: перенести `circleOwner` и `membership(newOwner)` внутрь транзакции (сразу после `BeginTx`), выполнив их через `tx`, и там же проверить `newMem.Status == StatusActive`.

### migrate.go: проверка «БД новее бинаря» пропускает дыры в середине — medium, CONFIRMED
`store/migrate.go:36-51`. Защита построена на `MAX(version)`: если максимум не из известного списка — отказ с «удалите wynd.db». Но `applied` (53) — это множество, и цикл (58) применяет любую **не применённую** миграцию с версией меньше максимума. Сценарий: БД записана бинарём с миграциями 0001–0012, откатились на бинарь с 0001–0009 → `maxVersion = 12`, `known[12] == false` → отказ, это корректно. Обратный сценарий опаснее: если в `schema_migrations` по какой-то причине нет строки 0005, а есть 0009, бинарь молча выполнит 0005 **поверх** более новой схемы. Каждая миграция идёт в своей транзакции (`applyMigration`, 139-163) — это хорошо, но означает, что при падении на N-й миграции БД остаётся в промежуточном состоянии на версии N-1, а сообщение об ошибке про это не говорит.

ОДНО исправление: после вычисления `applied` отвергать запуск, если есть версия меньше `maxVersion`, отсутствующая в `applied` («в БД пропущена миграция %d») — вместо того чтобы её применять.

### migrate.go: `stripSQLComments` считает пустой не ту строку — low, CONFIRMED
`store/migrate.go:146-149`. Условие вычисляется по очищенному телу, а выполняется **исходный** `m.sql`. Для миграции, состоящей только из комментариев, `tx.Exec` не зовётся — верно; но если в файле есть только комментарии и, скажем, `PRAGMA`, обрезка тут ни на что не влияет: функция существует ради одного частного случая. Кроме того, `--` внутри строкового литерала SQL строка будет считаться комментарием и такая миграция будет ошибочно признана пустой, если в ней больше ничего нет.

ОДНО исправление: сравнивать не по `stripSQLComments`, а просто всегда выполнять `m.sql` (SQLite на пустом вводе не падает), удалив `stripSQLComments`.

### 0009: осознанные 100 ГБ администратора молча превращаются в 80% диска — medium, CONFIRMED
`migrations/0009_instance_quota_default_percent.sql`. `UPDATE … SET storage_quota_disk_percent = 80 WHERE storage_quota_bytes = 107374182400 AND storage_quota_disk_percent IS NULL`. Отличить «это заводское умолчание, которое никто не трогал» от «администратор сознательно выставил ровно 100 ГБ» по данным невозможно: колонки «когда меняли квоту» нет. На диске 500 ГБ администратор получит 400 ГБ вместо своих 100 — тихое четырёхкратное расширение потолка. Обратного хода у миграции нет.

ОДНО исправление: не менять саму миграцию (перенумерация закрыта решением), а добавить 0010, которая при наличии колонки-отметки о ручной правке возвращает абсолют; практичнее — считать это принятым риском и зафиксировать его строкой в `server-reference.md` рядом с описанием 0009, чтобы администратор знал, что после обновления потолок надо перепроверить.

## Транзакции и конкурентность

### Все транзакции — deferred, при read-then-write возможен SQLITE_BUSY вместо ожидания — medium
`BeginTx(ctx, nil)` во всём пакете (`circle.go:154,206`, `members.go:205`, …). Deferred-транзакция берёт read-lock на первом SELECT и пытается повыситься до write-lock на первом UPDATE/INSERT. Если между этим кто-то другой успел взять write-lock, SQLite возвращает `SQLITE_BUSY` **немедленно**, не дожидаясь `busy_timeout` (это документированное поведение: upgrade deadlock не разрешается ожиданием). `busy_timeout=5000` в `sqlite.go:18` от этого класса ошибок не спасает. Для домашнего инстанса на несколько человек вероятность мала, но при SSE-опросе раз в 2 с и фоновых джобах она не нулевая.

ОДНО исправление: завести в пакете хелпер `beginWrite(ctx)`, который делает `BEGIN IMMEDIATE` (для `modernc.org/sqlite` — `tx, _ := db.BeginTx(...)` + `tx.Exec("ROLLBACK; BEGIN IMMEDIATE")` неудобно; проще выполнить `db.ExecContext(ctx, "BEGIN IMMEDIATE")` на выделенном `*sql.Conn`), и звать его во всех пишущих путях хроники.

### Пять худших TOCTOU: проверка на `c.db`, запись в `tx`
| # | Место | Что читается вне tx | Чем плохо |
|---|---|---|---|
| 1 | `circle.go:188,198` `TransferOwnership` | владелец круга и статус цели | владелец мог смениться параллельным переводом; цель могла выйти между проверкой и `UPDATE` |
| 2 | `member.go:201` `forbidOwnerLeave` → `leave` (227) | владелец | между проверкой и tx владение могло перейти к уходящему, и круг останется без владельца |
| 3 | `member.go:213` `Exclude` | владелец | то же: бывший владелец успевает исключить участника уже после перевода прав |
| 4 | `members.go:196,200` `SetCircleName` | membership и имя актора | имя в событии может не совпасть с именем на момент коммита |
| 5 | `members.go:268,272` `DeleteCircle` | владелец и имя круга | круг могли переименовать между проверкой подтверждения имени и `DELETE` — удаление пройдёт по устаревшему подтверждению |
Плюс окно правок: см. ниже про `assertEditable`.

ОДНО исправление: перенести все эти чтения внутрь соответствующей транзакции (везде, где tx уже есть, — заменить `c.db` на `tx`; в `SetCircleColor`/`Exclude` завести tx).

### `DaysSnapshot` — N+1 без потолка — medium, CONFIRMED
`snapshot.go:287-339`. Внешний запрос по `days` идёт **без LIMIT** (потолок 2000 к нему не применён), и на каждый день выполняются ещё три запроса: `visiblePostCountForDay` (317), `dayTitleEditableUntil` (324), `dayCoverEditableUntil` (328). Для круга с тремя годами истории это `1 + 3×1100 ≈ 3300` запросов на один `GET /circles/{id}/days`, причём выполняются они внутри открытого курсора `rows` — пул `database/sql` вынужден держать вторую связь к SQLite.

Счёт запросов на N постов/дней:
- `FeedSnapshot` / `DayPostsSnapshot` — **5 запросов на любое N** (`visiblePostIDs` + `loadPostsByIDs` + `listMediaForPosts` + `listCommentsForPosts` + `listReactionsForPosts`, все через `IN (...)`). Сделано хорошо.
- `GridSnapshot` / `MapSnapshot` — 1 запрос. Хорошо.
- `DaysSnapshot` — **3N + 1**.
- `VisiblePostSeqs` (visibility.go:180) — **2N + 1**: на каждый пост зовётся `CanReadEvent`, а та каждый раз заново читает `memberships` и все `membership_spans`. Функция при этом не вызывается из production вообще (см. «Мёртвый код»).

ОДНО исправление: в `DaysSnapshot` заменить три запроса в цикле на три групповых (`GROUP BY entry_date` по `posts` с предикатом видимости; два `MAX(...) GROUP BY entry_date` по `day_titles`/`day_covers`), слить результаты в map по `entry_date` и добавить `LIMIT SnapshotPostLimit` на внешний запрос.

### Пост №2001 молча теряется — medium, CONFIRMED
`snapshot.go:179,208,260,350`. Везде `LIMIT SnapshotPostLimit` и ни одного признака усечения: ни поля `truncated` в `FeedPost`/ответе, ни лога. Решение «потолок 2000 без пагинации» закрыто, но «тихо» — это отдельная беда: круг, переваливший порог, начнёт терять самые старые записи в ленте, и никто об этом не узнает (в ленте сортировка `created_at DESC`, то есть пропадает хвост истории, а в `DayPostsSnapshot` — наоборот, `ASC`, пропадает конец дня).

ОДНО исправление: выбирать `LIMIT SnapshotPostLimit + 1`, и если вернулось больше потолка — обрезать и залогировать `slog.Warn` с `circle_id` и числом (это не пагинация и не меняет API).

### `DayPostsSnapshot`: сортировка через `datetime()` теряет доли секунды — low, CONFIRMED
`snapshot.go:350`: `ORDER BY datetime(COALESCE(captured_at, created_at)) ASC`. `datetime()` в SQLite усекает до секунд, поэтому все записи одной секунды получают равный ключ и порядок между ними не определён — устойчивого тай-брейкера (`, id`) нет. Заодно доктор-комментарий на 341 говорит «ordered by captured_at then created_at», а в коде `COALESCE` — это не «сначала одно, потом другое».

ОДНО исправление: `ORDER BY COALESCE(captured_at, created_at) ASC, id ASC` — без `datetime()` (строки уже сравнимы лексикографически после перехода на фиксированную ширину из плана 0010) и с устойчивым тай-брейкером.

### `Exclude` не работает против вышедшего с доступом — high, CONFIRMED
`member.go:212-224` → `leave` → `leaveInTx`, где `member.go:243-245`:
```go
if mem.Status != StatusActive {
    return ErrInvalid
}
```
Участник со статусом `left_with_access` сохраняет `can_read = 1` на закрытом отрезке, то есть продолжает видеть прошлое. Владелец не может отобрать у него этот доступ: `Exclude` упрётся в `ErrInvalid`, потому что статус уже не `active`. Единственный способ — удалить весь круг. Это дыра ровно в том сценарии, ради которого исключение и существует (поссорились после ухода).

ОДНО исправление: в `leaveInTx` разрешить переход `left_with_access → gone`, а для этого случая вместо `closeOpenSpan` выполнять `UPDATE membership_spans SET can_read = 0 WHERE membership_id = ?` (открытого отрезка там уже нет, поэтому текущий `closeOpenSpan` вернёт `ErrInvalid` на `RowsAffected() == 0`, member.go:297).

### Повторный вход: дыры между отрезками нет — проверено
`rejoinTx` (member.go:127-177) заводит **новый** отрезок с `started_at = now` и не трогает закрытый. Инвариант «повторный вход не открывает дыру» выполняется. Замечание low: `rejoinTx` не проверяет, что у членства нет уже открытого отрезка; если строка `memberships` каким-то образом оказалась `gone` при живом отрезке, появится два открытых отрезка и `closeOpenSpan` закроет оба одним `UPDATE` — рассинхрон не диагностируется. Исправление: перед вставкой отрезка требовать `SELECT count(*) … ended_at IS NULL` = 0, иначе `ErrInvalid`.

### Одинокий круг и владелец
`forbidOwnerLeave` (member.go:200-209) запрещает владельцу и `Leave`, и `LeaveWithAccess`; `Exclude` (220) запрещает исключать владельца. Значит единственный участник круга не может выйти — только `DeleteCircle` или `TransferOwnership`. Это последовательно, но у `TransferOwnership` в одиночном круге нет цели, а `DeleteCircle` не чистит блобы (см. выше). Не баг, но единственный выход из одиночного круга ведёт через самую дырявую функцию пакета.

### Мягко удалённый аккаунт
`LeaveInTx` (member.go:196) ставит `gone` и закрывает отрезок в чужой транзакции — как и требует решение про soft-delete в одной tx. Но `memberships.account_id` при этом **не** обнуляется (обнуляется только `identities.account_id`), так что строка `memberships` удалённого аккаунта продолжает занимать `UNIQUE (circle_id, account_id)`. Регистрация той же почты даёт новый `account.id`, поэтому конфликта нет — но `ListMembers` и любые выборки по `memberships` продолжат видеть «мёртвую» строку с рабочим `account_id`, указывающим на заблокированный аккаунт. Это надо иметь в виду при отображении списка участников.
