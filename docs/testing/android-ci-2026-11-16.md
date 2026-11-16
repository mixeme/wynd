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

Реальный новый [прогон GitHub Actions](https://github.com/mixeme/wynd/actions/runs/43911538040)
завершился успешно на отдельной ветке `codex/android-ci-7384cf2e`, SHA
`7384cf2eaff1835d5012e13e10ec109d43a261e9`, событие workflow_dispatch. Задача build
успешна, draft пропущена. Оба Actions artifacts скачаны обратно; JUnit содержит
30 тестов, 0 failures/errors. APK: пакет mixeme.wynd, 0.1.0 / 1000, min/target
21/36. SHA-256 совпала с CI:
`65e579520e7b2107b47756850525b3f04c9a17be68e4dc2ca7ab2445f670d70e`.
Артефакты проверки сохранены в `D:\android-env\ci-artifacts\43911538040`.

Ветка проверки отправлена непосредственно в существующее GitHub-зеркало;
main и теги не изменялись. Gitea в этой сессии отклонила аутентификацию.
Поэтому проверка не объявляется подтверждением доставки изменения в Gitea/main
и не выдаётся за срабатывание нового события push/PR. Это требует следующей
обычной синхронизации основной ветки. PR для проверки не создавался.

Новый тег/релиз не создавался. Реальная tag-ветка нового разделения задач
требует проверки при возобновлении канала обновлений. Прежний успешный прогон
`android-v0.1.0` относится к старому workflow и не подтверждает новую tag-ветку.
