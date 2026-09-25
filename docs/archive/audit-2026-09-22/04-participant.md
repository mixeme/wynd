# 04 — Участнические маршруты круга и права внутри хроники (SEC-6)

Аудит безопасности Wynd 0.7.0, 2026-09-22. Область: маршруты под `paid(...)` из `internal/api/server.go:124-191`, кроме `/uploads*`, `/blobs/*`, `/archive/*`, `/pay/*`, `/push/*`.

## Итог

- Средняя: 3 (№1 идемпотентность по `client_id` без проверки автора + дозапись медиа; №2 стирание заголовка/обложки дня без `requireWriter`; №3 поиск отдаёт вышедшему комментарии после ухода).
- Низкая: 2 (№4 push-сигналы вышедшему; №5 правки после ухода видны, деталь круга для `gone`).
- Заметки: 6 (№6–№11).
- Критических и высоких не найдено. SEC-3 (`requireWriter`/`requireAuthor` в мутаторах) в текущем коде выполнен, кроме `ClearDayTitle`/`ClearDayCover`.

## Таблица маршрутов

| маршрут | обработчик | проверка прав в домене | замечание |
|---|---|---|---|
| `POST /circles/{circle_id}/posts` | `handleCreatePost` (`journal.go:57`) | `CreatePost/createPostInTx` → `requireWriter` (членство + `CanWrite`: статус active и открытый отрезок с can_write). Блобы: `Blobs.ValidateOwnedComplete(sess.AccountID, ids)` в обработчике. | Идемпотентность по `(circle_id, client_id)` без проверки автора: при совпадении возвращается чужая запись и к ней **дописываются медиа** (`createPostWithMedia` → `AttachMediaInTx`). См. находку №1. `client_id` без ограничения длины. |
| `PATCH /circles/{circle_id}/posts/{post_id}` | `handleEditPost` (`journal.go:124`) | `EditPost/EditPostInTx`: `post.CircleID == circleID` (иначе 404), `requireAuthor` (identity автора == identity актора, `CanWrite`), окно `post.EditWindow.CanEdit`. `SetPostCover`: то же + `BlobOnPost`. Медиа: новые блобы через `ValidateOwnedComplete`, `ReplacePostMediaInTx`. | Проверка «блоб ещё не занят» — зона блобов (другой агент). Нет `assertPostInteractive` (архивный срез) — только окно редактирования. |
| `DELETE /circles/{circle_id}/posts/{post_id}` | `handleDeletePost` (`journal.go:175`) | `DeletePost`: круг, `requireAuthor` (в tx), окно. | Нет архивной проверки, как и в PATCH. |
| `POST .../posts/{post_id}/comments` | `handleCreateComment` (`journal.go:197`) | `CreateComment`: `requireWriter`, `post.CircleID == circleID && !Deleted`, `assertPostInteractive` (отрезок читателя + архивный срез). | Идемпотентность `commentByClientID` без проверки автора и `post_id` (находка №1). |
| `PATCH .../comments/{comment_id}` | `handleEditComment` (`journal.go:226`) | `EditComment`: `comment.CircleID == circleID`, `requireAuthor`, окно. | `post_id` из пути **не сверяется** с `comment.PostID` — безвредно, т.к. права по автору и кругу. |
| `DELETE .../comments/{comment_id}` | `handleDeleteComment` (`journal.go:247`) | `DeleteComment`: то же. | То же. |
| `PUT .../posts/{post_id}/reactions` | `handleSetReaction` (`journal.go:263`) | `SetReaction`: `validReactionKey` (4 ключа), `requireWriter`, круг записи, `assertPostInteractive`. Одна реакция на identity (upsert). | В порядке. |
| `DELETE .../posts/{post_id}/reactions` | `handleDeleteReaction` (`journal.go:288`) | `ReactionIDForPost(circle, post, mem.IdentityID)` → `DeleteReaction`: круг, `requireAuthor`, окно, `assertPostInteractive`. | В порядке. |
| `PUT /circles/{circle_id}/days/{date}/title` | `handleSetDayTitle` (`journal.go:314`) | `SetDayTitle`: `hasPostForDay` (у актора есть запись за день), `requireWriter`. | Любой писатель с записью за день перезаписывает чужой заголовок — «last-write-wins», по замыслу. |
| `DELETE .../days/{date}/title` | `handleClearDayTitle` (`journal.go:338`) | `ClearDayTitle` → `canClearDaySaid`: `hasPostForDay` (только `membership` — **без проверки статуса**) + окно текущего заголовка. **Нет `requireWriter`.** | Находка №2: исключённый / вышедший может стирать заголовки и обложки дня. |
| `PUT .../days/{date}/cover` | `handleSetDayCover` (`journal.go:354`) | `SetDayCover`: `hasPostForDay`, `loadPost` (круг, дата, не удалена), `BlobOnPost(post, blob)`, `requireWriter`. | `blob_id` из другого круга невозможен — блоб должен быть на записи этого круга. В порядке. |
| `DELETE .../days/{date}/cover` | `handleClearDayCover` (`journal.go:378`) | `ClearDayCover` → `canClearDaySaid` — как у заголовка. **Нет `requireWriter`.** | Находка №2. |
| `GET /circles` | `handleListCircles` (`circles.go:11`) | `ListAccountCircles`: только свои членства; unread/last_summary через `sqlVisibleAtMembership` (отрезок). | В порядке. Статус `gone` тоже в списке (без содержимого). |
| `POST /circles` | `handleCreateCircle` (`circles.go:51`) | `CreateCircle`: длины имён. | `edit_window_sec` **отрицательный не отклоняется** (в `PatchCircle`/`SetEditWindow` — отклоняется); см. `edit_window_sql.go` ниже. |
| `POST /circles/{circle_id}/invites` | `handleCreateCircleInvite` (`circles.go:89`) | `RequireCanInvite`: при `invite_who=owner` — `RequireOwner`, иначе активное членство. | `max_uses` и `ttl_sec` **без верхнего предела** (заметка №7). |
| `GET /circles/{circle_id}/invites`, `DELETE .../invites/{id}` | `circles.go:144,163` | `RequireCanInvite`; `Auth.RevokeCircleInvite(circleID, inviteID)` — по паре круг+id. | В порядке (id из другого круга не отзывается — проверить в auth другой агент). |
| `POST /circles/{circle_id}/leave` | `handleLeaveCircle` (`circles.go:186`) | `Leave/LeaveWithAccess` → `forbidOwnerLeaveTx` в tx; `leaveInTx` → `ErrInvalid`, если статус не active. | В порядке. |
| `PUT /circles/{circle_id}/read_cursor` | `handleSetReadCursor` (`circles.go:218`) | `SetReadCursor`: `seq >= 0`, `membership()` (любой статус). Ключ — свой `account_id`. | В порядке: только своя строка. |
| `GET /circles/{circle_id}/members` | `handleCircleMembers` (`circle_settings.go:11`) | `ListMembers` → `requireReader` (не `gone`, есть отрезок с can_read). | Отдаёт **`account_id` всех участников** каждому читателю, включая `left_with_access` (заметка №6). E-mail нет. |
| `PATCH /circles/{circle_id}` | `handlePatchCircle` (`circle_settings.go:48`) | `PatchCircle` → `requireSettingsTx` (active + can_settings, либо владелец). Значения проверяются до записи. | Не «только владелец»: участник с `can_settings` меняет окно редактирования, `invite_who`, `invite_kind_default`. Соответствует замыслу флага. |
| `PUT /circles/{circle_id}/identity` | `handleUpdateIdentity` (`circle_settings.go:83`) | Аватар: `ValidateOwnedComplete(sess.AccountID, blob)` в обработчике; `UpdateIdentity`: статус active; `RenameIdentity`: длина. | Чужой блоб — 403. Ref на аватарный блоб не ставится (зона блобов). |
| `PUT /circles/{circle_id}/members/{account_id}` | `handleSetMember` (`circle_settings.go:116`) | `SetMemberCanSettings`: `RequireOwner`, цель ≠ владелец, цель active. Только флаг `can_settings`. | В порядке. |
| `GET /circles/{circle_id}/identity` | `handleIdentityHistory` (`circle_settings.go:137`) | Только собственная identity (`MembershipForAccount`). | В порядке. |
| `POST /circles/{circle_id}/transfer` | `handleTransferOwnership` (`circle_settings.go:168`) | `TransferOwnership` в tx: актор — владелец, новый ≠ старый, новый — active член. | Несуществующему/ушедшему — 404/403. Заблокированный аккаунт (auth) не проверяется — но он не пройдёт шлюз для своих запросов; владелец-«блокированный» — редкий случай, заметка. |
| `POST /circles/{circle_id}/exclude` | `handleExcludeMember` (`circle_settings.go:192`) | `Exclude`: актор — владелец, цель ≠ владелец (в tx). | Владелец себя исключить не может. В порядке. |
| `DELETE /circles/{circle_id}` | `handleDeleteCircle` (`circle_settings.go:216`) | `DeleteCircle` в tx: владелец + совпадение имени. | В порядке. |
| `GET /circles/{circle_id}/feed` | `handleFeed` (`snapshots.go:10`) | `FeedSnapshot`: `requireReader` (не `gone`, есть can_read-отрезок); записи — `sqlVisibleAt(created_at)` + `readScope.canRead`; комментарии/реакции — по своему `created_at`; служебные события — `CanReadEvent`. | Тело записи — **текущее**: правки после ухода `left_with_access` видны (заметка №5). Медиа не фильтруются по времени (замена вложений после ухода видна). |
| `GET /circles/{circle_id}/grid` | `handleGrid` (`snapshots.go:54`) | `GridSnapshot`: `requireReader` + `sqlVisibleAt(p.created_at)`. | В порядке. |
| `GET /circles/{circle_id}/map` | `handleMap` (`snapshots.go:79`) | `MapSnapshot`: то же. | В порядке. |
| `GET /circles/{circle_id}/days` | `handleDays` (`snapshots.go:107`) | `DaysSnapshot`: `requireReader`; день попадает, если `visiblePostCountForDay > 0`. | `title`/`cover_post_id`/`cover_blob_id` дня отдаются целиком, даже если обложка — из записи вне отрезка читателя (утечка `blob_id`; отдаст ли файл — зона блобов). Заметка №5. |
| `GET /circles/{circle_id}/days/{date}` | `handleDayDetail` (`snapshots.go:141`) | `DayPostsSnapshot`: как feed. | В порядке. |
| `GET /circles/{circle_id}/search`, `.../search/authors` | `handleCircleSearch`, `handleCircleSearchAuthors` (`search.go:12,28`) | `RequireReader` + `visibleCarrierSQL` (носитель: запись по `p.created_at`, день по `entry_date`). FTS: слова в `"…"` с удвоением кавычек, ≤256 байт, ≤8 слов; `limit` зажат в 1..50. | **Комментарий виден по дате записи-носителя, а не по своей** — вышедший с доступом находит поиском комментарии, написанные после его ухода (находка №3). |
| `GET /search` | `handleGlobalSearch` (`search.go:45`) | `SearchAll`: тот же `visibleCarrierSQL` с `m.account_id = ?` по всем кругам — только круги с can_read-отрезком. | В порядке по кругам; та же находка №3. |
| `GET /sync` (JSON / SSE / NDJSON) | `handleSync` (`sync.go:15`) | `SyncEvents` (`chronicle/sync.go:27`): `EXISTS` по `memberships`+`membership_spans` для `e.circle_id` и `e.created_at` — отрезок читателя. `cursor` — глобальный `seq`, но фильтр по кругу/отрезку применяется к каждому событию; чужой круг подстановкой cursor не получить. SSE перечитывает сессию и оплату каждый цикл. | `max_seq` — глобальный по инстансу (заметка №8). В порядке. |
| `GET/PUT /notify_prefs`, `GET/PUT /circles/{circle_id}/notify_prefs` | `notify.go:50-131` | Только свой `account_id`. Членство в круге **не проверяется** (`CircleNotifyPrefs`/`SaveCircleNotifyPrefs` — по паре account+circle). | Мусорные строки для произвольных `circle_id`, `mute_until` — произвольная строка (заметка №9). |
| `POST /circles/{circle_id}/quota_requests` | `handleCreateQuotaRequest` (`notify.go:332`) | `RequireOwner`. | Значение `requested_bytes` — зона блобов. |
| `GET /circles/{circle_id}/invite-candidates`, `POST .../member-invites` | `member_invites.go:12,37` | `ListInviteCandidates` → `RequireCanInvite`; `CanInviteAccount`: цель не active в целевом круге и состоит с приглашающим в другом круге (любой статус, включая `gone`). | Отдаёт `account_id` и имена участников **других** кругов приглашающего (по замыслу функции); заметка №6. |
| `GET /circles/{circle_id}/join-preview`, `POST .../join`, `GET /pending-circle-joins` | `member_invites.go:89-181` | По наличию `pending join` для своей учётки (auth). `InvitePeekForCircle` — имена активных участников. | В порядке (сами приглашения — у другого агента). |
| `GET /circles/{circle_id}` | `handleCircleDetail` (`archive.go:40`) | Наличие членства (любой статус, из `ListAccountCircles`). Отдаёт: имя, цвет, окно, `invite_who`, своя identity, `is_owner`. | `gone` видит текущее имя/цвет/настройки круга после исключения (заметка №5). `account_id` владельца не отдаётся. |
| `GET /circles/{circle_id}/quota` | `handleCircleQuota` (`archive.go:125`) | `RequireOwner`. | В порядке. |
| `POST /circles/{circle_id}/notify_prefs` mention-сигналы | `notifyAccounts`/`notifyCircle`/`notifyComment` (`notify.go:133-273`) | Push-payload: только `circle_id`, `type`, `count` — **текста нет**. `MentionedAccountIDs` — только active. Получатели `notifyCircle`: `status != 'gone'` → **включая `left_with_access`**. | Заметка №4 (сигналы вышедшему), заметка №10 (упоминание обходит mute). |

