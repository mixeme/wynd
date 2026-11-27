# «Настольный» на Wails v3: сборка и живая проверка — 18 ноября 2026

Второй шаг миграции (план 64 §4, C94): окно переведено на `github.com/wailsapp/wails/v3@v3.1.0` в ветке `wails-v3`. Здесь — сборка настоящего exe, живой прогон и границы проверки.

## Основа

- Оформление не менялось: оболочка, рамка, экраны и тексты — те же, что в выпущенном 0.3.1. Принятые кадры оболочки — [reviews/desktop-server-flow-2026-11-10](../visual/reviews/desktop-server-flow-2026-11-10/) и [reviews/desktop-server-roles-2026-11-10](../visual/reviews/desktop-server-roles-2026-11-10/), действующий кадр начального экрана — desktop-update-strip-2026-11-11/before.png (снят при 760 × 780); принятая надпись заголовка с версией — коммит `731c56d3`.
- Исходник окна: `cmd/wynd-desktop/main.go`, `frontend/src/{api.ts,App.svelte}`.

## Что изменено в коде

- `main.go`: `wails.Run(&options.App{…})` → `application.New(application.Options{…})` + `Services`; `OnStartup` → `ServiceStartup`; `AssetServer.Handler` → `AssetOptions.Handler`; `Windows.WebviewUserDataPath` → `application.WindowsOptions`; окно — `app.Window.NewWithOptions` (760×780, мин 500×480, `Frameless`, фон F4F0E9, URL «/»); `runtime.Quit`/`BrowserOpenURL` → `app.Quit()`/`app.Browser.OpenURL`. Прокси, CORS, локальный порт apiBase и self-update не менялись.
- `frontend/src/api.ts`: мост — `Call.ByName("main.App.*")` (ручной адаптер, как у «Подписи»; запасной путь `window.go` для mock сохранён), `runtime()` — `Window`/`Application` из `@wailsio/runtime 3.1.0`.
- `frontend/package.json`: добавлен `@wailsio/runtime 3.1.0`.
- `build/windows/info.json` и `wails.exe.manifest`: конкретные значения вместо v2-шаблонов (0.3.1, «Wynd», «Wynd Настольный», `com.wynd.desktop`).
- `scripts/build-desktop.bat`: путь v3 — frontend `npm install` + `npm run build`, `wails3 generate syso`, `go build -tags "desktop,production"` прямо в `dist\wynd-desktop.exe` с вшитым `SigningKey` (как раньше), `.syso` удаляется.
- `.github/workflows/desktop-client.yaml`: CI ставит `wails3@v3.1.0` вместо `wails@v2.16.0`.

## Сборка

`scripts\build-desktop.bat` отработал без ошибок: `dist\wynd-desktop.exe`, 13 568 000 байт. Ресурсы exe: FileVersion/ProductVersion 0.3.1, ProductName «Wynd», FileDescription «Wynd», Comments «Wynd Настольный», CompanyName «Wynd». Открытый ключ вшит (строка ключа находится в бинарнике). Дерево после сборки чистое (`.syso` удалён).

## Живой прогон

Запуск — с изолированным `%APPDATA%` (временная папка), поэтому настоящий профиль `%APPDATA%\Wynd\desktop`, `servers.json`, webview-профиль и автозапуск не затрагивались; после прогона во временном профиле появились только его собственные `servers.json` и `webview\EBWebView`. Кадры — `capture-workshop-window.ps1` (PrintWindow, `PW_RENDERFULLCONTENT`) по pid.

| Проверка | Результат |
| --- | --- |
| Начальный экран | ✓ 760×780, своя рамка, заголовок «Wynd Настольный [0.3.1]» (версия пришла через `Call.ByName`), «Войти в Wynd», поле адреса и «Дальше» — совпадает с принятым кадром, кроме принятой надписи версии — `login-shell.png` |
| Проверка адреса (`CheckServer`) на `https://wynd.mixfed.ru` | ✓ открылся интерфейс сервера в iframe, карточка «Wynd · wynd.mixfed.ru · Только по приглашению» |
| Локальный стенд `127.0.0.1:7676` | ✓ `CheckServer` записал сервер в изолированный `servers.json`, iframe загрузил клиент через v3 `AssetOptions.Handler` и Go-прокси |
| Вход по коду на стенде | ✓ код из журнала сервера принят, открылся живой клиент: «Что нового? Wynd 0.53», круг «Семья», событие «Круг оживлён из архива: Carol сегодня, 10:51» — `app-live.png` |
| ПК7 из клиента | ✓ «Настройки → Приложение → Сменить сервер интерфейса» → окно показало свою страницу «Сервер интерфейса» со строкой «Дом Ани · 127.0.0.1:7676» (обратный путь `postMessage` работает) — `pk7-servers.png` |
| Кнопки своей рамки | ✓ «Развернуть» → 1024×768, значок «Восстановить», обратно 760×780; «Закрыть» завершает процесс |
| CORS локального порта | ✓ как в аудите 17.11: `GET` с `Origin: http://wails.localhost` → 200 + `Access-Control-Allow-Origin`; с `https://evil.example` → 200 без ACAO; `OPTIONS` окна → 204 с полным набором заголовков |
| apiBase и поток событий | ✓ после входа у процесса два установленных соединения с локальным портом окна (входящие от WebView2) и два к порту стенда; долгоживущее соединение — поток событий страницы через apiBase, а не через WebView2 |

Границы:

- Перетаскивание окна и изменение размера краем не проверялись — в хелпере компьютерного доступа этой сборки нет пиксельного кадра для координатных действий.
- «Добавить сервер интерфейса» (предпросмотр по blur) не доведён: без переднего плана не удалось снять фокус с поля, поэтому `PreviewServer`/`AddInterface` живьём не прогонялись (обе — те же `desktop.CheckServer`, что проверен выше; `AddInterface` покрыт Go-тестом).
- Полоса самообновления не показывалась: 0.3.1 — текущая версия, более новой на GitHub нет. Путь `CheckSelfUpdate` вызывался без ошибок; проверка подписи и установки — в тестах `internal/desktop`.
- Подпись/установка обновления и автозапуск живьём не трогались (меняют файлы и реестр пользователя).

## Ворота

- `scripts\test-desktop.bat` — зелёный (frontend «Настольного» check/build, Go vet и тесты с тегом `desktop`, production-сборки трёх окон).
- `scripts\test.bat` — зелёный целиком (web 112 файлов тестов, `go vet ./...`, `go test ./...`, сборка `cmd/wynd`, настольные ворота).

## Осталось по плану 64

Мастерская — последнее окно v2 (диалоги с фильтрами, `EventsEmit`, `overrides_dev.go`, `profiles.test.mjs`). Затем §4.5: убрать v2 из `go.mod`, решить судьбу `wails.json`/`package.json.md5`/`frontend/wailsjs` всех окон. C66: системные уведомления в v3.1.0 по-прежнему не подтверждены.
