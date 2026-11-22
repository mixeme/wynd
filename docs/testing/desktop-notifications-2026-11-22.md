# Настольный: механизм системных уведомлений — 22 ноября 2026

Функция 2 плана [66](../plans/66-desktop-functions.plan.md), шаг 1 — механизм. Кто и когда зовёт мост (шаг 2) — решение владельца, сюда не входит.

## Реализация

- Wails v3.1.0 системных уведомлений Windows не имеет (только iOS) — «иной механизм» по записи плана 46: toast через [go-toast](https://github.com/go-toast/toast) (MIT, в exe только окна; `go list -deps ./cmd/wynd` — сервер не затронут, лицензии сервера не меняются). Скрипт уведомления запускается скрытым PowerShell (`HideWindow: true`).
- **Идентичность** (`internal/desktop/notify_windows.go`): `EnsureToastIdentity` пишет `HKCU\Software\Classes\AppUserModelId\com.wynd.desktop` — DisplayName «Wynd Настольный» и IconUri; значок — крупнейший PNG-слой `build/windows/icon.ico`, извлекается как есть (`extractIconPNG`, все шесть слоёв значка — PNG) и пишется в `%AppData%\Wynd\desktop\toast-icon.png` только при изменении. По прецеденту автозапуска — права только HKCU; AUMID тот же, что у SingleInstance.
- **Правило показа** (`notify.go`): `ShouldNotify(visible, minimised)` — уведомлять, когда окно спрятано в трей или свёрнуто; на виду мост молчит. Мост `App.Notify(title, body)` в `cmd/wynd-desktop/main.go` применяет правило, `ShowToast` — механизм без правила (им пользуется живая проверка).
- **Охраняющие тесты** (`notify_test.go`): таблица ShouldNotify; `sanitizeToastText` — «]]>» не должна закрыть CDATA шаблона раньше времени; `extractIconPNG` на синтетическом ICO (крупный слой, не-PNG слой, мусор); охрана настоящего `icon.ico` — без PNG-слоя тест падает, значок уведомлений молча не потерять.

## Ворота

`scripts\test-desktop.bat` — зелёный целиком (`[desktop] OK`); exe пересобран.

## Живая проверка (пробник `dev/notify-probe` + `dev/notify-shot.ps1`, `dev/notify-frame.ps1` — в git не входят)

1. Пробник (console-приложение, тот же `internal/desktop`): `identity ok` → `toast sent` — без ошибок.
2. Реестр после прогона: `AppUserModelId\com.wynd.desktop` → `DisplayName: Wynd Настольный`, `IconUri: …\toast-icon.png`; значок — корректный PNG 256×256 (8840 байт, слой значка окна).
3. Платформа уведомлений приняла приложение: ключ настроек `HKCU\...\Notifications\Settings\com.wynd.desktop` создан после прогона; `WpnService` Running, RuntimeBroker живы.

## Ограничение проверки — показ баннера за владельцем

Снимок экрана в этой среде недоступен (`CopyFromScreen` — «Неверный дескриптор»), окно toast (`Windows.UI.Core.CoreWindow`) перечислением не поймано — показ баннера на экране не зафиксирован и не заменяется ничем из этого; приёмка не выдаётся за выполненную. **Владельцу показать: баннер «Wynd Настольный — Проверка уведомлений»** с именем и значком — ручной пункт реестра §7.

## Дальше (шаг 2 — решение владельца)

Кандидаты потребителя моста: «вышла новая версия окна» (полоса уже есть, toast — для спрятанного окна), новые записи в кругах (серверная часть веб-клиента + правила показа, как у пушей Android: беззвучные часы, mute). Экрана настроек уведомлений в этом плане нет.