## Находки

### 1. Идемпотентный повтор по `client_id` возвращает чужую запись и дописывает к ней медиа
- Серьёзность: средняя
- Уверенность: подтверждено кодом (проследил вызовы до конца)
- Где: `internal/chronicle/content.go:62-70` (`createPostInTx`), `content.go:839-852` (`postByClientID`), `content.go:451-459` (`CreateComment`), `internal/api/journal.go:445-471` (`createPostWithMedia`), `internal/chronicle/media.go:54-97` (`attachMedia`), `internal/blob/serve.go:118-127` (`AddRef` — `INSERT OR IGNORE`).
- Атакующий: участник того же круга (B) против другого участника (A); также сам клиент при честном повторе.
- Что: `postByClientID`/`commentByClientID` ищут по `(circle_id, client_id)` и не сверяют `identity_id` найденной сущности с актором. Если B пришлёт `POST /circles/{c}/posts` с `client_id` записи A, `CreatePostInTx` вернёт запись A (201 с телом A, в т.ч. если запись создана до отрезка видимости B), а затем `createPostWithMedia` вызовет `AttachMediaInTx(post.ID, media)` — блобы B (проверенные `ValidateOwnedComplete` как *его* блобы) окажутся вложениями записи A; `AddRef` с `INSERT OR IGNORE` не мешает. Далее уходит `notifyCircle` и упоминания из тела B. Тот же путь при **честном** повторе очереди с медиа: вложения дублируются на своей же записи (post_media без уникального ключа). Для комментария: `commentByClientID` не сверяет ни автора, ни `post_id` из пути — возвращается чужой комментарий с другой записи. `client_id` — произвольная строка без ограничения длины (`journal.go:110,212`): предугадываемые ключи («1», «post-1») в стороннем клиенте делают атаку реалистичной; `client_id` никуда не отдаётся, поэтому официальные UUID перебором не угадать.
- Как воспроизвести: A создаёт запись с `{"body":"a","entry_date":"2026-09-01","client_id":"k1"}`. B в том же круге: `POST /api/v1/circles/{c}/posts` `{"body":"x","entry_date":"2026-09-01","client_id":"k1","media":[{"blob_id":"<блоб B>","kind":"photo"}]}` → 201 с записью A, `GET feed` показывает фото B на записи A. Повтор: A шлёт свой запрос с медиа дважды → два одинаковых вложения.
- Исправление: в `postByClientID`/`commentByClientID` (или сразу после них) проверять `existing.IdentityID == mem.IdentityID` (иначе `ErrInvalid`/`ErrForbidden`) и для комментария `existing.PostID == in.PostID`; в `createPostWithMedia` при возврате уже существующей записи **не** вызывать `AttachMediaInTx`/`AddRef` (признак «создана сейчас» из `createPostInTx`); ограничить `client_id` (например, `MaxNameChars`/64 байта).

