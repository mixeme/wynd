# План 64. Миграция окон на Wails v3 (C94) — подготовка

**Создан:** 2026-11-17.
**Статус:** 2026-11-18/19 в ветке `wails-v3` все три окна переведены на v3 — «Подпись», Настольный (C94), Мастерская; exe собраны, живые проверки и ворота пройдены (§7). Осталась чистка: убрать v2 из `go.mod`, решить судьбу `wails.json`/`package.json.md5`/`frontend/wailsjs`.
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

## 7. Ход миграции

### 2026-11-18 — пилот «Подпись» (ветка `wails-v3`)

Владелец выбрал пилот «Подпись». Сделано:

- `go.mod`: добавлен `github.com/wailsapp/wails/v3 v3.1.0` рядом с v2.16.0; `go mod tidy` (v3 — прямая зависимость).
- **Общие зависимости поднялись.** v3.1.0 требует `modernc.org/sqlite v1.44.3` (было 1.36.1), поэтому MVS поднял sqlite/libc/memory, `x/crypto` 0.57, `x/sys` 0.48, `x/net` 0.60, `x/text` 0.42, `x/exp`, go-colorable 0.1.14, go-strftime 1.0.0. Сервер и все окна компилируются, но полные ворота `scripts\test.bat` с новым sqlite ещё не прогонялись — обязательный пункт перед слиянием.
- `cmd/wynd-signer/main.go`: `application.New` + `NewService`, `ServiceStartup`/`ServiceShutdown`, окно `app.Window.NewWithOptions` (640×560, мин 560×480, `Frameless`, фон F4F0E9), диалоги `app.Dialog…PromptForSingleSelection`, прогресс `app.Event.Emit`, ссылки `app.Browser.OpenURL`.
- `frontend`: добавлен `@wailsio/runtime 3.1.0`; `api.ts` зовёт `Call.ByName("main.App.*")`; `watchProgress` — `Events.On` с запасным путём для mock (`window.runtime.EventsOn`); `Window.svelte` — `Window`/`Application` из рантайма с тем же запасным путём. `mock/mock.js` не менялся.
- **Отклонение от §4:** сгенерированные биндинги не импортируются. `wails3 generate bindings -ts -names -f "-tags desktop" .` проверен (321 пакет, 1 сервис, 12 методов, 6 моделей; имена `main.App.*`), но импорт каталога `frontend/bindings` заставил бы `test-desktop.bat` и CI требовать CLI wails3. Поэтому `api.ts` остался ручным типизированным адаптером, как v2 `window.go`; каталог bindings не коммитится (удалён после проверки).
- `build/windows/info.json` и `wails.exe.manifest`: v2-шаблоны (`{{.Info.*}}`, `{{.Name}}`) заменены конкретными значениями (Wynd Подпись 0.2.0, `com.wynd.signer`, `asInvoker`) — генератор v3 шаблоны не подставляет.
- `wails3 build` без `Taskfile.yml` не работает («No Taskfile found»). Принят путь явных шагов: `wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest build/windows/wails.exe.manifest -info build/windows/info.json -out <pkg>.syso` + `go build -tags "desktop,production" -trimpath -buildvcs=false -ldflags "-w -s -H windowsgui"`. `scripts\build-signer.bat` ещё не переписан.

Проверено на 2026-11-18:

- `npm run test` — 3/3; `npm run check` — 0 ошибок; `npm run build` — OK.
- `go test -tags desktop ./cmd/wynd-signer` — OK (тест shutdown переведён на `ServiceStartup`/`ServiceShutdown`).
- `go build -tags desktop` и `-tags desktop,production` для «Подписи» — OK; `go build ./cmd/wynd` и production-сборки Настольного/Мастерской — OK (проверка компиляции после подъёма зависимостей).

Сделано 18.11 (продолжение):

1. `scripts\build-signer.bat` переписан: frontend `npm install` + `npm run build`, `wails3 generate syso`, `go build -tags "desktop,production"` прямо в `dist\wynd-signer.exe`, `.syso` удаляется. Exe собран (15 138 304 байта, ресурсы 0.2.0 «Wynd Подпись»).
2. Живая проверка окна пройдена: 640×560, кадры 12а/12б/12д совпадают с принятыми; список выпусков и прогресс проверки идут через `Call.ByName`/`Events.On`; родные диалоги (сохранение ключа, файл ключа, файл для подписи) открываются; тестовый файл подписан; кнопки своей рамки разворачивают/восстанавливают/закрывают окно. Подробно — [протокол](../testing/signer-wails-v3-live-2026-11-18.md).
3. **Найден и исправлен дефект миграции:** отмена родного диалога в v3 возвращает ошибку внутренней библиотеки («cancelled by user»), и окно показывало «Не удалось открыть окно выбора файла: cancelled by user». Добавлен `dialogCancelled` (`ChooseKeyPlace`/`ChooseKey` считают отмену отказом) и охраняющий тест `TestDialogCancelled`. После пересборки проверено живьём.
4. `scripts\test-desktop.bat` — зелёный; `scripts\test.bat` — зелёный целиком с новым sqlite. Попутно (не относится к миграции): поправлено форматирование `StepList.svelte` (prettier падал и в `main`) и пересобран `THIRD_PARTY_LICENSES.txt`; для теста часов запущена и настроена служба `w32time`.
5. Границы живой проверки: перетаскивание окна и изменение размера краем не проверялись — в хелпере компьютерного доступа нет пиксельного кадра; опции размера те же, что в v2.

