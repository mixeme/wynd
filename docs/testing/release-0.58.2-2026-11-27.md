# Выпуск 0.58.2 и Android 0.1.11 (2026-11-27)

Владелец 27.11: «Выпускай накопившиеся правки». После 0.58.1 накопилось:

- **сервер** — клиентская часть C158 («Назад» в APK: страница отвечает
  оболочке, что значит системная кнопка) и C159 (мост «Поделиться»);
- **Android** — C158 и C159 в оболочке: выход по системной «Назад» как на
  платформе, восстановление после гибели рендерера, полный экран видео,
  системный лист «Поделиться», `tel:`/`mailto:`/`sms:`/`geo:`, геолокация
  карты;
- **Настольный** — правок с 0.5.7 нет, не перевыпускается;
- **Мастерская** — правок с 0.4.12 нет, не перевыпускается.

Версии: сервер — **0.58.2** (патч: только исправления, миграций нет),
Android — **0.1.11** (code 1011).

Правки интерфейса в браузере и окне Настольного поведения не меняют: C158 и
C159 работают в паре с оболочкой Android 0.1.11 и старым сборкам молчат.

## Ворота

`scripts\test.bat` на дереве выпуска: **`[test] OK`** — svelte-check 0/0,
`check:ui` (workshop-baseline 4 approved, chrome-guard, viewport-guard,
dialog-guard, check-spacing 19 файлов), prettier + eslint, **154 файла / 756
тестов** vitest, сборка web, `go vet`, `go test ./...`, сборка сервера;
desktop-ворота — **`[desktop] OK`**.

В собранном `web/dist` проверены маркеры C158/C159: `wynd:android-back` — 4
файла, `__wyndShare` — 1 файл (как и ожидалось по коду).

## Находки Android-ворот (исправлены до выпуска)

GitHub-прогон Android от 15:41 на `main` уже был красным; при выпуске тег
упал на тех же местах. Исправлено двумя коммитами:

- **`didCrash()` — API 26 при minSdk 21.** `onRenderProcessGone` приходит
  только с API 26, но lint требует явной проверки вызова
  `RenderProcessGoneDetail.didCrash()`. Теперь
  `if (Build.VERSION.SDK_INT >= 26) crashed = detail.didCrash();`
  (`d1d0503f`).
- **`JavaScriptReplyProxy.postMessage`.** Lint требует
  `WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)` в
  самом методе, а не только при регистрации слушателя; проверка добавлена в
  `ShareBridge.answer` (`e79b3bf1`). Без неё страница получает отказ по
  времени, как и задумано.

Теги `v0.58.2` и `android-v0.1.11` переставлены с `412a33ba` на `d1d0503f` и
затем на `e79b3bf1` **до появления каких-либо ассетов выпуска**: ни один
опубликованный файл не заменялся, черновик Android создан только успешным
прогоном `46043783234`. Красные прогоны для истории: `46042129412` (тег на
`412a33ba`, `didCrash`) и `46042328258` (тег на `d1d0503f`, `postMessage`).

## GitHub Actions

- **Server** `46043778889` — success (коммит `e79b3bf1`).
- **Image** `46042125828` — success, gates 4m35s, publish 3m41s.
- **Android main** `46043778606` — success; **Android tag** `46043783234` —
  success, создан черновик `android-v0.1.11`.
- **Desktop checks** `46042125845` — success (правки `web` и `android`);
  Desktop-тегов нет: Настольный не перевыпускается.

## Образ

`ghcr.io/mixeme/wynd:0.58.2` (+`latest`):

- index `sha256:e585e988ba4583f5cc8c44fa156d5a83eb3cb60abad3fbd41d9a7d22a2acf234`;
- amd64 `sha256:7545346171ced592b6e2dabaf63c225d142663bf4d39c0c60573c8764efa824c`;
- arm64 `sha256:0836864cf84c046f851dea81dbceec8ab4cd04e182633eb28306899d068223cc`.

Образ собран с `412a33ba` — коммита, меняющего `VERSION` (`image.yaml`
срабатывает на `VERSION`). В `d1d0503f` и `e79b3bf1` менялась только
Java-оболочка Android, содержимое сервера (`web/dist`, Go) то же, что в
`e79b3bf1`. Пересборка не делалась, чтобы не оставить в пакете безымянные
слои прежней сборки.

## Развёртывание (2026-11-27, UTC)

- **main (wynd.mixfed.ru)**: бэкап `backups/pre-0.58.2-20261127-173813`
  (`wynd.db`, `-wal`, `-shm`, `config.json`), `compose.yaml.pre-0.58.2`;
  образ 0.58.2, `wynd-wynd-1` healthy, журнал `schema version: 3`,
  публичный `https://wynd.mixfed.ru/api/v1/instance` — `0.58.2`.
- **wynd2 (45.144.64.241)**: бэкап `backups/pre-0.58.2-20261127-173936`,
  `compose.yaml.pre-0.58.2`; образ 0.58.2, healthy, схема 3,
  `https://wynd2.mixfed.ru/api/v1/instance` — `0.58.2`.
- Миграций нет: схема остаётся 3; откат — образом 0.58.1.

## Черновик Android 0.1.11 (ждёт подписи владельца)

- `android-v0.1.11`: `wynd-android-unsigned.apk` 6 104 909 байт, SHA-256
  `7484d3e6631e743d3bafd3ba94bd613d286eacda47a693c19cec902db22fd8b3`;
  файл `.sha256` сходится; `aapt dump badging` — `versionName=0.1.11`,
  `versionCode=1011`, разрешения `ACCESS_COARSE_LOCATION` и
  `ACCESS_FINE_LOCATION` на месте; `gh attestation verify` — rc 0, источник —
  тег `android-v0.1.11`.
- **Старый черновик `android-v0.1.10` оставлен как есть:** в нём уже было
  скачивание владельцем. Он полностью заменён 0.1.11 (тот содержит C155–C157
  и C158/C159) — подписывать и публиковать нужно **0.1.11**, не 0.1.10.

## Коммиты и теги

- `412a33ba` «Версия 0.58.2 и Android 0.1.11: «Назад» в приложении, полный
  экран, «Поделиться» и место на карте» — VERSION, `internal/version`,
  `web/package*`, два OpenAPI, CHANGELOG, `whatsnew.md` Настольного и Android,
  `internal/desktop/version.go`, два `info.json`, `android/version.properties`.
- `d1d0503f` «C159: didCrash под проверкой версии — Android-ворота снова
  зелёные».
- `e79b3bf1` «C159: postMessage только при поддержке
  WEB_MESSAGE_LISTENER — Android-ворота зелёные».
- Теги **`v0.58.2`**, **`android-v0.1.11`** — на `e79b3bf1`; отправлены в
  Gitea, зеркало GitHub получило `main` и теги.