### 2. Стирание заголовка и обложки дня доступно исключённому и вышедшему участнику
- Серьёзность: средняя
- Уверенность: подтверждено кодом
- Где: `internal/chronicle/day.go:437-455` (`canClearDaySaid`), `day.go:274-285` (`hasPostForDay` — только `membership()`, без статуса и отрезка), `day.go:458-501` (`ClearDayTitle`, `ClearDayCover`); для сравнения `SetDayTitle`/`SetDayCover` (`day.go:303`, `day.go:382`) вызывают `requireWriter`.
- Атакующий: исключённый (`gone`), вышедший (`left_with_access`/`gone`) участник, у которого осталась хотя бы одна запись за этот день.
- Что: `DELETE /circles/{c}/days/{date}/title|cover` → `canClearDaySaid` проверяет лишь наличие *записи актора за этот день* и окно редактирования текущего заголовка/обложки; `requireWriter`/`CanWrite` не вызываются. Строка `memberships` при исключении не удаляется, записи остаются с `identity_id` автора, поэтому бывший участник стирает заголовки/обложки дней (в том числе чужие — они общие). На уровне шлюза `RequirePaidParticipant` проверяет только учётку. Действие также не оставляет следа: `removeSaidHistory` удаляет событие.
- Как воспроизвести: участник B пишет запись за 2026-09-01; владелец ставит заголовок дня и исключает B (`POST /circles/{c}/exclude`). B: `DELETE /api/v1/circles/{c}/days/2026-09-01/title` → 200, заголовок исчез (в пределах окна редактирования круга; при безлимитном окне — всегда).
- Исправление: в `canClearDaySaid` добавить `c.requireWriter(ctx, circleID, accountID, now)` (или `CanWrite`) перед проверкой окна; заодно `hasPostForDay` сделать зависимым от статуса `active`.

