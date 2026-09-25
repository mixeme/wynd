# Ревью 2: блобы, архив, задачи, поиск

Область: `internal/blob/`, `internal/archive/`, `internal/backup/`, `cmd/wynd/backup.go`,
`internal/jobs/`, `internal/search/`, FTS-триггеры в миграциях 0001/0002/0005.
Только чтение; ничего в репозитории не изменялось.

---

## Ссылки на блобы и сборка мусора (системное решение)

### 1.1 Полный перечень колонок, ссылающихся на blob id — CONFIRMED

Grep по миграциям даёт шесть мест, откуда на блоб может быть ссылка:

| Таблица.колонка | FK на blobs | Кто учитывает |
|---|---|---|
| `blob_refs.blob_id` | есть, `ON DELETE CASCADE` (0001:22-27) | `cleanOrphanedBlobs`, `gcBlobIfUnreferenced`, `auth/pay.go:730` |
| `post_media.blob_id` | есть, **без** cascade (0001:98) | все три |
| `day_covers.blob_id` | **нет FK** (0001:92-104) | только `gcBlobIfUnreferenced` и `pay.go:730` |
| `days.cover_blob_id` | **нет FK** (0001:127) | **никто** |
| `identity_names.avatar_blob_id` | **нет FK** (0001:158) | только `gcBlobIfUnreferenced` и `pay.go:730` |
| `pay_requests.blob_id` | есть, **без** cascade (0004:17) | **никто** |

Три независимые реализации предиката «на блоб кто-то ссылается», и все три разные:

* `internal/jobs/routine.go:85-93` — `blob_refs` + `post_media`;
* `internal/blob/serve.go:179-201` — `blob_refs` + `post_media` + `day_covers` + `identity_names`;
* `internal/auth/pay.go:~725-735` — копия предыдущего.

Отсюда оба подтверждённых дефекта (A из вводной): аватары (`identity_names`) и
скриншоты оплаты (`pay_requests`) не видны `cleanOrphanedBlobs`; последний ещё и
ломает всю дневную рутину, потому что `os.Remove` (routine.go:121) идёт **до**
`DELETE FROM blobs` (routine.go:123), а DELETE падает на FK из `pay_requests` →
файл уже удалён, строка осталась, `RunDailyRoutine` возвращает ошибку.

**Severity: критическая** (потеря пользовательских данных + отказ всей рутины).

### 1.2 ОДНО системное исправление

Ввести в `internal/blob` единственный экспортируемый предикат и звать его из
обоих мест (и из `auth/pay.go` вместо копии). SQL (одним запросом, без
конкатенации таблиц в Go):

```sql
-- blob.IsReferenced(ctx, q, blobID) -> bool
SELECT EXISTS (SELECT 1 FROM blob_refs        WHERE blob_id = :id)
    OR EXISTS (SELECT 1 FROM post_media       WHERE blob_id = :id)
    OR EXISTS (SELECT 1 FROM day_covers       WHERE blob_id = :id AND deleted = 0)
    OR EXISTS (SELECT 1 FROM days             WHERE cover_blob_id = :id)
    OR EXISTS (SELECT 1 FROM identity_names   WHERE avatar_blob_id = :id AND erased_at IS NULL)
    OR EXISTS (SELECT 1 FROM pay_requests     WHERE blob_id = :id);
```

Для пакетного прохода `cleanOrphanedBlobs` — тот же предикат в виде `NOT EXISTS`-набора:

```sql
SELECT b.id, b.storage_path
FROM blobs b
WHERE b.status = 'complete'
  AND b.created_at < :cutoff
  AND NOT EXISTS (SELECT 1 FROM blob_refs      r  WHERE r.blob_id        = b.id)
  AND NOT EXISTS (SELECT 1 FROM post_media     pm WHERE pm.blob_id       = b.id)
  AND NOT EXISTS (SELECT 1 FROM day_covers     dc WHERE dc.blob_id       = b.id AND dc.deleted = 0)
  AND NOT EXISTS (SELECT 1 FROM days           d  WHERE d.cover_blob_id  = b.id)
  AND NOT EXISTS (SELECT 1 FROM identity_names inm WHERE inm.avatar_blob_id = b.id AND inm.erased_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM pay_requests   pr WHERE pr.blob_id       = b.id);
```

