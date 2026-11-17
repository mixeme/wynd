# План 64. Миграция окон на Wails v3 (C94) — подготовка

**Создан:** 2026-11-17.
**Статус:** подготовка выполнена, миграция не начата. Старт — по решению владельца.
**Основание:** C94 (план 46) — v3 вышла из беты (v3.0.0 — 26.10.2026, v3.1.0 — Latest); повод — потребности C66 (трей, системные уведомления, работа при закрытом окне).

## 1. Инструменты — подготовлены и проверены 17.11

| Инструмент | Результат |
|---|---|
| CLI `wails3` **v3.1.0** | установлен `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0` → `%USERPROFILE%\go\bin`; `wails3 version` → v3.1.0 |
| `wails3 doctor` | «No issues found / Your system is ready»; WebView2 157.0.4341.46; Go 1.27.3; npm 11.19.0 |
| Проба сборки | `dev/wails3-probe` (vanilla + TypeScript, вне git): `wails3 init -n wails3-probe -t vanilla` + `wails3 build` — полный цикл (npm → syso → `go build -tags production -ldflags="-H windowsgui"`) прошёл, `bin/wails3-probe.exe` собран. Живой запуск окна с пробы не делался |
| Генерация биндингов | `wails3 generate bindings` — работает, создаёт `frontend/bindings/` |
| Требования v3 | Go ≥ 1.26 (документация) — у нас 1.27.3; мобильное направление и `wails3 setup` — экспериментальные, не трогаем |

Необязательные зависимости, которых нет (для настольной сборки не нужны): NSIS, SignTool/MSIX, Android SDK, docker.

## 2. Инвентаризация Wails v2 в репо

Корневой `go.mod` — `github.com/wailsapp/wails/v2 v2.16.0`, один модуль на все окна:

| Место | Что использует |
|---|---|
| `cmd/wynd-desktop/main.go` | `wails.Run(options.App)`: Title/Width/Height/MinWidth/MinHeight, AssetServer со своим `Handler` (proxy + FileServer), Frameless, BackgroundColour, `windows.Options.WebviewUserDataPath`, `Bind`; `runtime.Quit`, `runtime.BrowserOpenURL`; собственный локальный порт apiBase для SSE — к Wails не относится |
| `cmd/wynd-signer/main.go` | `wails.Run`, AssetServer; `runtime.BrowserOpenURL`, `runtime.ClipboardSetText`, диалоги (`OpenFileDialog`, `SaveFileDialog`, `OpenDialogOptions`, `FileFilter`, `SaveDialogOptions`), `runtime.Quit` |
| `cmd/wynd-workshop/main.go`, `archives.go`, `overrides_dev.go` | `wails.Run`, AssetServer; диалоги с фильтрами, `OpenDirectoryDialog`, `OpenMultipleFilesDialog`, `runtime.EventsEmit`, `BrowserOpenURL`, `Quit` |
| Мост во всех трёх окнах | `window.go.main.App.*` (`api.ts` каждого окна); в тесте Мастерской `profiles.test.mjs` — `dom.window.go = { main: { App } }` |

## 3. Сверка API — все нужные механизмы в v3.1.0 есть

Проверено по исходникам пакета (`~/go/pkg/mod/github.com/wailsapp/wails/v3@v3.1.0/pkg/application`):