### 3. Поиск показывает вышедшему комментарии, написанные после его ухода
- Серьёзность: средняя
- Уверенность: подтверждено кодом
- Где: `internal/search/search.go:20-34` (`visibleCarrierSQL`: для `kind != 'day'` проверяется `p.created_at` записи-носителя), `search.go:152-165` (`snippet(content_fts, 0, '', '', '…', 32)` — фактически весь текст короткого комментария); для сравнения лента фильтрует комментарии по собственному `created_at` — `internal/chronicle/snapshot_feed.go:195-207`.
- Атакующий: вышедший с доступом (`left_with_access`); также участник, чей отрезок закрыт по любой причине, но can_read сохранён.
- Что: SRCH-1 привязал видимость попадания к записи-носителю, но для комментария потерялось второе условие — собственный `created_at` комментария должен попадать в отрезок читателя. Итог: после ухода участник через `GET /circles/{c}/search?q=слово` (или `GET /search`) получает сниппеты и `author_name` комментариев, оставленных другими после его ухода, к любой записи, которую он мог видеть. В ленте те же комментарии скрыты.
- Как воспроизвести: A и B в круге; A публикует запись; B выходит с сохранением доступа; A пишет комментарий «секретное слово». B: `GET /api/v1/circles/{c}/search?q=секретное` → hit с `kind=comment`, `snippet` содержит текст. `GET feed` этот комментарий не показывает.
- Исправление: в `visibleCarrierSQL` для `f.kind = 'comment'` добавить условие `f.created_at >= ms.started_at AND (ms.ended_at IS NULL OR f.created_at < ms.ended_at)` (при сохранении условия по носителю).

