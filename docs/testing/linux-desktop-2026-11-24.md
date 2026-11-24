# Linux «Wynd Настольный»: сборка, secure context и первое окно (2026-11-24)

План [69](../plans/69-linux-desktop.plan.md), этап 1. Проверка шла в CI на
`ubuntu-24.04`, сборка GTK3/WebKit2GTK 4.1, окно — под Xvfb (рабочего стола
с треем в CI нет).

## Сборка и ворота

- `scripts/build-desktop-linux.sh`: `CGO_ENABLED=1 go build -tags
  "desktop,production,gtk3"` → `dist/wynd-desktop`; `web/dist` и оболочка
  собираются заранее.
- `scripts/test-desktop-linux.sh`: frontend check, сборка, `go vet/test
  -tags "desktop,gtk3"`, smoke `--smoke-test` в Xvfb.
- GitHub Actions `Desktop checks` (`45372762750`, коммит `f75d58aa`):
  `desktop-linux` — success. Windows job того же прогона — success.
- В Xvfb нет StatusNotifierWatcher: Wails пишет `systray error`, окно
  продолжает работать — это ожидаемое поведение для headless-среды.

## Замер `wails://` (временный `--probe`, после замера удалён)

```
secure: true
origin: wails://localhost
href: wails://localhost/
mediaRecorder: function
getUserMedia: true
clipboard: true
secrets: 200
```

Вывод: `wails://localhost` на WebKitGTK даёт secure context; локальный API
`http://127.0.0.1:<port>` читается с этим Origin (CORS работает), запись
голоса/видео, камера и clipboard не запрещены самим окном. Локальная
HTTP-оболочка и отказ от secure-context-функций не нужны.

Ещё не проверены: `<a download>` на WebKitGTK, SSE на реальном сервере,
фактическая запись звука/видео и кодеки GStreamer — этап живой проверки.

## Снимок

`window.png` — настоящий кадр окна из Xvfb, снят `import -window root`
(1024×900). Видны своя полоса заголовка «Wynd Настольный [0.4.5]», экран
«Войти в Wynd», поле адреса и кнопка «Дальше». Это существующий экран
Windows-версии; новых элементов на нём нет.

## Остатки этапа 1

- Вход и лента на настоящем сервере в Linux-окне (нужна Linux-среда или
  тестовый сервер в CI) — не выполнялись.
- Скачивание файла/архива, SSE и медиа — отдельным прогоном.
- Трей, уведомления, автозапуск, схема `wynd:`, токены — этап 2.
