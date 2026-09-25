# Wynd

Self-hosted журнал кругов: один бинарник Go, SQLite, SvelteKit SPA внутри `go:embed`.
Для домашнего инстанса на несколько человек, не для SaaS.

**Версия:** файл [`VERSION`](VERSION); что менялось — [CHANGELOG.md](CHANGELOG.md).

**Лицензия:** [GNU AGPL v3](LICENSE). Исходный код: <https://github.com/mixeme/wynd>.

---

## Требования

**Чтобы запустить:** только бинарник — он статический (`CGO_ENABLED=0`), клиент вшит внутрь. Linux с systemd для `deploy/install.sh` или Docker для `deploy/docker/`.

**Чтобы инстанс видели снаружи:**

- домен и обратный прокси с TLS перед `127.0.0.1:7676` — Caddy ставит `install.sh`, шаблоны nginx и Traefik лежат в `deploy/proxy/`; таймаут чтения прокси не меньше 300 с (SSE и длинные загрузки);
- SMTP-релей: участники входят по коду из письма. Без релея вход работает только на loopback — код пишется в журнал сервера и в `<каталог данных>/dev-auth-codes.log`.

**Чтобы собрать из исходников:** Go 1.26 (`go.mod`), Node 22 и npm (та же версия, что в `deploy/docker/Dockerfile`). Для `scripts\test-integration.bat` — учётки SMTP-релеев в `dev/`.

---

## Установка

Linux с systemd — из корня репозитория:

```bash
sudo ./deploy/install.sh --from-source --public-url https://home.example.org
```

Скрипт заводит пользователя `wynd`, каталог данных `/var/lib/wynd`, юнит `wynd.service`, при публичном адресе — Caddy (`--own-proxy` — не трогать прокси, напечатать фрагмент конфигурации). В конце печатает адрес первого запуска с токеном: там задаются пароль панели, имя инстанса и SMTP.

Docker — `deploy/docker/compose.yaml` (Wynd и Caddy, данные в томе `wynd-data`); адрес — в `WYND_PUBLIC_URL`, домен — в `deploy/caddy/Caddyfile`.

---

## Конфигурация

Три слоя, каждый следующий перекрывает предыдущий: умолчания → `config.json` в каталоге данных → переменные окружения. Большая часть настроек (имя, режим регистрации, SMTP, квоты, сжатие, оплата) живёт не здесь, а в базе и меняется в панели `/admin`.

| Переменная | Поле `config.json` | Умолчание | Что задаёт |
|---|---|---|---|
| `WYND_DATA_DIR` | — | `dev/data` (от текущего каталога) | Каталог данных. `install.sh` — `/var/lib/wynd`, образ Docker — `/data` |
| `WYND_LISTEN` | `listen` | `127.0.0.1:7676` | Адрес, который слушает сервер. В Docker — `:7676` |
| `WYND_PUBLIC_URL` | `public_url` | `http://127.0.0.1:7676` | Адрес, по которому инстанс видят участники: ссылки в письмах, приглашения, проверки |
| `WYND_TRUSTED_PROXIES` | `trusted_proxies` | пусто = только loopback | Через запятую — адреса или CIDR, чьему `X-Forwarded-For` верить (лимиты по адресу) |

- `public_url` — только `http(s)://хост[:порт]`: без пути, запроса, якоря и учётных данных. С неверным адресом сервер не стартует и пишет почему.
- Адрес на `127.0.0.1`, `localhost` или `::1` — профиль loopback: установка и вход без SMTP, проверки прокси и HTTPS «не применимо».
- `config.json` появляется, только когда его сохраняют: `install.sh`, первый запуск, смена адреса в панели. Если `WYND_PUBLIC_URL` перекрывает сохранённый адрес, сервер пишет это в журнал.
- `WYND_CREDENTIALS_DIR` и `WYND_CREDENTIALS_FILE` читают только интеграционные тесты почты.

---

## Каталог данных