### 4. Вышедший с доступом получает push-сигналы о новой активности круга
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/chronicle/archive.go:535-554` (`CircleMemberAccountIDs`: `status != 'gone'`), `internal/api/notify.go:194-273` (`notifyCircle`, `notifyComment`).
- Атакующий: вышедший с доступом (`left_with_access`) как пассивный наблюдатель.
- Что: после ухода участник продолжает получать push «новая запись/комментарий/реакция в круге X» (без текста), хотя само содержимое ему уже не видно. Это метаданные активности круга, которых он видеть не должен; при `posts=true` — сигнал на каждую запись.
- Как воспроизвести: B выходит с сохранением доступа, подписка push жива; A публикует запись → B получает сигнал `type=post` для круга.
- Исправление: в `CircleMemberAccountIDs` (или в рассылке) брать только `status = 'active'`.

### 5. Содержимое, изменённое после ухода, остаётся видимым вышедшему
- Серьёзность: низкая
- Уверенность: подтверждено кодом
- Где: `internal/chronicle/snapshot_feed.go:48-85` (`buildFeedPosts` — тело записи текущее, медиа без фильтра по времени), `snapshot.go:286-339` (`DaysSnapshot` — `title`/`cover_blob_id` дня целиком), `internal/api/archive.go:40-123` (`handleCircleDetail` — любое членство, включая `gone`).
- Атакующий: вышедший с доступом; для детали круга — исключённый.
- Что: отрезок видимости применяется к `created_at` записи/комментария/события, но не к последующим правкам: правка тела записи, замена вложений (`ReplacePostMediaInTx`), новый заголовок или обложка дня после ухода видны `left_with_access` через `feed`/`days`/`days/{date}`. `cover_blob_id` дня может указывать на запись вне отрезка читателя (отдаст ли файл `/blobs/{id}` — зона блобов). `GET /circles` и `GET /circles/{id}` отдают `gone`-участнику текущее имя, цвет, окно и invite-настройки круга после исключения. Sync это делает правильно (`post.edited` после ухода не отдаётся), т.е. модель неконсистентна между sync и снимками.
- Как воспроизвести: B выходит с сохранением доступа; A правит старую запись (`PATCH`) → `GET feed` у B показывает новый текст; A ставит заголовок старому дню → у B он появляется в `GET days`.
- Исправление: либо признать («снимок отдаёт текущее состояние видимых записей») и записать в справочник, либо для `left_with_access` замораживать тело по журналу событий (последний `post.edited` в отрезке) — дорого; минимум — `handleCircleDetail`/`ListAccountCircles` для `gone` отдавать только `id`+`status`.

### 6. `account_id` других участников виден каждому читателю
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/api/circle_settings.go:23-36` (`members`), `internal/chronicle/invite_candidates.go:27-85` (`invite-candidates`: участники **других** кругов с `status`, в т.ч. `gone`).
- Что: модель «identity на круг» размывается: глобальный `account_id` отдаётся всем читателям круга (включая `left_with_access`), а список кандидатов раскрывает приглашающему статусы участников чужих для целевого круга сообществ (кто откуда исключён). E-mail никуда не отдаётся (проверено: `MemberRow`, `InviteCandidate`, `InvitePeek`, `feedPostResponse`, `commentResponse`, `eventResponse`). `account_id` нужен владельцу для `members/{account_id}`, `exclude`, `transfer` — можно отдавать его только владельцу/`can_settings`, остальным — `identity_id`.

