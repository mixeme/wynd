# 69 — Linux-сборка «Wynd Настольный»

**Тип:** новая платформа клиента.
**Статус:** план утверждён 2026-11-24. Целевые системы — Ubuntu 24.04 и Debian 12, x86_64. Первая версия — паритет с Windows, кроме самообновления (отдельным решением). Поставка — `.deb` и AppImage. Рабочее предложение по GTK — одна сборка `-tags "desktop,production,gtk3"` (WebKit2GTK 4.1) для обеих систем; окончательно подтверждается замером на живом окне.
**Обновлено:** 2026-11-24.

## Контекст

Оболочка сейчас собирается только под Windows: Wails v3.1, WebView2, `w32`, реестр, Credential Manager, AUMID, `-H windowsgui`. В [справочнике](../reference/desktop-reference.md) Linux обозначен как «после Windows»; отдельного плана и сборочного контура нет. Первый Linux-выпуск должен дать рабочее окно с тем же интерфейсом и системной интеграцией, что на Windows, но без механизма самообновления.

## Принятые решения

- **Системы:** Ubuntu 24.04 + Debian 12, x86_64. ARM64, Fedora, Arch и старые LTS — не в первой версии.
- **Поставка:** `.deb` (основной путь для Ubuntu/Debian) и AppImage (переносимый запуск). Обычный tar.gz отдельным продуктом не выпускаем.
- **Паритет:** вход, круги, лента, очередь, вложения, трей, уведомления, автозапуск, горячая клавиша, токены, схема `wynd:`. Самообновление — позже.
- **GTK:** одна сборка GTK3/WebKit2GTK 4.1 для обеих систем. GTK4 не подходит Debian 12 (GTK 4.8 при требовании Wails 4.14+). GTK3 в Wails v3.1 — legacy; версия Wails закреплена, обновление проверяется отдельно.
- **Origin и WebKit:** `wails://localhost` на Linux не зарегистрирован как secure. Первый этап — замер `isSecureContext`, `Origin`, fetch, SSE и скачивания. Обход (локальная HTTP-оболочка, мост скачивания или принятые ограничения) выбирается по результату, а не заранее.
- **Проверка:** сначала wynd3 + Xvfb для окна и рендера; финальная приёмка трея, уведомлений, автозапуска и схемы — на Linux-машине с рабочим столом.
- **Первый выпуск:** `desktop-v0.5.0`, Windows exe и Linux-пакеты в одном черновике. Linux-пакеты без Ed25519-подписи до подписи владельцем; после неё — `.sig` для каждого.

## Этап 0. План и границы

- Этот документ, строка в [ROADMAP](../ROADMAP.md), раздел Linux в [справочнике](../reference/desktop-reference.md).
- Windows-поведение не меняется; новых экранов не рисуем, используем существующие.

## Этап 1. Компиляция, окно и замер WebKit

**Статус:** сборка, Linux-ворота, smoke и замер `wails://` выполнены 24.11
([протокол](../testing/linux-desktop-2026-11-24.md)); настоящее окно снято в
Xvfb. Вход/лента на сервере, SSE и скачивание ждут живой Linux-среды.

1. **Платформенные точки** в `cmd/wynd-desktop`:
   - `w32.GetCursorPos` и показ окна tray-menu — `tray_windows.go`; Linux — нативное меню (этап 2);
   - иконка трея: ICO на Windows, PNG на Linux;
   - `WindowsOptions`/`WindowsWindow`, удаление `EBWebView/.../Service Worker`, `explorer.exe` — под Windows-хелперы; на Linux — `App.Env.OpenFileManager`/`xdg-open`;
   - `startInTray`/`--from-startup`: на Linux не прятать окно, если трей недоступен.