Порядок операций в цикле удаления обязан быть обратным нынешнему: сначала
`DELETE FROM blobs WHERE id = ?`, и только если он прошёл — `os.Remove`. Тогда
любой недоучтённый в будущем FK даёт «блоб остался цел», а не «файл потерян».

Тест на регрессию: загрузить блоб, привязать его как аватар (и вторым тестом —
как скриншот оплаты), сдвинуть `created_at` на 48 ч назад, прогнать
`RunDailyRoutine`, проверить, что `blobs`-строка и файл на месте, а ошибки нет.

### 1.3 `RunDailyRoutine`: ранний `return counts, err` — CONFIRMED

`internal/jobs/routine.go:49-81`: семь шагов, после каждого `if err != nil { return
counts, err }`. Падение первого же шага (а он, см. 1.1, падает гарантированно при
наличии хоть одного скриншота оплаты старше суток) отменяет очистку брошенных
загрузок, пустых аккаунтов, инвайтов, сессий, кодов и запись `last_routine_at`.
Поскольку `last_routine_at` не обновляется, планировщик будет пытаться снова и
снова и снова падать на том же блобе — рутина мертва навсегда.

**Severity: критическая.** Форма исправления — выполнять все шаги, агрегировать
ошибки:

```go
var errs []error
step := func(name string, n *int, f func() (int, error)) {
    v, err := f()
    *n = v
    if err != nil { errs = append(errs, fmt.Errorf("%s: %w", name, err)) }
}
step("orphaned_blobs", &counts.OrphanedBlobs, func() (int, error) { ... })
...
// last_routine_at пишем всегда: проход состоялся, пусть и частично
return counts, errors.Join(errs...)
```

Вызывающая сторона логирует `errors.Join`, а не только первую ошибку.

---

## Архивный цикл и освобождение места

### 2.1 Корень B: `PurgeBeforeCutoff` не снимает `blob_refs` — CONFIRMED

`internal/chronicle/archive.go:682-694` (`purgePostBranch`) собирает blob id из
`post_media`, скрабит ветку и делает `DELETE FROM post_media WHERE post_id = ?`.
Строки `blob_refs (ref_type='post', ref_id=postID)` не трогаются нигде в
`archive.go`. Далее `jobs/archive.go:45-49` зовёт `blobs.ReleaseBlobs`, а тот
(`blob/serve.go:179-188`) первым делом считает `SELECT COUNT(*) FROM blob_refs
WHERE blob_id = ?`, получает ≥ 1 и **возвращается, ничего не освободив**.

Контрольный пример правильного порядка — `internal/api/journal.go:186-190`:
там перед `ReleaseBlobs` вызывается `RemoveRefsFor(ctx, "post", postID)`.

**Severity: высокая** (место не освобождается, удалённая медиа остаётся отдаваемой
по прямой ссылке — обход «архивного забвения»).

Исправление (одно): в `purgePostBranch` после удаления `post_media` выполнить
`DELETE FROM blob_refs WHERE ref_type = 'post' AND ref_id = ?` в той же транзакции.
`ReleaseBlobs` после коммита тогда отработает.

### 2.2 Удаление круга не освобождает ни байта — CONFIRMED

`internal/chronicle/members.go:266-284`: `DeleteCircle` проверяет владельца и имя,
затем `DELETE FROM circles WHERE id = ?` — и всё. Каскады уносят `posts`,
`post_media`, `days`, `day_covers`, `identities`, `identity_names`. Но:

* `blob_refs` каскадом **не** затрагивается (FK только `blobs → blob_refs`), так что
  строки `('post', postID)` остаются навсегда;
* `blob.ReleaseBlobs` / `RemoveRefsFor` не вызываются ни здесь, ни в HTTP-слое
  (`internal/api/circle_settings.go:232-245` просто зовёт `DeleteCircle`);
