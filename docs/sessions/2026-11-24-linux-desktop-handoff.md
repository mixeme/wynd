# Передача 24.11.2026 — Linux-сборка «Wynd Настольный»

Сессия: план [69](../plans/69-linux-desktop.plan.md). Начало — «Давай
составим план работы на Linux сборкой Настольного»; после утверждения
плана сделаны этапы 1–3 (без живого рабочего стола). Выпуски сервера и
Android этой сессии — в отдельных передачах
[0.53.7/Android 0.1.2](2026-11-24-android-native-queue-handoff.md) и
[0.53.8](2026-11-24-android-native-queue-handoff.md).

## Сделано

### 1. План 69 и границы (`191f5372`)

- `docs/plans/69-linux-desktop.plan.md`: Ubuntu 24.04 + Debian 12 x86_64,
  `.deb` и AppImage, паритет без самообновления, GTK3/WebKit2GTK 4.1.
- ROADMAP и `desktop-reference.md` — раздел Linux.

### 2. Компиляция, окно и замер WebKit

- `cmd/wynd-desktop/platform_windows.go` и `platform_linux.go`: `w32`,
  иконка трея, WindowsOptions/WindowsWindow, очистка WebView2 и
  `explorer.exe` вынесены из `main.go`; Linux — PNG, `App.Env`,
  нативное меню трея, проверка StatusNotifierWatcher.
- `scripts/build-desktop-linux.sh`, `scripts/test-desktop-linux.sh`,
  job `desktop-linux` в `.github/workflows/desktop.yaml` на ubuntu-24.04.
  Сборка `-tags "desktop,production,gtk3"`, без syso и `-H windowsgui`.
- Временный `--probe` (удалён в `864e8171`) дал в CI:
  `isSecureContext=true`, `Origin=wails://localhost`, fetch к `/__secrets`
  — 200, `MediaRecorder`, `getUserMedia`, clipboard доступны. Локальная
  HTTP-оболочка не нужна.
- Реальное окно снято в Xvfb: `docs/testing/linux-desktop-2026-11-24/window.png`
  ([протокол](../testing/linux-desktop-2026-11-24.md)); виден экран входа
  и своя полоса заголовка.
- CI: `desktop-linux` success (`45372762750`, `45374883523`), Windows job
  того же прогона success.

### 3. Системная интеграция (`07aabb60`, `26f0317c`)

- `internal/desktop/secrets_linux.go` — Secret Service через `go-keyring`,
  индекс origin’ов; при отказе ошибка, IndexedDB остаётся копией.
- `autostart_linux.go` — `~/.config/autostart/wynd.desktop`,
  `--from-startup`, чтение/удаление; тест XDG-файла.
- `scheme_linux.go` — пользовательский `.desktop` с
  `x-scheme-handler/wynd`, `xdg-mime default`; тест.
- `notify_linux.go` — D-Bus `org.freedesktop.Notifications`; без демона
  ошибка, не ложный успех.
- Трей: `SetLabel("Wynd")`, нативное `SetMenu` «Открыть»/«Выход»; окно
  прячется только при живом `org.kde.StatusNotifierWatcher`.
- Строка автозапуска: «при входе в систему» на всех платформах (решение
  владельца 24.11), без определения ОС; реальный кадр настроек снят и
  **принят 24.11 («Принимаем»)**.

### 4. Пакеты и выпуск (`96cefd9f`, `09ad3d29`, `d8686b9d`)

- `cmd/wynd-desktop/build/linux/`: `.desktop`, AppStream, hicolor-иконки
  48/64/128/256, README.
- `scripts/package-desktop-linux.sh`: `.deb` (зависимости GTK3/WebKit2GTK,
  postinst/prerm) и AppImage (appimagetool 1.9.1, SHA-256 закреплён).
- CI: `wynd-desktop_0.4.5_amd64.deb` 5,2 МБ, SHA-256
  `ae88ab40…`; `Wynd-0.4.5-x86_64.AppImage` 6,7 МБ, SHA-256
  `4311e68f…`; `.deb` установлен и smoke пройден, AppImage —
  `APPIMAGE_EXTRACT_AND_RUN=1` smoke.
- `.github/workflows/desktop-client.yaml` разделён на `build-windows` и
  `build-linux`; `draft` ждёт оба, аттестует exe/.deb/AppImage и кладёт
  всё в один черновик. `workflow_dispatch` (`45378840783`) собрал обе
  платформы; черновик не создавался.

## Открыто и дальше

1. **Живая проверка на Linux с рабочим столом** (раздел 12 реестра):
   установка `.deb`/AppImage, вход и лента, SSE, очередь, скачивание,
   запись голоса/видео, трей, уведомления, автозапуск, `wynd:`, токены,
   X11/Wayland/HiDPI. Без неё визуальная приёмка не закрыта.
2. **Кадры на приёмку:** меню трея Linux (нативное, не 92×56), строка
   автозапуска «при входе в систему»; кнопка «Открыть в приложении» в
   Linux-браузере пока не включена.
3. **Скачивание:** `<a download>` на WebKitGTK не проверен; если не
   работает — мост сохранения файла через Go/диалог.
4. **Выпуск:** тег `desktop-v0.5.0` (Windows exe + `.deb` + AppImage) не
   ставился; решение после живой проверки. Linux-пакеты без Ed25519,
   с SHA-256 и attestation. Самообновление Linux — позже.
5. **GTK3 legacy:** Wails может убрать стек; версию держим, обновление
   Wails проверять отдельно. AppImage в Ubuntu 24.04 требует libfuse2t64,
   в Debian 12 — libfuse2.

## Грабли

- **Pathspec-коммит на Windows теряет mode change.** `git commit -- path`
  после `git update-index --chmod=+x` оставляет 100644 в HEAD; CI получает
  Permission denied. Коммитить mode change без pathspec, проверять
  `git ls-tree HEAD`.
- `go:embed` требует готовые `web/dist` и `frontend/dist` до `go vet/test`;
  в Linux-воротах сборка идёт раньше проверок.
- Wails Linux: `App.Env`, а не `App.Environment`; иконка трея — PNG;
  `ContextMenu`/`openMenu` не работают, меню — `SetMenu`; позиционирование
  у курсора на Wayland невозможно.
- В Xvfb нет StatusNotifierWatcher и демона уведомлений: `systray error`
  ожидаем, окно продолжает работать.
- Имя AppImage `Wynd-<версия>-x86_64.AppImage` (дефис перед архитектурой).
- Временный `--probe` удалён; не возвращать в релиз.

## Состояние дерева и среды

- Дерево чистое, кроме untracked `.zcode/` (артефакт инструмента, не наш).
- Коммиты Linux-работы: `191f5372`, `e40d7b81`, `e9bb03fa`, `f56f6ee0`,
  `ebcf4895`, `2601e017`, `c6c17574`, `864e8171`, `f75d58aa`, `7c0de939`,
  `07aabb60`, `26f0317c`, `96cefd9f`, `7307800c`, `09ad3d29`, `d8686b9d`,
  `f6757bfa`. Все в Gitea.
- Артефакты пакетов — GitHub Actions `wynd-linux-packages` (14 дней),
  снимок — `wynd-linux-screenshot`; локальные копии в `dev/linux-packages/`
  и `dev/linux-screenshot/` (в git не входят).
- Windows-выпуск 0.4.5 — черновик, подпись за владельцем; Linux-работы
  его поведение не меняли, Windows job зелёный.
