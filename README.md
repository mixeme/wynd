# Wynd

Self-hosted журнал кругов: один бинарник Go, SQLite, SvelteKit SPA внутри `go:embed`.
Для домашнего инстанса на несколько человек, не для SaaS.

**Версия:** см. файл [`VERSION`](VERSION) (сейчас 0.3.5).

**Лицензия:** [GNU AGPL v3](LICENSE). Исходный код: <https://github.com/mixeme/wynd>.

---

## Быстрый старт (loopback)

Каталог данных по умолчанию — `dev/data` (создаётся при первом запуске).

```bash
go run ./cmd/wynd
```

Клиент в dev-режиме (Vite :5173, proxy `/api` → :7676):

```bash
cd web && npm install && npm run dev
```

На Windows можно `scripts\run.bat` — собирает бинарник при необходимости и открывает браузер.

Проверка, что сервер жив:

```bash
curl http://127.0.0.1:7676/health
```

На loopback bootstrap и код входа работают без SMTP (код в логе сервера). Подробнее — [server-reference.md](docs/reference/server-reference.md), раздел «Локальный прогон».

---

## Тесты

Из корня репозитория:

```bash
scripts\test.bat
```

Или вручную:

```bash
go test ./...
cd web && npm run check && npm run check:ui && npm run test
```

### Инварианты и тесты

| Область | Где смотреть |
|---------|----------------|
| Хроника (спаны, снимки, журнал) | `internal/chronicle/*_test.go` |
| Панель F (квота, учётки, SMTP-хвосты) | `internal/api/admin_wave_f_test.go` |
| Архив (ZIP, HTML без внешних ссылок) | `internal/archive/archive_test.go`, `internal/chronicle/archive_test.go` |
| Security HTTP (push, заголовки, границы) | `internal/api/security_test.go`, `internal/auth/*_test.go` |
| Клиент: present, sync SSE, очередь | `web/src/lib/journal/present.test.ts`, `web/src/lib/sync/sync.test.ts`, `web/src/lib/queue/queue.test.ts` |

---

## Документация

| Что | Где |
|-----|-----|
| Образ продукта | [docs/wynd.html](docs/wynd.html) |
| Сервер | [docs/reference/server-reference.md](docs/reference/server-reference.md) |
| Клиент и UI | [docs/reference/client-reference.md](docs/reference/client-reference.md), [ui-components.md](docs/reference/ui-components.md) |
| Экраны (макеты) | [docs/visual/screens.html](docs/visual/screens.html) |
| Стек (история решений) | [docs/stack.html](docs/stack.html) |
| Изменения | [CHANGELOG.md](CHANGELOG.md) |

Шрифт интерфейса — Golos Text ([SIL OFL](web/static/fonts/OFL.txt)).
Версия продукта — файл [`VERSION`](VERSION). То же значение — `internal/version.Number`, OpenAPI `info.version` и `version` в `web/package.json` (пакет `private`, в npm не публикуется). Сторож: `TestMatchesVERSIONFile`.

---

## Сборка

```bash
cd web && npm run build
go build -o dist/wynd ./cmd/wynd
```

Шаблоны деплоя — `deploy/` (Docker, systemd, Caddy/nginx/Traefik).