* следовательно `cleanOrphanedBlobs` тоже никогда их не подберёт — у блоба
  по-прежнему есть строка в `blob_refs`.

Итог: все файлы удалённого круга остаются на диске и остаются **отдаваемыми**
(`CanAccessBlob` вернёт true владельцу блоба, см. 6.1) неограниченно долго.

**Severity: высокая.** Исправление (одно): в `DeleteCircle` перед `DELETE FROM
circles` собрать `SELECT DISTINCT pm.blob_id FROM post_media pm JOIN posts p ON
p.id = pm.post_id WHERE p.circle_id = ?` плюс аватары круга, удалить
соответствующие `blob_refs`, и после успешного `DELETE FROM circles` вызвать
`blobs.ReleaseBlobs(ids)`. После исправления 1.2 достаточно и второго пути:
исправленный `cleanOrphanedBlobs` подберёт их в течение суток — но только если
`blob_refs` вычищены.

### 2.3 «Квота круга» пересчитывается на лету и не включает половину блобов — CONFIRMED

Колонки `media_bytes` в схеме нет; `internal/api/admin_storage.go:81` отдаёт поле
`media_bytes`, значение берётся из `blob.CircleUsedBytes`, а тот
(`internal/blob/quota.go:127-139`) суммирует **только** `post_media` живых постов.
Поэтому в квоту круга не попадают: аватары (`identity_names.avatar_blob_id`),
обложки дней (`day_covers.blob_id`, `days.cover_blob_id`), скриншоты оплаты и
незавершённые загрузки. Место они занимают, в панели не видны, за лимит круг
выйти может незаметно. Плюс: после purge (2.1) число падает сразу, а байты на
диске — нет, то есть панель показывает освобождение, которого не произошло.

**Severity: средняя.** Исправление: считать `circleUsedBytes` как `SUM(size_bytes)`
по всем блобам, на которые ссылается что-либо, принадлежащее кругу (UNION четырёх
источников с `DISTINCT b.id`), а не только по `post_media`.

---

## Загрузка и квоты

### 4.1 Окно краха между файлом и строкой в БД + нечищеные «сироты на диске» — CONFIRMED

`internal/blob/upload.go:212-243` (`CompleteSession`): сначала `copyFile(part,
dest)` + `os.Remove(part)`, и только потом транзакция `INSERT INTO blobs` +
`DELETE FROM upload_sessions`. Три проблемы в одном месте:

1. Крах (или ошибка `INSERT`) между 221 и 241 оставляет файл в `blobs/xx/...`, на
   который нет строки. `cleanOrphanedBlobs` ходит от таблицы `blobs` к файлам и
   такие файлы не видит **никогда** — обхода каталога в репозитории нет
   (grep по `filepath.Walk`/`WalkDir` в `internal/blob` пуст). Утечка места навсегда.
2. `os.Remove(path)` (221) выполняется до коммита: если транзакция не прошла,
   сессия в БД жива, но `.part` уже нет → клиент не может ни докачать, ни
   завершить; HEAD вернёт `received_bytes == expected`, а `CompleteSession` упадёт
   на `fileSHA256` с ENOENT до истечения TTL.
3. `copyFile` вместо `os.Rename` внутри одного каталога данных: двойной ввод-вывод
   и требование 2× места на каждый файл (при 100 МБ-вложениях по умолчанию —
   заметно). `.uploads` и `blobs/xx` лежат в одном `s.dir`, так что rename атомарен.

**Severity: средняя-высокая.** Одно исправление: сначала транзакция (вставить
`blobs` со `status='pending'`, удалить сессию, коммит), затем `os.Rename(part,
dest)`, затем `UPDATE blobs SET status='complete'`. Тогда любой крах оставляет
`pending`-строку, которую рутина уже умеет подбирать по `status`, а не безымянный
файл. Тест на регрессию: подменить `db` так, чтобы `INSERT` падал, и убедиться,
что после ошибки в `blobs/` не осталось файла.

### 4.2 Параллельные чанки одной сессии не сериализованы — PLAUSIBLE

