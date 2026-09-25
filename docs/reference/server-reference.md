# Серверный слой — справочник

Сжатая выжимка из закрытого плана реализации. Эталоны: [wynd.html](../wynd.html), [stack.html](../stack.html), [screens.html](../visual/screens.html).  
Продуктовые правила: [журнал](../wynd.html#journal), [дни](../wynd.html#days), [квота](../wynd.html#quota), [серверы](../wynd.html#servers).

---

## Архитектура

```mermaid
flowchart TB
  subgraph bin [cmd/wynd]
    HTTP["net/http :7676"]
  end
  subgraph domain [internal]
    API[api]
    Auth[auth]
    Chron[chronicle]
    Store[store]
    Blob[blob]
    Mail[mail]
    Push[push]
    Jobs[jobs]
    Check[check]
    Archive[archive]
  end
  HTTP --> API
  API --> Auth
  API --> Chron
  API --> Blob
  API --> Mail
  API --> Push
  API --> Check
  API --> Archive
  Chron --> Store
  Blob --> Store
  Archive --> Chron
  Archive --> Blob
  Jobs --> Store
  Jobs --> Blob
  Jobs --> Mail
  Store --> SQLite[(wynd.db)]
  Blob --> FS[data/blobs]
```

---

## Ключевые решения

Зафиксировать в коде, не пересматривать по ходу.

- **Термин `circle` везде** в коде и JSON (`circle_id`, не `group`/`space`).
- **Идентификаторы — UUID v7.** Курсор sync — монотонный `seq` внутри инстанса.
- **Аккаунт ≠ идентичность.** Аккаунт: почта на этом сервере. Идентичность: имя и лицо в одном круге, версионна. Admin API не отдаёт связку «один email — несколько лиц».
- **У инстанса есть имя.** Короткая строка («Дом Ани»); задаётся при первом запуске. Идентификатор сервера — адрес.
- **Учётка двумя путями:** приглашение в круг или регистрация без круга. Режим инстанса: `open` | `invite` (умолч.) | `closed`.
- **Сервер не знает о других серверах.** Миграция круга между инстансами — не реализована; UUID заложены.
- **Хроника — источник sync, снимок — источник чтения.** Одна SQLite-транзакция: событие + проекция.
- **Журнал — сказанное и структура.** Подробнее — [образ, журнал](../wynd.html#journal).
- **Отрезки видимости** — интервалы `[started_at, ended_at)` на пару (account, circle). Повторный вход — новый интервал.
- **Окно правок** — параметр круга (0 = летопись). У каждой единицы сказанного своё окно; смена параметра действует только вперёд.
- **Три времени записи:** `created_at` (sync, лента), `entry_date` (отнесение, правится в окне), `captured_at` (EXIF, справочно).
- **День — проекция** `(circle_id, entry_date)`. Название и обложка — сказанное.
- **Цикл архивации по квоте** — событие круга. `cutoff_locked_at` при первом скачивании архива. Пока цикл активен, комментарий и реакция на пост с `created_at` раньше UTC-полуночи `cutoff_date` — `forbidden` (`assertPostInteractive`). Тот же хелпер режет запись вне отрезка видимости. ZIP и `PurgeBeforeCutoff` не менять: комментарии после отсечки в архив не входят; purge стирает ветку поста до cutoff.
- **Сессии участников — Bearer.** Админ — отдельная сессия, журнал через неё недоступен.
- **SQLite** через `modernc.org/sqlite`. WAL, `busy_timeout`, `foreign_keys`. Один процесс на файл БД; второй на том же `wynd.db` вне модели. Пул `SetMaxOpenConns(1)` не ставим. Интерфейс `store.Store` — Open/Close/Ping/Version; драйвер один.
- **Миграции.** Baseline `0001_schema.sql`; поверх — `0002_search_days`, `0003_notify_prefs`, `0004_payment`, `0005_search_day_replace`. Текущая схема **v5**. Старый `wynd.db` с версией не из этой цепочки — удалить.
- **Список кругов аккаунта.** `ListAccountCircles` — три batch-запроса (курсоры, unread постов, последнее видимое событие), не N+1 на круг. Видимость — `sqlVisibleAtMembership`.
- **Время в БД** — `RFC3339Nano` (`internal/xtime`). JSON снаружи может остаться секундами (`RFC3339`).
- **Снимок ленты/сетки** — потолок `chronicle.SnapshotPostLimit` (2000 постов). Круг больше — вне модели, не пагинация API.
- **SSE** — опрос SQLite раз в 2 с, без LISTEN/NOTIFY.
- **Liveness / ready.** `GET /health` — процесс жив. `GET /ready` — `Ping` SQLite.
- **Бэкап** — `VACUUM INTO`, не копия голого `wynd.db` при WAL.
- **HTTP — stdlib**, порт **7676**.
- **Push notify.** `notifyCircle` логирует ошибки членства, prefs и `SendSignal`. Горутина — 10 с; `WaitNotify` после `http.Server.Shutdown` (тот же ctx, 10 с).
- **SMTP / VAPID не настроены.** `mail.ErrNotConfigured` / `push.ErrNotConfigured` → 400 `smtp_not_configured` / `push_not_configured`, не 500. Сбой отправки → 502 `smtp_failed` (с `detail`), не 500. Порт 465 — implicit TLS, иначе STARTTLS; разговор с релеем не дольше 15 с. `MAIL FROM` — только адрес из «От кого». Ошибка JSON encode и немаппленный domain-error — в лог.
- **Имя файла блоба.** `sanitizeFilename` обрезает по рунам (255), не по байтам.
- **Блобы** — без ffmpeg. SVG и HTML всегда `Content-Disposition: attachment`.
- **Код на английском**, строки писем и пушей — на русском.
- **Квота круга.** `instance_settings.default_circle_quota_bytes` (`NULL` = нет потолка круга). `circles.quota_custom`: 0 → умолчание инстанса; 1 → `circles.quota_bytes` (`NULL` = без квоты). Смена умолчания круги не UPDATE-ит. `PUT /admin/storage/default_quota` не смешивать с `PUT /admin/storage/quota`. `PUT /admin/circles/{id}/quota` `{custom, quota_bytes?}`: `custom=false` игнор тела; `custom=true` + число — своя; без числа — без квоты. Ниже `media_bytes` и выше потолка инстанса ставить можно. Событие хроники не писать. Pending при PUT: `approved` + `resolved_at` (абсолют). `POST .../approve` ставит `quota_custom=1` и `quota_bytes = requested` (абсолют); UI «Дать» зовёт `PUT .../quota`, не approve.
- **Учётка в панели.** `GET /admin/accounts` без `deleted_at`; `circle_count` — только `memberships.status='active'`. `GET /admin/accounts/{id}`: `{id,email,created_at,last_login_at,blocked,owns_circle,circles:[]}` — лица нет, `circles` всегда массив, `joined_at` из `memberships.created_at`. `last_login_at` — только после успешного participant-кода. `DELETE /admin/accounts/{id}` — soft-delete, не `DELETE FROM accounts` (blobs `ON DELETE CASCADE`): владелец круга → 409; sentinel → 404; `LeaveInTx` (Gone) и soft-delete в **одной** транзакции; `identities.account_id` NULL (`UNIQUE (circle_id, account_id)` тогда допускает несколько отвязанных лиц на круг — SQLite NULL ≠ NULL); `deleted_at`, `blocked=1`, email `deleted+{id}@wynd.local`; сессии как при block. Повторная регистрация той же почты — новый account. Записи и хронику не трогать. `jobs.cleanEmptyAccounts` — тот же soft-delete, не `DELETE FROM accounts`: `created_at` старше 30 дней и нет ни одной строки `memberships` (любой `status`); владелец круга джобом не трогается.
- **Реакции.** В API только ключи `heart` | `laugh` | `surprise` | `anger`; иначе `ErrInvalid`. Миграции старых эмодзи нет. `reactionResponse` несёт окно правок. `DELETE .../posts/{post_id}/reactions` ищет свою реакцию на пост, затем `DeleteReaction` по id.
- **Серверный инвайт.** `createInviteBody.ttl_sec`; иначе неделя. Инвайт круга — `ttl_sec`; иначе 3 дня. `GET /invites/{token}` для `circle_id IS NULL` отдаёт `{server_name, host, inviter_name?}` без круга (не 404). `inviter_name` — живое лицо создателя ссылки; у sentinel его нет, тогда имя владельца самого старого круга; кругов нет — поле опущено, на 1.7 только адрес.
- **Живые ссылки.** `GET /circles/{id}/invites` — неотозванные, неистёкшие, с остатком лимита. `DELETE /circles/{id}/invites/{id}` — отзыв (только свой круг). Панель: `GET/DELETE /admin/invites` — то же для серверных ссылок (`circle_id IS NULL`).
- **Текст.** Тело записи, комментария и названия дня — не длиннее 32768 байт UTF-8. Создание: `TrimSpace(body)==""` без media и комментарий из пробелов — `invalid`; в базу пишется уже обрезанный текст. Правка записи из пробелов без медиа — тоже `invalid`. `entry_date` — строго `YYYY-MM-DD` (parse + format round-trip; `2026-02-30` не принимается).
- **Поиск.** FTS `kind` = `post` \| `comment` \| `day`. День: пустой `post_id`, `LEFT JOIN posts`. Смена названия дня — INSERT в `day_titles`; триггер `fts_day_insert` заменяет предыдущую FTS-строку дня. Фильтры query: `from`, `to`, `has_photo=1`, `has_location=1`, `author` (только поиск круга). `GET /circles/{id}/search/authors` — DISTINCT `author_name` из попаданий FTS (до 100), не участники; те же фильтры, кроме `author`.
- **Notify.** `comments_mine` / `comments_all` / `events` / `mute_until`. Умолчание реакций — выкл. Упоминание пробивает mute. Пуш `mention` при создании записи и комментария (правка тела — нет).
- **Оплата.** `0004_payment.sql`: реквизиты, баннер сбора (`until` — `YYYY-MM-DD`, действует до конца этой даты UTC), подписка. Продление `max(now, subscription_expires_at)+days`. Скриншот обязателен. Скрины одобренных заявок: файл удаляется после истечения срока, строка `blobs` остаётся из-за FK. Отклонение заявки удаляет скрин сразу. Шлюз — layout `/circles` и `/search` **и** API кругов (`RequirePaidParticipant` → 403 `payment_required`, когда `required && expired && has_requisites`). `PayStatus.reminder` не ставится при `pending`. Письмо напоминания — SMTP `mail.notify`, плюс пуш `subscription`. Версию баннера поднимать только когда `show` и текст/срок реально меняются.
- **Проверка 9.5.** HTTP→HTTPS 301/308 считает сервер (`check.ProbeHTTPRedirect` по `public_url`), не браузер. Браузерная проба тела — `body_probe_bytes` из `GET /probe` (`min(attachment_max_bytes, 2 МБ)`).
- **Код входа.** `GET /instance` отдаёт `code_delivery`: `log` на loopback, если SMTP не сконфигурирован или проверка SMTP не удалась, иначе `mail`. Лимит кодов: `retry_after_sec` в JSON и `Retry-After` (секунды до выхода самой старой записи из часового окна; email 5 и IP 10 — больший из двух). Почта: `ParseParticipantEmail` (`net/mail.ParseAddress`, local и domain непусты, в domain есть `.`).
- **Имя в круге.** `addIdentityName` копирует `avatar_blob_id` с предыдущей неистёртой строки той же идентичности.
- **Окно правок.** `SetEditWindow` отвергает отрицательные секунды (`invalid`); `0` — летопись, `nil` — без ограничения.

---

## Локальный прогон

**Loopback:** host `public_url` — `127.0.0.1`, `localhost`, `::1`. Каталог данных — `dev/data`.

**Поднять:** `go run ./cmd/wynd`. Smoke: `curl http://127.0.0.1:7676/health`.

**На loopback:** bootstrap без SMTP; код входа в лог без релея; экран `/auth/code` при `code_delivery=log` говорит «Код с сервера», не про письмо; `GET /api/v1/instance` отличает закрытый сервер от сломанного; HTTPS/прокси/«снаружи» — «не применимо», не FAIL.

**Здесь закрывается:** хроника, auth, журнал, sync/SSE, поиск, admin API, квота и архив, `go test ./...`.

**Здесь не закрывается:** Let's Encrypt, прокси-заголовки, Web Push на localhost, systemd. `http://192.168.x.x` — не loopback-профиль.

---

## Инварианты для тестов (хроника)

- новичок не видит прошлое;
- вышедший с доступом видит до выхода, не пишет;
- вышедший совсем и исключённый не читают; хроника — «покинул круг»;
- повторный вход не открывает дыру между отрезками;
- летопись запрещает правку и удаление;
- смена окна не трогает опубликованное;
- отрицательное окно правок — `invalid`;
- комментарий и реакция — своё окно от момента публикации;
- комментарий и реакция на пост вне отрезка видимости или до отсечки активного архива — `forbidden`;
- пустое (после trim) тело поста без media и комментарий из пробелов — `invalid`; несуществующая календарная дата — `invalid`;
- удаление ветки: в БД нет текста, в журнале нет строки об удалении;
- backdated-запись не меняет порядок ленты и курсоры непрочитанного;
- удаление последней записи дня уносит день; новая запись тем же числом возрождает пустой день;
- `day_cover_set` без записи за день — отклоняется;
- чистка `identity_names` не меняет события, имя больше не разрешается;
- смена имени копирует `avatar_blob_id` на новую строку `identity_names`.

## Инварианты для тестов (панель)

- default 5 ГБ + `quota_custom=0` режет медиа; своя `NULL` не режет; смена default не трогает `custom=1`.
- `DELETE /admin/accounts` владельца — 409; блобы участника не каскадятся.
- `SetReaction` отклоняет ключ вне whitelist; `circle_count` не считает `gone`.

## Инварианты для тестов (архив)

- cutoff двигается до первого скачивания и не двигается после;
- архивы двух участников с разными отрезками не совпадают;
- архив открывается офлайн, без внешних ссылок в HTML;
- удаление по deadline срабатывает, даже если скачали не все;
- после удаления служебные события до cutoff остались в логе.

---

## Вне scope

- UI, IndexedDB, service worker, офлайн-очередь
- Сжатие медиа на клиенте
- PostgreSQL, UnifiedPush, вёрстка писем
- Миграция круга между серверами
- Группы кругов
- GDPR-экран удаления профиля участником
- SQL `DELETE FROM accounts` из карточки людей; отсечка из панели; лица в GET учётки; событие квоты в хронике