| Путь | Что | Права |
|---|---|---|
| `wynd.db` (+ `-wal`, `-shm`) | Вся база: учётки, круги, хроника, настройки. **Содержит пароль SMTP и приватный ключ VAPID открытым текстом** | `0600`, выравнивается при старте |
| `config.json` | Адрес, порт, доверенные прокси | `0640` |
| `blobs/<2 знака>/<uuid>` | Файлы вложений, аватаров, скриншотов оплаты — ровно те байты, что прислал клиент | |
| `blobs/.uploads/` | Незавершённые загрузки; в бэкап не входят | |
| `tmp/` | Архивы кругов на время скачивания; очищается при старте, в бэкап не входит | `0700` |
| `keys/bootstrap` | Токен первого запуска | `0600` |
| `backups/pre-update-*` | Копии, которые `install.sh` снимает перед заменой бинарника | |
| `dev-auth-codes.log` | Коды входа — только на loopback | |

Один процесс на каталог: второй сервер на том же `wynd.db` не поддерживается.

---

## Бэкап

```bash
sudo -u wynd env WYND_DATA_DIR=/var/lib/wynd wynd backup /var/backups/wynd/2026-09-23
sudo -u wynd env WYND_DATA_DIR=/var/lib/wynd wynd backup -incremental /var/backups/wynd/current
```

- Сервер останавливать не нужно: база снимается `VACUUM INTO` — целостная копия без `-wal`/`-shm`.
- В каталог назначения ложатся `wynd.db`, `config.json` (если есть), `keys/` и `blobs/` с `blobs/manifest.json`.
- `-incremental` в тот же каталог копирует только файлы, которых нет в `manifest.json` или чья копия отсутствует либо короче оригинала; остальные не перечитываются. База, конфиг и ключи переписываются всегда. Файлы, удалённые с инстанса после прошлого прогона, в каталоге бэкапа остаются. Такой каталог — рабочая копия, а не единственная: держите рядом хотя бы одну датированную.
- Запускать от пользователя сервиса: команда отмечает время бэкапа в самой базе (его показывает панель) и должна читать файлы `0600`. Каталог бэкапа создаётся `0700`, копия базы — `0600`. В нём те же секреты, что в `wynd.db`, — в общедоступное место не класть. Сам каталог данных и каталоги внутри `blobs/` и `keys/` команда как цель не примет; подкаталог рядом, например `/var/lib/wynd/backups/<дата>`, можно.
- В Docker: `docker compose exec wynd wynd backup /data/backups/<дата>`, затем `docker compose cp wynd:/data/backups/<дата> ./<дата>`.

---

## Пароль панели

Забытый пароль панели меняется на хосте — старый не нужен, сервер останавливать не надо. Новый пароль команда читает первой строкой стандартного ввода; открытые сессии панели закрываются.

```bash
read -rs P && printf '%s\n' "$P" | sudo -u wynd env WYND_DATA_DIR=/var/lib/wynd wynd admin-password; unset P
```

`read -rs` не показывает пароль на экране; если ввести его прямо в команду, он будет виден. Не короче 8 знаков. В Docker: `docker compose exec -T wynd wynd admin-password`, пароль — так же через трубу.

---

## Восстановление

Из каталога, снятого `wynd backup`, в чистый каталог данных. Процедура прогнана на бинарнике 0.8.0 (Windows, без systemd — шаги те же, кроме прав): полный бэкап на работающем сервере, три записи после него, восстановление в чистый каталог, лента без этих записей, сессия участника, новая запись дошла клиенту, который помнил номера событий новее копии. На 0.7.3 ещё проверены инкрементальный бэкап и файлы побайтно.

1. Остановить сервер: `sudo systemctl stop wynd`.
2. Отложить текущий каталог, не удалять: `sudo mv /var/lib/wynd /var/lib/wynd.broken`.
3. Разложить копию:

   ```bash
   sudo install -d -o wynd -g wynd -m 0750 /var/lib/wynd
   sudo cp -a /var/backups/wynd/2026-09-23/. /var/lib/wynd/
   sudo rm -f /var/lib/wynd/blobs/manifest.json
   sudo chown -R wynd:wynd /var/lib/wynd
   sudo chmod 0600 /var/lib/wynd/wynd.db
   ```

   Если в копии нет `config.json` — адрес задавался только окружением, так и оставить.
4. Запустить: `sudo systemctl start wynd`. В журнале — `schema version: N`; `curl http://127.0.0.1:7676/ready` отвечает `ready`.
5. Проверить в клиенте ленту круга и пару вложений.