`WriteChunk` (upload.go:140-178) читает `received_bytes`, сверяет `offset`, пишет,
затем `UPDATE upload_sessions SET received_bytes = ?`. Ни транзакции, ни
`UPDATE ... WHERE received_bytes = :offset`, ни мьютекса по `sessionID`. Два
одновременных PUT с одинаковым `offset` оба пройдут проверку 145 и оба будут
писать в один дескриптор со `Seek` — содержимое перемешается, `received_bytes`
получит значение последнего. Ошибка вскроется только на `sha256` в `Complete`.

Исправление (одно): сделать продвижение счётчика условным и атомарным —
`UPDATE upload_sessions SET received_bytes = ? WHERE id = ? AND received_bytes = ?`
и при `RowsAffected()==0` возвращать `ErrInvalid`, а запись в файл вести через
`f.WriteAt` под именованным мьютексом сессии. Тест: два параллельных `WriteChunk`
с одним offset → ровно один успех.

### 4.3 Перелёт на 1 MiB портит `.part` безвозвратно — CONFIRMED

upload.go:161-169: `io.LimitReader(r, remaining+chunkOverhead)` сначала **пишет** в
файл до 1 MiB сверх ожидаемого и лишь затем возвращает `ErrInvalid`. Файл
открывается `O_WRONLY` без `O_TRUNC` и никогда не усекается, поэтому «хвост»
остаётся; `fileSHA256` считает хэш по всему файлу, а не по `expected_size`.
Итог: одна ошибка клиента делает сессию неисправимой — корректная докачка всё
равно даст неверный хэш.

Исправление: `written, err := io.Copy(f, io.LimitReader(r, remaining))`, а признак
перелёта определять попыткой прочитать один лишний байт; при любой ошибке
делать `f.Truncate(offset)`.

### 4.4 Проверка квоты — классический check-then-act, `pending` не учитываются — CONFIRMED

`CreateSession` зовёт `CheckMediaQuota(ctx, "", expectedSize)` (upload.go:66), а
`usedBytes` (quota.go:116-125) суммирует только `status = 'complete'`.
Открытые сессии (`upload_sessions.expected_size`) не учитываются нигде.
Значит N параллельных сессий по X байт все пройдут проверку и все завершатся:
потолок инстанса превышается в N раз. Повторной проверки в `CompleteSession` нет.

**Severity: средняя.** Одно исправление: включить в `usedBytes` слагаемое
`(SELECT COALESCE(SUM(expected_size),0) FROM upload_sessions WHERE expires_at >= :now)`
и повторить `CheckMediaQuota` внутри транзакции `CompleteSession`.

### 4.5 Ни одного ограничения на число сессий у аккаунта — CONFIRMED

Grep по `upload_sessions` не находит ни `COUNT(*) ... WHERE account_id`, ни лимита.
Любой участник может открыть произвольное число сессий; каждая создаёт файл в
`.uploads`. Чистятся они только по TTL (`cleanAbandonedUploads`, routine.go:133),
то есть раз в сутки и только для истёкших. **Severity: низкая-средняя** (домашний
инстанс, доверенный круг), исправление — лимит вида «не более 8 активных сессий
на аккаунт» в `CreateSession`.

### 4.6 MIME только со слов клиента; sniffing отсутствует — CONFIRMED

`CreateSession` принимает `in.MimeType` как есть (проверяется лишь непустота и
длина ≤255, upload.go:53-58), и это значение попадает в `blobs.mime_type` и дальше
в заголовок ответа. Содержимое не нюхается (`http.DetectContentType` в
`internal/blob` не встречается). Защита сведена к `executableExtension`
(serve.go:56-70), которая ловит фиксированный список из шести строк: объявив
`image/webp` для HTML-файла, получаем отдачу HTML с этим Content-Type. Поскольку
`Disposition` жёстко `attachment` (serve.go:18,52), это не XSS, но проверять стоит.
Одно исправление: в `CompleteSession` вызвать `http.DetectContentType` по первым
512 байтам и, если сниффнутый тип относится к «активным» (`text/html`,
`image/svg+xml`, `application/javascript`), записывать `application/octet-stream`
независимо от объявленного.

