# Выпуск 0.55.0 и клиенты 0.5.3/0.1.6 (2026-11-26)

Владелец 26.11: «Выпускай всё». Утром того же дня он подписал и опубликовал
прошлую волну — Настольный 0.5.2, Мастерскую 0.4.10, Android 0.1.5; после их
тегов накопилось: сервер — план 72 (сессии устройств, подключение по QR,
схема 2), планы 73/74/75, C135–C144. Сервер — **0.55.0** (минор: миграция
схемы и новые возможности), Настольный — **0.5.3**, Android — **0.1.6**.
Мастерская не пересобиралась: правок после 0.4.10 в её путях нет.

## Коммиты и теги

- `b4a95adf` «Настольный 0.5.3: вход по коду подключения на стартовом экране,
  окно без полос» — `internal/desktop/version.go`,
  `cmd/wynd-desktop/build/windows/info.json`, `whatsnew.md`; тег
  **`desktop-v0.5.3`**.
- `d1c39ded` «Android 0.1.6: QR-код подключения на стартовом экране» —
  `android/version.properties` (0.1.6, code 1006), `android/whatsnew.md`; тег
  **`android-v0.1.6`**.
- `b87de6d6` «Версия 0.55.0: сессии устройств, подключение по QR, баннеры и две
  колонки» — VERSION, `internal/version`, `web/package*`, два OpenAPI,
  CHANGELOG, `web/src/lib/news/notes.ts`; тег **`v0.55.0`**.
- Всё в Gitea; зеркало GitHub получило `main` и три тега (`gh api` сверен
  26.11, `main` = `b87de6d6`).

## Ворота

- `scripts\test.bat` на релизном дереве: **`[test] OK`**, код 0 (журнал
  `dev/gates-0550.log`, в git не входит): svelte-check 0/0, check:ui,
  prettier/eslint, **148 файлов / 716 тестов** vitest, сборка web, `go vet`,
  `go test ./...`, сборка сервера, desktop-ворота (check/build трёх frontend,
  размеченные Go-тесты, сборки трёх окон) — `[desktop] OK`.
- GitHub Actions: Server `45841901997` ✓, Image `45841901826` ✓ (gates +
  publish), Desktop checks `45841901362` ✓, Android `45841901548` ✓.
- Тег Android: `45841912460` ✓. Тег Настольного: `45841913411` и
  `45841915526` ✓ — тег сработал дважды, оба прогона зелёные, черновик создан
  один (повторная публикация asset'ов не выполняется).

## Образ и развёртывание (2026-11-26, UTC)

- Образ `ghcr.io/mixeme/wynd:0.55.0` (плюс `latest`), digest
  `sha256:65b4ad7145a12304b9acd98a90f5c27a9eac3f08dc489b0de356413df5f84cab`;
  собран на push `main` с изменением `VERSION`.
- Бэкап на каждом сервере — `data/backups/pre-0.55.0-20261126/` (`wynd.db`,
  `-wal`, `-shm`, `config.json`), `compose.yaml.pre-0.55.0`; заменена только
  строка `image:`.
- **main (wynd.mixfed.ru)**: `docker compose up -d` в 14:53 UTC; контейнер
  `wynd-wynd-1` healthy, журнал `schema version: 2`, `/api/v1/instance` —
  `"version":"0.55.0"`.
- **wynd2 (wynd2.mixfed.ru)**: то же в 14:56 UTC; `wynd-wynd-1` healthy,
  `schema version: 2`, `"version":"0.55.0"`.
- Миграция `0002_sessions_device.sql` применилась на обоих: `{0,1}` → `{0,1,2}`.
  Откат — только восстановлением копии: прежний образ базу схемы 2 не откроет.

## Черновики клиентов (ждут подписи владельца)

- **Настольный 0.5.3** — `desktop-v0.5.3`: `wynd-desktop.exe`,
  `wynd-desktop_0.5.3_amd64.deb`, `Wynd-0.5.3-x86_64.AppImage`, `SHA256SUMS`;
  `.sig` появятся после подписи. «Что нового» — из `cmd/wynd-desktop/whatsnew.md`.
- **Android 0.1.6** — `android-v0.1.6`: `wynd-android-unsigned.apk` и сумма;
  подпись и публикация — окном «Wynd Подпись» (`releaseReady=true`).
- Мастерская 0.4.10 уже опубликована 26.11 07:37 UTC — повторно не собиралась.

## Грабли

- Workflow `image.yaml` срабатывает на push `main` с изменением `VERSION`, а не
  на тег: образ 0.55.0 — прогон `45841901826` ветки `main`.
- Тег Настольного в зеркале запускает два одинаковых прогона `desktop-client`;
  это ожидаемо, оба зелёные.
- В `compose.yaml` менять только строку `image: ghcr.io/mixeme/wynd:<версия>`:
  прежняя замена по «wynd:» задевала имена сервиса и сети (грабля 0.54.1).
- Бэкап базы делать вместе с `-wal` и `-shm`: без них копия неполная.
