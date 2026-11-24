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

## Системная интеграция (код)

- `secrets_linux.go` — Secret Service через `go-keyring`, список origin’ов в
  служебной записи `__index__`; при отказе хранилища ошибка, клиент оставляет
  копию в IndexedDB.
- `autostart_linux.go` — `~/.config/autostart/wynd.desktop`, `--from-startup`;
  строка в настройках говорит «при входе в систему» на всех платформах
  (решение владельца 24.11). **Принято владельцем 24.11 («Принимаем»).**
- `scheme_linux.go` — пользовательский `.desktop` с
  `x-scheme-handler/wynd`, `xdg-mime default`.
- `notify_linux.go` — D-Bus `org.freedesktop.Notifications`; без демона
  возвращается ошибка, а не ложный успех.
- Трей: `SetMenu` с «Открыть»/«Выход»; окно прячется в трей только при живом
  `org.kde.StatusNotifierWatcher`.

Живой проверки на рабочем столе (GNOME/KDE/XFCE), трея, уведомлений и
автозапуска не было: в CI Xvfb этих служб нет.

## Пакеты

- `scripts/package-desktop-linux.sh` собрал в CI:
  `wynd-desktop_0.4.5_amd64.deb` — 5,2 МБ,
  SHA-256 `ae88ab409ad2236fdc182fdfe1d7ff6b9f7146eee48964fd4ee69f91dd3758a8`;
  `Wynd-0.4.5-x86_64.AppImage` — 6,7 МБ,
  SHA-256 `4311e68f42bda216562655748a150ba477a2631856543367baf162c4684cd35d`.
- `.deb` установлен в CI (`apt install`), `wynd-desktop --smoke-test` —
  success; AppImage проверен с `APPIMAGE_EXTRACT_AND_RUN=1` — success.
- Общий выпуск: `.github/workflows/desktop-client.yaml` разделён на
  `build-windows` и `build-linux`, `draft` ждёт оба. `workflow_dispatch`
  (`45378840783`) собрал обе платформы; черновик не создавался.
- Тег `desktop-v0.5.0` пока не ставился: сначала живая проверка и приёмка.

## Остатки этапа 1

- Вход и лента на настоящем сервере в Linux-окне (нужна Linux-среда или
  тестовый сервер в CI) — не выполнялись.
- Скачивание файла/архива, SSE и медиа — отдельным прогоном.
- Трей, уведомления, автозапуск, схема `wynd:`, токены — этап 2.