---

## Поиск

### 3.1 Видимость проверяется по `created_at` самой строки FTS — CONFIRMED

`internal/search/search.go:143-150` (и дубликат в `SearchCircleAuthors`,
:83-90) — единственное условие видимости:

```sql
AND EXISTS (SELECT 1 FROM memberships m JOIN membership_spans ms ...
            WHERE m.circle_id = f.circle_id AND m.account_id = ?
              AND ms.can_read = 1
              AND f.created_at >= ms.started_at
              AND (ms.ended_at IS NULL OR f.created_at < ms.ended_at))
```

`f.created_at` — это время создания **самой** записи FTS. Для `kind='comment'` это
время комментария, а не поста. Отсюда подтверждённый дефект C: комментарий,
написанный внутри моего промежутка к посту до него, находится, хотя сам пост в
ленте не виден. Через `snippet` и `thumb_blob_id` (search.go:136-139) утекают
и текст комментария, и **id блоба из скрытого поста** — то есть тут же прямая
ссылка на медиа, которую `CanAccessBlob` не обязана отклонить.

То же для `kind='day'`: заголовок дня, поставленный внутри моего промежутка, но
относящийся к `entry_date` вне его, проходит проверку — сравнивается
`day_titles.created_at`, а не `entry_date`. Никакой привязки к дню нет.

**Severity: высокая** (нарушение центральной модели «промежутков членства»).

Исправленный предикат — один, общий, разбирающий три вида:

```sql
AND EXISTS (
  SELECT 1
  FROM memberships m
  JOIN membership_spans ms ON ms.membership_id = m.id AND ms.can_read = 1
  LEFT JOIN posts pp ON pp.id = f.post_id AND f.post_id != ''
  WHERE m.circle_id = f.circle_id
    AND m.account_id = :account
    -- сама запись внутри промежутка
    AND f.created_at >= ms.started_at
    AND (ms.ended_at IS NULL OR f.created_at < ms.ended_at)
    -- и её «носитель» тоже
    AND (
      f.kind = 'post'
      OR (f.kind = 'comment'
          AND pp.id IS NOT NULL AND pp.deleted = 0
          AND pp.created_at >= ms.started_at
          AND (ms.ended_at IS NULL OR pp.created_at < ms.ended_at))
      OR (f.kind = 'day'
          AND f.entry_date >= substr(ms.started_at, 1, 10)
          AND (ms.ended_at IS NULL OR f.entry_date <= substr(ms.ended_at, 1, 10)))
    )
)
```

Тест на регрессию: пост от A до входа B; комментарий B-времени под ним; поиск от
имени B по слову из комментария → 0 попаданий; аналогично заголовок дня с
`entry_date` до входа B.

### 3.2 `ftsEscape` — фраза, а не набор слов; управляющие символы не отсекаются — CONFIRMED

`search.go:191-195`:

```go
func ftsEscape(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}
```

Вся строка становится одной FTS5-фразой: «вчера дождь» найдёт только точное
соседство слов (подтверждено C). Ни длина, ни число токенов, ни управляющие
символы не ограничены; NUL внутри `q` превращает запрос в «unterminated string»
на стороне SQLite и отдаётся как 500 (тоже C).

**Severity: средняя** (функциональная — поиск для пользователя почти бесполезен —
плюс 500 на управляющем символе).

Одно решение, закрывающее оба пункта: разбирать `q` на токены и собирать
конъюнкцию цитированных токенов.

```go
func ftsQuery(q string) (string, error) {
	q = strings.Map(func(r rune) rune {
		if r == '�' || (unicode.IsControl(r) && r != ' ') { return -1 }
		return r
	}, q)
	if len(q) > 256 { q = q[:256] }                      // потолок длины
	fields := strings.Fields(q)
	if len(fields) == 0 { return "", chronicle.ErrInvalid }
	if len(fields) > 8 { fields = fields[:8] }           // потолок числа токенов
	parts := make([]string, len(fields))
	for i, t := range fields {
		parts[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
	}
	return strings.Join(parts, " AND "), nil
}
```