| v2 | v3.1.0 |
|---|---|
| `AssetServer: &assetserver.Options{Handler: …}` | `AssetOptions.Handler http.Handler` (+ Middleware) — свой proxy остаётся |
| `Frameless`, `BackgroundColour RGBA`, `MinWidth/MinHeight` | те же поля в `WebviewWindowOptions` |
| `windows.Options.WebviewUserDataPath` | `WindowsOptions` — перенесены на уровень приложения (`application.Options.Windows`); там же `DisableQuitOnLastWindowClosed` (пригодится для C66) |
| Origin окна на Windows | по-прежнему `http://wails.localhost/` — `windowOrigin()` и CORS локального порта не меняются |
| `runtime.Quit(ctx)` | `app.Quit()` |
| `runtime.BrowserOpenURL(ctx, …)` | `app.Browser.OpenURL(url)` |
| `runtime.ClipboardSetText(ctx, …)` | `app.Clipboard.SetText(text)` |
| `runtime.EventsEmit(ctx, …)` | `app.Event.Emit(…)` |
| Диалоги + `OpenDialogOptions{Filters: FileFilter}` | `app.Dialog.OpenFile()/SaveFile()` → `OpenFileDialogOptions{Filters []FileFilter, CanChooseDirectories}` / `SaveFileDialogOptions{Filters}`; выбор — `PromptForSingleSelection()` и др. |
| `Bind: []any{app}`, поле `ctx`, `startup(ctx)` | `Services: application.NewService(&App{})`, хук `ServiceStartup(ctx, options)`; поле `ctx` не нужно |
| `window.go.main.App` в JS | сгенерированные биндинги в `frontend/bindings/`, события — пакет `@wailsio/runtime` |
| — (нет в v2) | трей: `app.SystemTray.New()` — есть в v3.1.0. Системных уведомлений в списке исходников v3.1.0 не видно — проверить отдельно, это открытый вопрос для C66 |

Мотивация C66 в v3.1.0: трей — есть; `DisableQuitOnLastWindowClosed` — есть; системные уведомления — не подтверждены, проверить до старта миграции Настольного.

## 4. Предлагаемые этапы (не начаты)

1. **Ветка + модуль.** Ветка `wails-v3`; в корневой `go.mod` добавляется `github.com/wailsapp/wails/v3` — рядом с v2, модули разные, окна переводятся по одному; ворота v2 не трогает.
2. **Пилот — Подпись.** Самое простое окно: перевод `main.go` на `application.New` + сервис, сгенерированные биндинги в `api.ts`, `wails3 build`, `test-desktop.bat`, живая проверка окна, сборка через `build-signer.bat`. Конвенции по сборке — как в «Пересборка exe после правок Go».
3. **Настольный (сам C94).** Перенос параметров окна, свой `Handler` в `AssetOptions`, `WindowsOptions` на уровень приложения, биндинги в `frontend/src/api.ts`; SSE/apiBase не меняются. Живая проверка: те же 760×780/мин 500×480, своя рамка, SSE течёт, CORS отвечает, самообновление работает.
4. **Мастерская.** Диалоги с фильтрами, `EventsEmit` → сервис, `overrides_dev.go`, `profiles.test.mjs` (подмена `window.go` → подмена сгенерированного биндинга).
5. **Выпуск и чистка.** После перевода всех окон — убрать v2 из `go.mod`; сборочные скрипты (`build-*.bat`) и CI-ворота обновляются по ходу этапов.

Порядок (пилот Подпись → Настольный → Мастерская) — предложение; владелец может начать сразу с Настольного.

## 5. Риски и проверки на пробе

- **Мост api.ts** — единственная широкая правка в трёх фронтендах; каждая — отдельным шагом с тестами.
- **Своя рамка (TitleBar) во Frameless** — механику перетаскивания сверить на пробе до перевода окон.
- **Сборка v3 использует Taskfile** (`wails3 task build`); `wails3 build` работает как обёртка — сверить, какой путь взять в `build-*.bat`.
- **C91 (прыжок размера окна Мастерской)** — после миграции прогнать те же переходы.
- **Заголовок окна** (`Title: "Wynd [" + version + "]"`) и имя процесса — сверить со «Подписью» (она смотрит имена/пути из exe).
- **CI** (C76): убедиться, что web/server-ворота не зависят от окон (build tags сохранены) и при необходимости добавить сборку окон в gates.

## 6. Решения владельца перед стартом

1. Подтвердить старт миграции и порядок окон (пилот Подпись или сразу Настольный).
2. Целевая версия — v3.1.0 (Latest) или свежее к моменту старта.
3. Окнам, которые остаются на v2 во время поэтапного переезда: поднимать ли их до v2.17.1 (IPC-уязвимость Linux нас не касается — окна на Windows), или не трогать до полного перехода.
4. По C66: если системные уведомления в v3 не подтвердятся — решать, ждать ли их в v3.2 или брать иной механизм.
