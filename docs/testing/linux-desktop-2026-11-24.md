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
По коду Wails v3.1 обработчика `WebKitDownload`/`download-started` нет:
скорее всего, ссылки скачивания в Linux-окне не начнут сохранение без
своего моста. Это не выдаётся за проверенный факт; мост не реализован,
живой прогон — на Linux-машине.

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
- **Версия 0.5.0 подготовлена 24.11** (номер, whatsnew, CHANGELOG), пакеты
  собраны CI (`45431552325`): `wynd-desktop_0.5.0_amd64.deb` — SHA-256
  `936dbfde20d604abe8723930668ce6b8ea42c82a3412b626e7c6b2b525d3a1a2`;
  `Wynd-0.5.0-x86_64.AppImage` — SHA-256
  `52c500c8d99313c0bfd4e00f698a13594d3681aeecea6f917e602557c75120e8`.
  Тег `desktop-v0.5.0` и черновик не создавались.
- `.deb` установлен в CI (`apt install`), `wynd-desktop --smoke-test` —
  success; AppImage проверен с `APPIMAGE_EXTRACT_AND_RUN=1` — success.
- **Debian 12 (контейнер, 24.11):** `.deb` установлен (`Package:
  wynd-desktop`, `Version: 0.4.5`, `Architecture: amd64`), зависимости
  разрешились, `wynd-desktop --smoke-test` — success. AppImage скопирован
  в контейнер (mount read-only не даёт chmod), запущен с
  `APPIMAGE_EXTRACT_AND_RUN=1` — success. Обычный запуск AppImage через
  FUSE в контейнере не проверялся: нет `/dev/fuse`; для Ubuntu 24.04
  нужен `libfuse2t64`, для Debian 12 — `libfuse2`.
- **Linux-ворота** теперь гоняют `go vet/test -tags "desktop,gtk3" ./...`
  по всему контуру; CI `Desktop checks` на `afe7c9c9` — success.
  Frontend Мастерской и «Подписи» собираются перед проверкой, иначе
  `go:embed` не находит их `frontend/dist`.
- **«Wynd Подпись» умеет Linux:** для выпуска Настольного проверяет
  `.deb` и AppImage по `SHA256SUMS` и удостоверению сборки и подписывает
  каждый файл тем же ключом; `.sig` выкладываются в выпуск. Кадр окна —
  `signer-linux-extras.png`; **принят 25.11 («Принимаем»)**.
- Общий выпуск: `.github/workflows/desktop-client.yaml` разделён на
  `build-windows` и `build-linux`, `draft` ждёт оба. `workflow_dispatch`
  (`45378840783`) собрал обе платформы; черновик не создавался.
- Тег `desktop-v0.5.0` пока не ставился: сначала живая проверка и приёмка.

## Остатки этапа 1

- Вход и лента на настоящем сервере в Linux-окне (нужна Linux-среда или
  тестовый сервер в CI) — не выполнялись.
- Скачивание файла/архива, SSE и медиа — отдельным прогоном.
- Трей, уведомления, автозапуск, схема `wynd:`, токены — этап 2.