Семантика: все слова присутствуют, порядок не важен, префиксного поиска нет
(решение закрыто). Точная фраза остаётся доступной, если пользователь сам
заключил запрос в кавычки — этот случай стоит распознать отдельно, но это уже
второй шаг.

### 3.3 Триггеры FTS не срабатывают на каскадных удалениях — CONFIRMED

`internal/store/sqlite.go:19,47-49` включает `foreign_keys(1)`, `busy_timeout` и
журнал, но **не** `recursive_triggers`. В SQLite `AFTER DELETE`-триггеры не
выполняются для строк, удалённых действием внешнего ключа `ON DELETE CASCADE`,
пока не включён `PRAGMA recursive_triggers = ON`.

Практически это значит: `DELETE FROM circles` (members.go:282) каскадом уносит
`posts`, `comments`, `day_titles`, но `fts_post_delete` (0001:378),
`fts_comment_delete` (0001:357) и `fts_day_delete` (0001:397) не срабатывают —
**весь текст удалённого круга остаётся в `content_fts` навсегда**. Из ленты он
исчезает (проверка членства ничего не найдёт, членства тоже каскадом удалены),
но текст остаётся в файле БД и попадает в каждый бэкап `VACUUM INTO`. То же при
каскадном удалении аккаунта.

**Severity: средняя** (приватность: «удалить» не означает «удалить»; плюс рост
индекса). Исправление (одно): добавить `recursive_triggers(1)` в тот же набор
прагм в `internal/store/sqlite.go` и проверить его в `verifyPragmas` рядом с
`foreign_keys`. Тест: создать круг с постом, удалить круг, убедиться, что
`SELECT COUNT(*) FROM content_fts WHERE circle_id = ?` равен нулю.

### 3.4 Триггеры дня затирают заголовок «соседней» версии — CONFIRMED

Все три дневных триггера (0001:397-414, переопределены в 0002 и 0005) адресуют
строку FTS по паре `(circle_id, entry_date)`, тогда как `day_titles` хранит
историю версий по `event_seq`. Поэтому:

* `fts_day_update` при правке **старой** версии сначала удаляет строку FTS дня, а
  затем вставляет заново из `NEW` — то есть в индексе оказывается устаревший
  заголовок вместо актуального;
* `fts_day_delete` при удалении любой старой версии удаляет строку целиком, не
  восстанавливая актуальную: день просто перестаёт находиться;
* тот же эффект даёт purge — `DELETE FROM day_titles WHERE circle_id = ? AND
  created_at < ?` (chronicle/archive.go:646-648) выносит строки FTS для дней,
  у которых есть и более свежий, не подпадающий под отсечку заголовок.

Исправление (одно): после `DELETE` во всех трёх триггерах вставлять не `NEW`, а
актуальную версию — `SELECT title ... FROM day_titles WHERE circle_id = ... AND
entry_date = ... AND deleted = 0 AND title != '' ORDER BY event_seq DESC LIMIT 1`
(ровно такой запрос уже написан в конце 0002 для первичного наполнения —
оттуда его и взять).

### 3.5 Мелочи поиска

* `search.go:130` берёт `limit*3` строк и обрезает до `limit` в Go
  (`collectHits:164`) — при этом `ORDER BY rank` применяется до фильтрации, так что
  «лишние» строки — чистая трата. После 3.1 (вся фильтрация в SQL) множитель
  надо убрать. Severity: низкая.
* `filterSQL` (`HasPhoto`, `HasLocation`, :209-221) джойнит `posts pp ... deleted = 0`,
  а основной запрос — нет; то есть фильтры неявно строже самого поиска.
  Severity: низкая, но признак того, что `p.deleted = 0` забыт в главном запросе:
  soft-deleted пост с непустым телом в FTS не попадает (триггер его снимает), а
  вот **комментарий** к soft-deleted посту — попадает (см. 3.1).
* `Hit` (:32-35) — поля с рассогласованными отступами; `gofmt` бы выровнял.
  Стоит проверить, что `gofmt -l` на пакете чист.