Осталось:

1. Коммит шага; следующий шаг по §4 — Настольный.
2. Решить судьбу `wails.json`, `package.json.md5` и `frontend/wailsjs` «Подписи» (v3 их не читает).
3. При переводе Мастерской и Настольного повторить проверку отмены диалогов (тот же дефект v3).

### 2026-11-18 — Настольный (C94), второй шаг

Сделано:

- `cmd/wynd-desktop/main.go`: `application.New` + `Services`, `ServiceStartup`, `AssetOptions.Handler` (прокси и CORS без изменений), `application.WindowsOptions.WebviewUserDataPath`, окно 760×780 (мин 500×480, `Frameless`, фон F4F0E9), `app.Quit()`/`app.Browser.OpenURL`.
- `frontend/src/api.ts`: мост на `Call.ByName("main.App.*")`, `runtime()` на `Window`/`Application` из `@wailsio/runtime 3.1.0` (зависимость добавлена); `build/windows` — конкретные значения вместо шаблонов v2 (`com.wynd.desktop`).
- `scripts\build-desktop.bat` — путь v3 (npm, syso, `go build` с вшитым `SigningKey`); `.github/workflows/desktop-client.yaml` ставит `wails3@v3.1.0`.
- Exe собран: 13 568 000 байт, ресурсы 0.3.1 «Wynd». Живая проверка: начальный экран 760×780, проверка адреса, вход на локальном стенде, живой клиент в iframe (СЕЕ через apiBase), ПК7 из клиента (`postMessage`), кнопки рамки, CORS локального порта — всё совпало с принятым оформлением и поведением v2. Подробно — [протокол](../testing/desktop-wails-v3-live-2026-11-18.md).
- Ворота: `test-desktop.bat` и `test.bat` зелёные целиком.

Осталось:

1. Мастерская — последнее окно (диалоги с фильтрами, `EventsEmit`, `overrides_dev.go`, `profiles.test.mjs`); повторить проверку отмены диалогов.
2. §4.5: убрать v2 из `go.mod`; решить судьбу `wails.json`, `package.json.md5`, `frontend/wailsjs` окон; проверить CI-воротa.
3. C66: системные уведомления в v3.1.0 не подтверждены.

### 2026-11-18/19 — Мастерская, третий шаг

Сделано:

- `main.go`: `application.New` + `Services`, `ServiceStartup`/`ServiceShutdown`, `AssetFileServerFS`, окно 760×560 (мин 680×480, `Frameless`, фон F4F0E9), `app.Quit()`/`Browser.OpenURL`/`Clipboard.SetText`.
- `archives.go`: диалоги на `app.Dialog` (включая `PromptForMultipleSelection` и выбор папки через `CanChooseDirectories`).
- `frontend/src/api.ts`: мост `Call.ByName("main.App.<метод>")` через объект-посредник; `Window.svelte` — `Window`/`Application` из `@wailsio/runtime 3.1.0`; mock-путь `window.go`/`window.runtime` для галереи и теста сохранён. В `profiles.test.mjs` — заглушка `fetch` (runtime v3 при загрузке проверяет `/wails/custom.js`).
- `build/windows` — конкретные значения (`com.wynd.workshop`); `build-workshop.bat` — путь v3; CI — `wails3@v3.1.0`. Exe собран: 18 239 488 байт, ресурсы 0.4.5 «Wynd Мастерская».
- Живая проверка: начальный экран 760×560, форма сервера и сохранение профиля, проверка сервера (отпечаток, отмена), вкладка «Архивы», выбор `.wynd` и отказ по паролю, диалог сохранения ZIP, множественный выбор и его отмена без ошибок, кнопки рамки. Подробно — [протокол](../testing/workshop-wails-v3-live-2026-11-19.md).
- Ворота: `test-desktop.bat` (24 теста фронтенда Мастерской) и `test.bat` зелёные целиком.

Осталось:

1. §4.5: убрать v2 из `go.mod`; решить судьбу `wails.json`, `package.json.md5`, `frontend/wailsjs` всех окон; проверить CI-воротa.
2. C66: системные уведомления в v3.1.0 не подтверждены.
3. Живьём на сервере не прогонялись установка/обновление/откат/переезд (логика `internal/workshop` не менялась) — при первой живой установке после выпуска.