2. **Сборка** `scripts/build-desktop-linux.sh`: frontend и `web/dist`, `CGO_ENABLED=1 go build -tags "desktop,production,gtk3" -ldflags "-s -w -X …/internal/desktop.SigningKey=" -o dist/wynd-desktop ./cmd/wynd-desktop`. Без `wails3 generate syso` и `-H windowsgui`.
3. **Ворота** `scripts/test-desktop-linux.sh` и job в `.github/workflows/desktop.yaml` на `ubuntu-24.04`: `libgtk-3-dev libwebkit2gtk-4.1-dev libx11-dev pkg-config build-essential`; frontend check/build, `go vet/test -tags "desktop,gtk3"`, сборка, артефакт и SHA-256; `xvfb-run` smoke с `--smoke-test`.
4. **Замер** временным `--probe` (после замера удалить): `window.isSecureContext`, фактический `Origin`, fetch на `apiBase`, SSE, `<a download>`, `MediaRecorder`/`getUserMedia`. Проверить `windowOrigin` для `wails://localhost` и тест CORS.
5. **Живой запуск** на Ubuntu 24.04/Debian 12 (сначала Xvfb): вход, список кругов, лента, `/sync`, медиа; настоящий снимок окна.

**Стоп-гейт снят замером 2026-11-24** (CI Ubuntu 24.04, Xvfb, GTK3): `isSecureContext=true`, `Origin=wails://localhost`, fetch к `/__secrets` — 200, `MediaRecorder`, `getUserMedia` и clipboard доступны. Локальная HTTP-оболочка и принятие ограничений не нужны. Ещё не проверены: `<a download>` на WebKitGTK, SSE на реальном сервере, кодеки GStreamer и запись звука/видео вживую.

## Этап 2. Системная интеграция

**Статус:** бэкенд сделан 24.11 — Secret Service, XDG-автозапуск, схема `wynd:`, D-Bus-уведомления, проверка StatusNotifierWatcher и общий текст автозапуска. **Строка «при входе в систему» принята 24.11.** Вживую на рабочем столе не проверено; кнопка «Открыть в приложении» в Linux-браузере пока не включена.

1. **Трей.** Linux: PNG-иконка, `SetLabel("Wynd")`, нативное `SetMenu` с «Открыть»/«Выход»; кастомное окно 92×56 остаётся только на Windows. Если трея/хоста нет — окно не прячется. Снимок меню — на приёмку.
2. **Уведомления.** Wails `pkg/services/notifications` (D-Bus) или прямой D-Bus; сохранить `ShouldNotify` и дедуп. Заглушки больше не возвращают ложный успех.
3. **Автозапуск.** `autostart_linux.go`: XDG `~/.config/autostart/wynd.desktop`, `--from-startup`, чтение/удаление; тесты. Текст «при входе в Windows» заменён на «при входе в систему» **на всех платформах** (решение владельца 24.11), реальный кадр снят и **принят 24.11 («Принимаем»)**.
4. **Токены.** `secrets_linux.go` на `go-keyring`/Secret Service; лимит 1024 байта только для Windows; при отказе хранилища — ошибка, чтобы клиент оставлял копию в IndexedDB.
5. **Схема `wynd:`.** `scheme_linux.go`: `.desktop` с `MimeType=x-scheme-handler/wynd;`, `Exec=<binary> %u`, установка в `~/.local/share/applications`, `xdg-mime default`, `update-desktop-database`. Кнопка «Открыть в приложении» — и на Linux, с кадром.
6. **Окно и закрытие.** Скрытие по крестику — только при доступном трее; иначе выход. Проверить frameless-ресайз GTK3 на X11/Wayland; при поломке — системные рамки отдельным решением.
7. **Горячая клавиша.** `Ctrl+Alt+W`: X11 работает; Wayland — portal, может быть отклонена композитором; ошибка логируется.
8. **Скачивание.** Если `<a download>` в WebKitGTK не работает — мост сохранения файла через Go и диалог для `openDownload.ts`/вложений. **Код-проверка 24.11:** в Wails v3.1 нет обработчика `WebKitDownload`; вероятно, без моста скачивание не начнётся. Живой прогон и решение о мосте — на Linux-машине; факт не подменяется.
9. **Тесты.** Юнит-тесты autostart/scheme/secrets/политики трея; Linux CI без реального D-Bus.

## Этап 3. Пакеты `.deb` и AppImage