### 7. Приглашения круга: `max_uses` и `ttl_sec` без потолка
- Серьёзность: заметка
- Уверенность: подтверждено кодом
- Где: `internal/api/circles.go:120-127`.
- Что: любой участник с правом приглашать создаёт ссылку на неограниченное число входов и на годы вперёд (`ttl_sec` до переполнения `time.Duration` ≈ 292 года). Лимиты самого auth-слоя — у другого агента.
- Исправление: зажать `max_uses` (например ≤ 100) и `ttl_sec` (≤ 30 сут.).

### 8. `max_seq` в `/sync` — глобальный счётчик инстанса
- Серьёзность: заметка
- Где: `internal/api/sync.go:41-49`, `internal/chronicle/sync.go:14-24`.
- Что: любой участник по `max_seq` наблюдает общую интенсивность событий во всех кругах инстанса (сколько событий за период). Событий не видит, только счётчик. Для однопользовательских/семейных инстансов несущественно.

### 9. Настройки уведомлений: мусорные строки и невалидный `mute_until`
- Серьёзность: заметка
- Где: `internal/api/notify.go:38-45,103-131`, `internal/auth/notify.go:173-203,239-248`.
- Что: `PUT /circles/{любой id}/notify_prefs` создаёт строку `circle_notify_prefs` без проверки членства — участник может насыпать неограниченно строк (дёшево, по одной на запрос). `mute_until` хранится как есть (до ~1 МБ по `limitBody`), при нераспознанном формате трактуется как «не заглушено». Исправление: `RequireReader` перед сохранением; `mute_until` парсить RFC3339 и хранить нормализованным.

