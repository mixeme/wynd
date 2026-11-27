# Выпуск 0.56.0 и клиенты 0.5.4/0.4.11/0.1.7 (2026-11-27)

Владелец 27.11: «Выпускай всё». После прошлой волны (сервер 0.55.0,
Настольный 0.5.3, Мастерская 0.4.10, Android 0.1.6 — 26.11) накопилось:

- **сервер** — план 76 (широкие экраны по разделам), принятая страница
  `/app` (план 73), C147 (вход в окне Настольного), C150 (окно не падает при
  отменённом ответе); серверного кода и миграций нет — только клиент в
  образе;
- **Настольный** — C149, C150, встроенный клиент из `main`;
- **Мастерская** — C146 (колонтитул одной высоты); после 0.4.10 других правок
  в её путях нет;
- **Android** — встроенный клиент из `main` (широкие экраны, `/app`).

Версии: сервер — **0.56.0** (минор: заметные экраны), Настольный — **0.5.4**,
Мастерская — **0.4.11**, Android — **0.1.7** (code 1007).

**План 76 и выписка.** По замечанию владельца 27.11 («широкие экраны не в
этом выписке») широкие экраны **не входят в список изменений** этого выпуска:
их нет в CHANGELOG, «Что нового» и news. Вёрстка остаётся в сборке — возврат к
отменённому рейлу не планируется; отдельная выписка — после приёмки оставшихся
кадров (приняты 3 из 7; параллельно идёт доработка значков меню настроек
круга) ([план 76](../plans/76-wide-sections.plan.md)).

## Коммиты и теги

- `be47dc66` «web: форматирование SettingsRow после плана 76» — prettier в
  release-воротах (файл пришёл из плана 76 неотформатированным).
- `28849a27` «Настольный 0.5.4…» — `internal/desktop/version.go`,
  `cmd/wynd-desktop/build/windows/info.json`, `whatsnew.md`; тег
  **`desktop-v0.5.4`**.
- `5e42e783` «Мастерская 0.4.11…» — `internal/workshop/version.go`,
  `cmd/wynd-workshop/build/windows/info.json`, `whatsnew.md`; тег
  **`workshop-v0.4.11`**.
- `cdc660f1` «Android 0.1.7…» — `android/version.properties` (0.1.7, code
  1007), `android/whatsnew.md`; тег **`android-v0.1.7`**.
- `a0031b8e` «Версия 0.56.0…» — VERSION, `internal/version`,
  `web/package*`, два OpenAPI, `CHANGELOG.md`, `web/src/lib/news/notes.ts`;
  тег **`v0.56.0`**.

Всё в Gitea; зеркало GitHub получило `main` и теги.

## Ворота

- `scripts\test.bat` на релизном дереве: **`[test] OK`** — svelte-check 0/0,
  check:ui (в т.ч. 0 layouts без приёмки), prettier/eslint, **151 файл /
  724 теста** vitest, сборка web, `go vet`, `go test ./...`, сборка сервера,
  desktop-ворота — `[desktop] OK`.
- `scripts\test-desktop.bat`: **`[desktop] OK`** — три frontend (проверка,
  тесты, сборка), размеченные Go-тесты, сборки трёх окон.
- Android: `testStandardDebugUnitTest testFdroidDebugUnitTest
  lintStandardRelease lintFdroidRelease assembleStandardRelease
  assembleFdroidRelease` — **BUILD SUCCESSFUL** (2m25s); release-APK standard
  6 102 353 байта.
- GitHub Actions на `a0031b8e`: Server `45947604083` ✓, Android
  `45947603865` ✓, Desktop checks `45947603870` ✓, Image `45947603922` ✓.

## Образ и развёртывание (2026-11-27, UTC)

- Образ `ghcr.io/mixeme/wynd:0.56.0` (плюс `latest`), digest
  `sha256:07c3dd8acc5fbb198bdc2e06c0a2fa622470922195fa4314d364d146d40f68a6`;
  архитектуры: amd64 `sha256:78afcc10…`, arm64 `sha256:8f1af234…`.
- **wynd2 (wynd2.mixfed.ru)**: бэкап `data/backups/pre-0.56.0-20261127/`
  (`wynd.db`, `-wal`, `-shm`, `config.json`), `compose.yaml.pre-0.56.0`;
  заменена строка `image:`; `docker compose pull && up -d` в 05:56 UTC;
  `wynd-wynd-1` healthy, журнал `schema version: 2`, `/api/v1/instance` —
  `"version":"0.56.0"`.
- **main (wynd.mixfed.ru)**: то же в 05:56:53 UTC; healthy, схема 2,
  `"version":"0.56.0"`.
- Миграций нет: схема базы остаётся 2; откат — заменой образа и
  восстановлением копии при необходимости.

## Черновики клиентов (ждут подписи владельца)

- **Настольный 0.5.4** — `desktop-v0.5.4`, прогон `45948606493`:
  `wynd-desktop.exe` 17 946 624 байт, SHA-256 `7959f3b6…`; `.deb`
  5 493 754 байта и AppImage 7 023 096 байт — по `SHA256SUMS`, сходятся;
  `VersionInfo` exe — 0.5.4; удостоверения сборки проверены (`gh attestation
  verify`, exe/.deb/AppImage — rc 0).
- **Мастерская 0.4.11** — `workshop-v0.4.11`, прогон `45948609546`:
  `wynd-workshop.exe` 18 322 944 байта, SHA-256 `bba5215e…`; `VersionInfo` —
  0.4.11; удостоверение проверено.
- **Android 0.1.7** — `android-v0.1.7`, прогон `45948612721`:
  `wynd-android-unsigned.apk` 6 102 305 байт, SHA-256 `9cdee682…`;
  `aapt`: `mixeme.wynd`, 0.1.7/1007; удостоверение проверено.
- Подпись и публикация — владельцем в окне «Wynd Подпись»; «Что нового» —
  из `whatsnew.md` соответствующего продукта. После публикации: Android 0.1.7
  на планшет/OPPO, Настольный 0.5.4 — вход на wynd2 (сервер 0.56.0 уже с
  C147) и проверка широких экранов в окне 1060.

## Грабли

- **Теги из зеркала не запускают workflow.** Gitea-зеркало создало теги на
  GitHub, но push-событий для них не было: прогонов по тегам не появилось.
  Теги пересозданы обычным `git push` на GitHub (те же коммиты) — событий
  тоже не дало; выпускные сборки запущены вручную:
  `gh workflow run <workflow> --ref <тег>`. В workflow условия черновика —
  `startsWith(github.ref, 'refs/tags/…')`, поэтому запуск на ref тега даёт
  тот же результат, что и тег. На будущее — проверять появление прогонов по
  тегу, иначе черновик не создаётся.
- **`str.replace(old, new, 0)` в Python ничего не заменяет** — при подъёме
  версий это молча оставило VERSION/Go-константы прежними; сверять каждую
  версию после правки.
- **Тест «Что нового» сверяет запись с `web/package.json`** — версию пакета
  поднимать вместе с VERSION, иначе `notes.test.ts` падает.
- **OpenAPI-файлы содержат `info.version`** — `internal/version` требует
  равенства VERSION; обновлять оба `openapi-*.yaml`.
- В `web/node_modules` после удаления worktree остался пустой каталог —
  перед воротами нужен `npm ci`.