**Статус:** пакеты и CI выпуска сделаны 24.11. CI собрал `wynd-desktop_0.4.5_amd64.deb` и `Wynd-0.4.5-x86_64.AppImage`, установил `.deb` и прошёл smoke обоих; `workflow_dispatch` собрал Windows exe и Linux-пакеты. **Debian 12:** `.deb` установлен и smoke пройден, AppImage — с `APPIMAGE_EXTRACT_AND_RUN`; обычный FUSE-запуск в контейнере не проверялся. **25.11 создан черновик `desktop-v0.5.0`** (exe, `.deb`, AppImage; [протокол](../testing/release-desktop-0.5.0-2026-11-25.md)); «Подпись» проверена на настоящем черновике, но подпись/публикация — за владельцем.

1. **Ресурсы** `cmd/wynd-desktop/build/linux/`: `wynd.desktop`, hicolor-иконки (256/128/64/48), `wynd.metainfo.xml`, `README-linux.txt`.
2. **`.deb`:** staging-дерево `/usr/bin/wynd-desktop`, `/usr/share/applications/wynd.desktop`, `/usr/share/icons/hicolor/...`, `/usr/share/metainfo/...`; control `wynd-desktop`, `amd64`, зависимости `libgtk-3-0`, `libwebkit2gtk-4.1-0`, `libjavascriptcoregtk-4.1-0`, `libsoup-3.0-0`, `xdg-utils`; recommends GStreamer, keyring, portal; `postinst`/`prerm` обновляют desktop/mime database. Имя `wynd-desktop_<версия>_amd64.deb`.
3. **AppImage:** AppDir + `appimagetool` (закреплённая версия, `APPIMAGE_EXTRACT_AND_RUN=1` в CI), `AppRun`, `.desktop`, иконка, metainfo. GTK/WebKit — системные; в README — `libfuse2t64` (Ubuntu 24.04) и `libfuse2` (Debian 12). Имя `Wynd-<версия>-x86_64.AppImage`.
4. **`scripts/package-desktop-linux.sh`** собирает оба пакета и `SHA256SUMS`; версия пакета сверяется с `desktop.Version`.
5. **CI выпуска:** `build-windows`, `build-linux`, затем `draft`: аттестовать exe, `.deb`, AppImage; один черновик с exe, суммами, `.deb` и AppImage. **«Подпись» проверяет Linux-пакеты по `SHA256SUMS`/удостоверению и подписывает их тем же ключом; `.sig` выкладывается для каждого файла. Кадр окна принят 25.11.**
6. **Первый выпуск:** `desktop-v0.5.0` (новая платформа), Windows exe + Linux `.deb`/AppImage; без Ed25519-подписи Linux, с SHA-256 и attestation. CHANGELOG, DEVELOPMENT, desktop-reference, ROADMAP.

## Этап 4. Живая проверка и приёмка

1. Установка `.deb` через `apt install ./...deb` и запуск AppImage; вход, круги, лента, медиа.
2. `/sync`, офлайн-очередь, вложения, скачивание архива/файла, запись голоса/видео (если secure context решён).
3. Трей: показать/скрыть, «Открыть», «Выход»; закрытие окна.
4. Уведомления о новой записи и версии; автозапуск после входа в систему.
5. Ссылка `wynd:` из браузера; второй запуск.
6. Горячая клавиша на X11; на Wayland — поведение portal.
7. Токены после перезапуска; Secret Service без ключей.
8. Тёмная/светлая тема, HiDPI, изменение размера, X11 и Wayland.

Новые Linux-элементы (трей-меню, текст настроек, кнопка приглашения) показывать в настоящем рендере и получать явное «принимаем». Windows-регрессия: `scripts\test.bat`, `build-desktop.bat`, запуск exe.

## Открытые вопросы

- Secure context и `wails://`: решение по обходу — после замера.
- Номер первого Linux-выпуска: предлагается 0.5.0.
- Linux-машина с рабочим столом для приёмки трея/уведомлений/автозапуска.
- Механизм обновления `.deb`/AppImage — после первого выпуска.
