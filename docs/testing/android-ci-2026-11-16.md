# Android CI для main и PR — 2026-11-16

Прежний `.github/workflows/android.yaml` запускался вручную и по Android-тегу.
Добавлены push в main и pull request с фильтром по `android/**`, двум исходникам
значков, `scripts/build-android.ps1` и самому workflow. Веб-клиент проверяется
существующими web/server-воротами; в APK он не вшивается.

Сборка выполняет `testDebugUnitTest lintDebug lintRelease assembleRelease`.
Actions artifact `wynd-android-unsigned` содержит APK и SHA-256, хранится 14 дней;
`wynd-android-reports` — отчёты тестов/lint на 7 дней, шаг выполняется и при ошибке.
При повторном запуске заменяются только Actions artifacts того же прогона.

Обычная сборочная задача наследует `contents: read`; секретов подписи нет.
Отдельная задача `draft` зависит от успешной сборки и запускается только для
`refs/tags/android-v*`. Только она получает contents/id-token/attestations write.
Она скачивает APK из того же workflow run, сверяет SHA-256, создаёт attestation
и прежний черновик релиза. Существующий релиз по-прежнему не заменяется.
Фильтр tag/version сохранён. Публичной публикации и подписи здесь нет.
Concurrency отменяет устаревшие проверки ветки/PR, Android-теги не отменяет.

Артефакты реализованы по официальным контрактам
[upload-artifact](https://github.com/actions/upload-artifact) и
[download-artifact](https://github.com/actions/download-artifact).
Версии actions согласованы с текущими README: v7/v8.

Локальная проверка: actionlint 1.7.12 (без доступных на Windows shellcheck/
pyflakes), разбор YAML и проверка границ прав build/draft. Инструмент и Go-кэши
находятся в `D:\android-env`, зависимости приложения не менялись.
Точный набор Gradle прошёл локально: 30 JVM-тестов, lintDebug/lintRelease и
assembleRelease, unsigned APK собран. Это Windows-проверка тех же задач;
она не выдаётся за новый прогон GitHub Actions на Linux.

Новый тег/релиз не создавался. Реальная tag-ветка нового разделения задач
требует проверки при возобновлении канала обновлений. Прежний успешный прогон
`android-v0.1.0` относится к старому workflow и не подтверждает новую tag-ветку.
