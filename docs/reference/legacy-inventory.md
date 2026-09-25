# Legacy-код и миграции

Инвентаризация обратной совместимости и оставшегося долга.
Срез: 2026-09-09. Текущая схема БД: **версия 1** (`internal/store/migrations/0001_schema.sql`).

Связанные документы: [server-reference.md](server-reference.md).

---

## SQL-миграции (`internal/store/migrations/`)

Мигратор: `internal/store/migrate.go`. Только вверх, без down. Следующая миграция — `0002_*.sql`.

| Версия | Файл | Назначение |
|--------|------|------------|
| 1 | `0001_schema.sql` | Полная схема (baseline после снятия цепочки 0001–0013) |

### Особенности migrator

- **`schema_migrations`** создаёт код, не SQL-файл. `applied_at` — `RFC3339Nano`.
- **Старый `wynd.db`** с `MAX(version)` ∉ {0, 1} — ошибка «удалите wynd.db», без автопочинки.
- Тесты: `TestOpenMigrateClose`, `TestReopenAppliesMigrationsOnce` (ожидает 1 строку),
  `TestSchemaAllowsNullableIdentityAccountID`.

---

## Совместимость API и форматов

### Инвайты

| Место | Поведение |
|-------|-----------|
| `internal/api/admin.go`, `internal/api/circles.go` | `ttl_sec`; иначе default (7 д / 3 д) |
| `web/src/lib/circles/settings.ts`, `web/src/lib/admin/admin.ts` | Клиент шлёт `ttl_sec` |

### `RFC3339Nano` (БД) vs `RFC3339` (JSON)

| Слой | Формат | Код |
|------|--------|-----|
| SQLite writes | `RFC3339Nano` | `internal/xtime` через `auth/timefmt.go`, `chronicle/timefmt.go`, `blob/timefmt.go` |
| JSON API | `RFC3339` (секунды) | `internal/api/*.go` |
| Legacy read | Оба | `xtime.Parse()` — сначала Nano, затем секунды |
| Purge expired | Shim | `jobs.expireBefore()` — сравнение legacy RFC3339 без дробных секунд |

Event payload `captured_at` в хронике — `RFC3339` (`chronicle/media.go`), не Nano.

### Реакции

В API только ключи `heart` | `laugh` | `surprise` | `anger`. Миграции старых эмодзи нет.

### Soft-delete учёток (Wave F)

- `accounts.deleted_at`, email → `deleted+{id}@wynd.local`, `blocked=1`
- `identities.account_id` → NULL (несколько отвязанных лиц на круг — следствие модели)
- Повторная регистрация той же почты — **новый** account
- Sentinel admin DELETE → 404
- `jobs.cleanEmptyAccounts` — тот же soft-delete, не `DELETE FROM accounts`

### SMTP fallback (loopback)

`internal/mail/mail.go` — при недоступном SMTP коды уходят в `auth.LogCodes` (fallback).

---

## Незавершённые миграции (долг)

### `$lib/components` → `$ui`

| Статус | Деталь |
|--------|--------|
| Алиас | `web/svelte.config.js` — `$ui` → `web/src/lib/components/` |
| Guard | `web/scripts/ui-guard.mjs` — `legacyImports` ловит `$lib/components` |
| Импорты | 0 нарушений в `web/src` |

Физическая папка `web/src/lib/components/` **остаётся** — меняется только путь импорта.

### Admin API `/admin/accounts`

Путь API сохранён (не переименовывать). GUI — `/admin/people/{id}`.

---

## Явные legacy-метки в коде

| Файл | Что |
|------|-----|
| `internal/xtime/xtime.go` | `Parse` — legacy RFC3339 (seconds) accepted |
| `internal/jobs/routine.go` | `expireBefore` — legacy RFC3339 без дробных секунд |
| `web/scripts/ui-guard.mjs` | `legacyImports` — запрет `$lib/components` |
| `internal/chronicle/snapshot_circles_test.go` | Сравнение batch vs legacy per-circle unread/last event |

---

## Тесты совместимости

| Файл | Что проверяет |
|------|---------------|
| `internal/jobs/routine_test.go` | `TestRunDailyRoutineExpiresUploadInSameSecond` — expires_at в legacy RFC3339 |
| `internal/store/store_test.go` | Схема v1; nullable `identities.account_id` |
| `internal/api/admin_wave_f_test.go` | Wave F: ttl_sec, sentinel 404, re-register, nullable identities |
| `internal/chronicle/snapshot_circles_test.go` | Batch list ≡ legacy N+1 пути |

---

## Политики «не трогать» (вне scope)

- `identities` с NULL account_id — не чинить частичным индексом
- PostgreSQL, E2E, пагинация ленты, GDPR HTTP, донаты, SPDX — вне очереди
