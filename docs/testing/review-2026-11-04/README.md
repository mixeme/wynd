# Протокол ревью 4 ноября 2026

Срез: `a6a1943a3a22d41280d539facf03d31b63022578` плюс начальные рабочие изменения из [worktree-sha256.json](worktree-sha256.json). Хеши фиксируют исходное состояние 27 dirty-файлов; документы плана и ROADMAP после проверки дополнены. Воспроизведение этого состояния требует сохранить соответствующее рабочее дерево: одни хеши не восстанавливают незакоммиченные байты.

Отчёты: [код](../../code-review-2026-11-04.md), [безопасность](../../security-audit-2026-11-04.md), [доработки](../../plans/59-review-followup.plan.md). Windows/amd64, Go 1.27.0, Node 24.20.0. Реальные серверы, SSH и ключ подписи не использовались. Пробники проверяют ожидаемое безопасное поведение и **падают на проверенном дереве**; это свидетельства дефектов, а не регрессии, добавленные в штатный набор.

## Воспроизведения

| ID | Материал | Наблюдение |
|---|---|---|
| SEC-01 | [api-probes.go.txt](api-probes.go.txt), [вывод](probes.log) | Локальный фиктивный сервер получил `/private?/api/v1/move` из пользовательской ссылки |
| SEC-02 | Тот же файл и вывод | Короткая строка blobs в ZIP вызвала `index out of range`; паника перехвачена только пробником |
| CODE-02 | [installer-probes.go.txt](installer-probes.go.txt), [вывод](installer-probes.log) | Подписанный выпуск на второй странице не найден; HTTP полностью подменён |
| CODE-01 | [queue-probe.test.ts.txt](queue-probe.test.ts.txt), [вывод](queue-probes.log) | HEAD исчезнувшей сессии оставляет запись `failed/not_found`, новая сессия не создаётся |

Go-пробники запускаются через overlay без добавления файлов в исходные пакеты. PowerShell, из корня репозитория:

```powershell
$reviewRoot = (Get-Location).Path
$env:GOCACHE = Join-Path $reviewRoot 'dev/go-cache'
$reviewDir = Join-Path $reviewRoot 'docs/testing/review-2026-11-04'
$reviewScratch = Join-Path $reviewRoot 'dev/review-repro'
New-Item -ItemType Directory -Force $reviewScratch | Out-Null
$reviewReplace = @{}
$reviewReplace[(Join-Path $reviewRoot 'internal/api/zz_review_probe_test.go')] = Join-Path $reviewDir 'api-probes.go.txt'
$reviewReplace[(Join-Path $reviewRoot 'internal/installer/zz_review_probe_test.go')] = Join-Path $reviewDir 'installer-probes.go.txt'
$reviewOverlay = Join-Path $reviewScratch 'overlay.json'
@{ Replace = $reviewReplace } | ConvertTo-Json | Set-Content -Encoding utf8 $reviewOverlay
go test -overlay $reviewOverlay ./internal/api ./internal/installer -run '^TestReview' -count=1 -v
```

Для Vitest требуется временное имя `.test.ts` рядом с очередью (относительный импорт `./queue`). Команды не перезаписывают существующий тест:

```powershell
$reviewTarget = Join-Path (Get-Location) 'web/src/lib/queue/review-2026-11-04.test.ts'
if (Test-Path -LiteralPath $reviewTarget) { throw 'Temporary test already exists' }
Copy-Item -LiteralPath 'docs/testing/review-2026-11-04/queue-probe.test.ts.txt' -Destination $reviewTarget
Push-Location web
try { npx vitest run src/lib/queue/review-2026-11-04.test.ts }
finally {
    Pop-Location
    Remove-Item -LiteralPath $reviewTarget
}
```

## Штатные проверки

| Проверка | Результат |
|---|---|
| `go test ./...` с локальным GOCACHE | Все пакеты прошли; [лог](go-test.log) |
| `go test -tags desktop ./cmd/wynd-installer ./cmd/wynd-signer` | Успех; у installer нет тестов; [лог](desktop-tests.log) |
| `go vet ./cmd/... ./internal/...` | Код выхода 0; desktop-tag в этом vet не включён |
| `npx vitest run` в web | 446/447, единственный отказ из-за системного GOCACHE; [лог](web-test.log) |
| Повтор `npx vitest run src/test/licenses.test.ts` с локальным GOCACHE | 2/2; [лог](licenses-retest.log). Полный набор повторно не запускался |
| Web `npm run check`, `npm run check:ui`, `npm run lint`, `npm run build` | Все команды завершились с кодом 0 |
| `npm run check` в каждом из двух desktop frontend | Оба завершились с кодом 0 |

Сборка настольных exe, подписанный выпуск, нагрузка до исчерпания диска/памяти и физические устройства не проверялись. Снимок Vitest, затронутый переводом строк, восстановлен до точного исходного SHA256; продуктовые исходники не менялись.

## Сканеры и инвентаризация

- [govulncheck.txt](govulncheck.txt): `govulncheck ./cmd/... ./internal/...`, свежая база, код выхода 3, 11 записей с найденными вызовами. Без desktop-tag; это анализ локального toolchain, не скан выпущенного бинарника. Первый запуск без сети не учитывался как успешная проверка.
- [npm-audit.json](npm-audit.json): `npm audit --json` в web, код 1, 41 затронутый пакет.
- [npm-audit-prod.json](npm-audit-prod.json): `npm audit --omit=dev --json` в web, код 1, 1 high (devalue). Применимость не определяется одной категорией dependencies.
- [npm-audit-installer.json](npm-audit-installer.json): `npm audit --json` в frontend Мастерской, код 1, 4 пакета (1 moderate, 3 low).
- [npm-audit-signer.json](npm-audit-signer.json): та же команда в frontend «Подписи», код 1, 3 low. Для обоих окон первый запрос был заблокирован сетевой изоляцией, результаты выше — успешные запросы к реестру с advisories, а не сетевые ошибки.
- [routes.json](routes.json): 174 регистрации из `internal/api/server.go`, включая исходные обёртки; не динамическая проверка каждого маршрута.

Подробности условий уязвимостей и применимости — в отчёте безопасности. Сканеры не устанавливают пакеты и не исправляют зависимости. База advisories меняется: повторный запуск позднее может дать иной результат.