Что теряется: всё, что появилось после бэкапа. Сессии, выданные после него, недействительны — участник входит заново. Клиенты, помнящие номера событий из прежней жизни базы, при первом подключении узнают от сервера последний номер и сами сбрасывают курсор и снимки — счётчик событий трогать не нужно. Копию, снятую **более новой** версией, старый бинарник не откроет («устаревшая схема»): восстанавливать тем же или более новым бинарником. Каталог `wynd.broken` удалить, когда восстановленный инстанс проверен.

---

## Обновление

1. Прочитать в [CHANGELOG.md](CHANGELOG.md) блоки «Оператору» всех версий между текущей и новой: миграции, новые поля конфигурации, требования к прокси.
2. systemd — тот же `install.sh` поверх установленного:

   ```bash
   git pull
   sudo ./deploy/install.sh --from-source
   ```

   Перед заменой бинарника скрипт снимает бэкап **старым** бинарником в `/var/lib/wynd/backups/pre-update-<время>/` и кладёт туда же прежний `wynd`; не удался бэкап — обновление не выполняется. `config.json` не переписывается.
3. Docker: `git pull`, затем `docker compose build` и `docker compose up -d` в `deploy/docker/`. Бэкап перед этим — вручную (см. выше).
4. Миграции схемы применяются сами при старте. В журнале — новая `schema version`.

Отката после миграции нет: схема только растёт. Вернуться на прежнюю версию — это восстановление из `pre-update-*` прежним бинарником, который лежит там же: вернуть его в `/usr/local/bin/wynd` и пройти «Восстановление». После шага 2 копия окажется в `/var/lib/wynd.broken/backups/`; файл `wynd` из неё в новый каталог данных не нужен.

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

Ворота релиза — из корня репозитория, на Windows `scripts\test.bat`, на Unix:

```bash
make test
```

Это `go vet`, `go test ./...`, `go build ./cmd/wynd` и в `web/` — `npm run check`, `check:ui`, `test`, `build`; то же гоняет CI в Gitea (`.gitea/workflows/ci.yaml`, плюс `docker build`). Правила — [CONTRIBUTING.md](CONTRIBUTING.md).

SMTP на реальных релеях (`dev/credentials.txt` или JSON в `dev/credentials/`, образцы в `internal/mail/testdata/`):

```bash
scripts\test-integration.bat
```

### Инварианты

Всё запускается одной командой — `go test ./...` и `npm run test` в `web/`; отдельной таблицы областей здесь нет: она устаревала быстрее, чем обновлялась. Перечень инвариантов — в [server-reference.md](docs/reference/server-reference.md) (хроника, панель, архив); у каждого теста в `internal/chronicle/invariants_test.go` и `archive_test.go` над ним строка с тем, что он держит.

---

## Документация

| Что | Где |
|-----|-----|
| Образ продукта | [docs/wynd.html](docs/wynd.html) |
| Сервер | [docs/reference/server-reference.md](docs/reference/server-reference.md) |
| Клиент и UI | [docs/reference/client-reference.md](docs/reference/client-reference.md), [ui-components.md](docs/reference/ui-components.md) |
| Экраны (макеты) | [docs/visual/screens.html](docs/visual/screens.html) |
| Стек (история решений) | [docs/stack.html](docs/stack.html) |
| Аудиты безопасности | [2026-09-22](docs/security-audit-2026-09-22.md) (срез 0.7.1), [2026-09-03](docs/security-audit-2026-09-03.md) (срез 0.1.14); триггеры повторного прохода — [CONTRIBUTING.md](CONTRIBUTING.md) |
| Изменения | [CHANGELOG.md](CHANGELOG.md) (текущая ветка; старшие — в истории git) |
| Как вносить изменения, глоссарий | [CONTRIBUTING.md](CONTRIBUTING.md) |

Шрифт интерфейса — Golos Text ([SIL OFL](web/static/fonts/OFL.txt)).
Версия продукта — файл [`VERSION`](VERSION); где ещё живёт номер и как его менять — [CONTRIBUTING.md](CONTRIBUTING.md#changelog-и-version).

---

## Сборка

```bash
cd web && npm run build
go build -o dist/wynd ./cmd/wynd
```

Шаблоны деплоя — `deploy/` (Docker, systemd, Caddy/nginx/Traefik).