### 10. Упоминание обходит mute получателя
- Серьёзность: заметка (продуктовое решение)
- Где: `internal/auth/notify.go:206-209`, `internal/api/notify.go:46`.
- Что: `mentions` принудительно `true`, `NotifyPrefAllows("mention")` возвращает `true` до проверки `NotifyMuted`. Участник может «пробивать» заглушённого коллегу упоминанием `@Имя` в каждом комментарии; текст в push не уходит, только сигнал. Если так задумано — зафиксировать в справочнике.

### 11. Мелочи ввода
- Серьёзность: заметка
- `POST /circles`: отрицательный `edit_window_sec` не отклоняется (`circle.go:11-60`; `PatchCircle`/`SetEditWindow` отклоняют) — эффект: записи сразу нередактируемы, вред только себе. Очень большое значение (> 9.2e9 с) переполняет `time.Duration` в `EditableUntil` (`types.go:53`) — тот же эффект.
- `client_id` без ограничения длины (`journal.go:110,212`) — до ~1 МБ в колонке `posts.client_id`.
- Число медиа на запись не ограничено (`parseMediaInput`, `journal.go:394`) — сотни `post_media` на одну запись в пределах 1 МБ тела; каждый блоб проверяется отдельным `LoadBlob` (`ValidateOwnedComplete`) — N запросов к БД на один POST. Лимит вроде 50 вложений разумен.
- `SetDayTitle` не делает `TrimSpace` (`day.go:289`) — заголовок из пробелов проходит.

## Проверено, в порядке

