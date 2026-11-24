# Android 0.1.2: выпуск — 24 ноября 2026

Выпуск обычной редакции `standard` (самообновление из GitHub-выпусков);
редакция `fdroid` в этот выпуск не входит — f-droid.org собирает её сам
(план [67](../plans/67-fdroid-android.plan.md)). Состав — этап 5 плана 50:
нативная очередь записей, комментариев и реакций; вложения с докачкой и SHA;
фоновое сжатие видео Media3; постоянный `.wynd`; native-кадр ролика.
Серверная часть кадра и ffmpeg удалены в серверном 0.53.7.

## Версия и ворота

- `android/version.properties`: `versionName=0.1.2`, `releaseReady=true`.
- `android/whatsnew.md`: раздел `## 0.1.2` — источник заметок выпуска для
  подписи.
- Локально: `testStandardDebugUnitTest testFdroidDebugUnitTest
  lintStandardRelease assembleStandardRelease` — `BUILD SUCCESSFUL in 3m 19s`.
  `aapt`: `mixeme.wynd`, `versionCode=1002`, `versionName=0.1.2`, без
  debug-флага; `zipalign -c 4` — выровнен. Локальный unsigned APK:
  SHA-256 `93ac2c840451bbbef1d954b3e18ce9f40d49988e92a3c41fcd93f960978ecf55`
  (в выпуск не входит).
- Серверные ворота — [0.53.7](release-0.53.7-2026-11-24.md).

## Тег и CI

- Релизный коммит `bfa28d4a`, тег `android-v0.1.2` сначала на нём; после
  неудачи сторожа вшитого интерфейса добавлен коммит `2af8dccb` и тег
  переставлен на него **до подписи и публикации** (история не переписывалась,
  оба коммита в `main`).
- Первый прогон по тегу (`45354110962`, `bfa28d4a`): тесты, lint и
  release-APK прошли, **Embedded interface build упал** —
  `validateSigningStandardDebug`: на чистой машине нет
  `keys/wynd-debug.keystore`. Исправлено в `.github/workflows/android.yaml`
  (`2af8dccb`): workflow создаёт стандартный `androiddebugkey` во временном
  каталоге окружения; ключ владельца не участвует.
- Повторный прогон по тегу (`45355034190`, `2af8dccb`) — **success**:
  обе редакции, lint, release-APK и сборка вшитого интерфейса; задача `draft`
  создала черновик.
- Черновик `android-v0.1.2` (`isDraft=true`), assets:
  `wynd-android-unsigned.apk` 6 079 265 байт,
  SHA-256 `859fd9fdf08a1eb438d35c72a01b1778681698649ba02ba86d79f1b8e91d8cdf`,
  и `.sha256`.

## Проверка черновика (вне подписи)

- Скачан заново: SHA-256 совпадает с asset `.sha256`.
- `aapt`: `mixeme.wynd`, `versionName=0.1.2`, `versionCode=1002`, без
  debug-флага.
- `zipalign -c 4` — выровнен.
- Запись удостоверения сборки для этого SHA-256 есть
  (`/repos/mixeme/wynd/attestations/sha256:859fd9fd…`, 1 шт.).

## Не выполнено здесь и почему

- **Подпись постоянным ключом и публикация** — шаг владельца в окне
  «Wynd Подпись» (план 65; пароль ключа локальный). `releaseReady=true`,
  поэтому публикация разблокирована. В описании выпуска окно добавляет
  строку, какой файл ставить: `wynd-android.apk`;
  `wynd-android-unsigned.apk` — исходный артефакт для проверки удостоверения.
- **Самообновление** на 0.1.2 у пользователей 0.1.x — не раньше 72 ч после
  публикации.

## Остатки

1. Владелец: подписать `wynd-android.apk` и опубликовать черновик.
2. Установить на OPPO и повторить UI-путь этапа 5 (запись, файл, офлайн-видео,
   `.wynd`, кадр ролика) — реестр §11.
3. Проверить WorkManager после гибели процесса без открытия приложения,
   Android 13+, энергосбережение производителей; длинные загрузки —
   foreground-режим (пока не включён).
4. Полный `npm run lint` и `first-frame.svelte.test.ts` — в общих воротах
   зелёные на этом дереве.