- `requireWriter`/`requireAuthor` (`content.go:375-409`): `CanWrite` требует `status = active` и открытый отрезок с `can_write`; `requireAuthor` дополнительно сверяет `identity_id`. Есть во всех мутаторах содержимого: `CreatePost`, `EditPost(InTx)`, `DeletePost`, `CreateComment`, `EditComment`, `DeleteComment`, `SetReaction`, `DeleteReaction`, `SetPostCover`, `SetDayTitle`, `SetDayCover`; `UpdateIdentity`/`RenameIdentity` — `status = active`. Исключение — `ClearDayTitle`/`ClearDayCover` (находка №2). SEC-3 в текущем коде выполнен.
- Составные пути: `EditPost`/`DeletePost`/`SetPostCover` — `post.CircleID == circleID` (иначе 404); `EditComment`/`DeleteComment` — `comment.CircleID == circleID`; `DeleteReaction` — реакция ищется по `(circle, post, своя identity)`; `CreateComment`/`SetReaction` — `post.CircleID == in.CircleID`; `SetDayCover` — `post.CircleID`, `post.EntryDate`, `BlobOnPost`. IDOR «сущность из другого круга» невозможен; `post_id` в пути комментария не сверяется, но права всё равно по автору.
- Окно редактирования: снимок `edit_window_sec`/`editable_until` хранится на строке, `CanEdit` проверяется в каждом мутаторе (`content.go:158,235,636,680,715`, `media.go:293`, `day.go:451`); менять окно круга задним числом не помогает — у существующих строк своё. Значение `0` («летопись») — `EditableUntil == nil` → редактирование запрещено всегда. Обхода не нашёл.
- `assertPostInteractive` (`content.go:411-434`): комментарий/реакция — только к записи в отрезке читателя и не старее архивного среза. Для `EditPost`/`DeletePost` архивного среза нет — отмечено в таблице как замечание, не дефект (автор правит своё в своём окне).
- Владельческие операции в одной транзакции с проверкой (`QLT-1`): `TransferOwnership` (владелец, новый ≠ старый, новый active → несуществующий 404, ушедший 403), `Exclude` (владелец, цель ≠ владелец → себя исключить нельзя; `left_with_access` → отзыв can_read, CHR-1), `DeleteCircle` (владелец + имя), `forbidOwnerLeaveTx`. `SetMemberCanSettings` — владелец, цель active и не владелец. `RequireOwner`/`RequireSettings`/`requireSettingsTx` корректны.
- Чтение: `FeedSnapshot`, `GridSnapshot`, `MapSnapshot`, `DaysSnapshot`, `DayPostsSnapshot`, `ListMembers`, `FeedMetaForAccount` — все через `requireReader` (`gone` → 403; нужен хотя бы один can_read-отрезок) и фильтр `sqlVisibleAt`/`readScope.canRead` по `created_at`. `ListAccountCircles` — unread и последний summary через `sqlVisibleAtMembership`. `SearchCircle*` — `RequireReader`; `SearchAll` — `EXISTS` по отрезкам всех кругов учётки, кругов без членства не отдаёт.
- `Exclude`/`Leave` закрывают отрезок с `can_read = 0`; `LeaveWithAccess` — `can_read = 1`, `can_write = 0`, `ended_at = now`. `gone` не проходит `requireReader`, `SyncEvents`, поиск, `CanWrite`.
- Идемпотентность `(circle_id, client_id)`: уникальные индексы `idx_posts_client_id`/`idx_comments_client_id` (миграция 0013) — дублей нет; дефект только в проверке автора и повторном `AttachMediaInTx` (находка №1).
- Реакции: белый список из 4 ключей (`content.go:11-17`), одна на identity на запись (upsert), `MaxEmojiChars`.
- Лимиты текста (`limits.go`): `MaxTextBytes` — тело записи (`createPostInTx`, `EditPostInTx`), комментария (`CreateComment`, `EditComment`), заголовка дня (`SetDayTitle`); `MaxNameChars` — имя круга (`CreateCircle`, `PatchCircle`), имя identity (`JoinInTx`, `RenameIdentity`), `owner_name`. `entry_date` — строгий `YYYY-MM-DD`. Общий предел тела 1 МБ (`maxJSONBody`).
- Поиск: FTS-токены заворачиваются в `"…"` с удвоением кавычек, управляющие символы вырезаются, ≤256 байт и ≤8 слов, `limit` 1..50, `offset` отсутствует; фильтры — параметризованные; `author` — сравнение строки. Инъекции в синтаксис FTS нет. Триггеры FTS удаляют строки удалённых записей/комментариев (`0002_search_days.sql:42-80`); носитель `p.deleted = 0`.
- Push-payload (`push.Signal{CircleID, Type, Count}`) и `notifyAccounts` для упоминаний: текста записи нет; `MentionedAccountIDs` — только активные участники, не-участника упомянуть нельзя. Письма из этих путей не отправляются.
- Ответы: `feed`/`comments`/`reactions`/`events` отдают `identity_id`, `author_name`, `avatar_blob_id` — без `account_id`/e-mail; `GET /circles/{id}` не раскрывает `account_id` владельца; `identity` history — только своя. E-mail в участнических ответах не найден (`requester_email` — только в админском `handleAdminQuotaRequests`).
- `SetReadCursor` — своя строка по `account_id`, `seq >= 0`, монотонно вверх.
- Аватар: `ValidateOwnedComplete(sess.AccountID, blob)` — чужой блоб 403; `UpdateIdentity` — только active.
- `writeDomainError`/`writeError`: `ErrNotFound → 404`, `ErrForbidden → 403`, `ErrTooLong → 400`, `ErrInvalid → 400`; неизвестная ошибка — 500 без текста ошибки в ответе (в лог).
