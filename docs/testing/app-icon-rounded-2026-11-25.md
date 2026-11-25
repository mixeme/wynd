# Иконка приложения: скруглённые углы — проверка 25.11.2026

## Основа

- Эталон: `assets/icon.svg` (1024×1024, квадратная бумажная подложка); правило
  «своё скругление в эталон не зашивать» — `assets/README.md`, `docs/visual/mark.html`.
- Растры и значки до правки: `web/static/icon-192.png`, `icon-512.png`,
  `cmd/wynd-desktop|wynd-workshop|wynd-signer/build/appicon.png` и
  `build/windows/icon.ico`, `android/app/src/main/AndroidManifest.xml`
  (`@drawable/wynd_icon`). Коммит на начало задачи — `4f90edce`.

## Изменение (коммит `a5d76086`)

1. Производный эталон `assets/icon-rounded.svg`: тот же знак и бумага, скругление
   225 из 1024 (22%), вне скругления прозрачно. Основной `assets/icon.svg` и
   maskable-иконка не меняются: там подложка обязана быть во всё поле.
2. PWA: обычные иконки (`purpose` без maskable) — `icon-192-rounded.png` и
   `icon-512-rounded.png`; maskable остаётся `icon-512.png`; apple-touch — прежняя
   квадратная (iOS маскирует сам).
3. Android: `android:icon="@mipmap/ic_launcher"`. С 8.0 — adaptive-иконка
   (`mipmap-anydpi-v26/ic_launcher.xml`): бумага цветом `#F4F0E9`, знак —
   `assets/icon-android-foreground.png` (высота 73.8% видимой области).
   До 8.0 — готовая скруглённая плитка `icon-192-rounded.png` в `mipmap-nodpi`.
4. Окна Настольного, Мастерской и «Подписи»: скруглённые `appicon.png` и
   `icon.ico` (шесть PNG-слоёв, прозрачные углы).
5. Документы: `assets/README.md`, `docs/visual/mark.html`, `docs/CHANGELOG.md`.

## Рендер

- `assets/icon-rounded.svg` отрисован в Edge headless (1024×1024, прозрачный фон) —
  скругление и прозрачные углы подтверждены.
- Растровые размеры и `.ico` выведены из утверждённого растра 1024 (PIL, LANCZOS);
  наложение снятого знака обратно на бумагу даёт исходник с расхождением ≤1 уровня.
- Adaptive-кадры собраны из настоящих слоёв (бумага + знак из safe zone); форму
  маски задаёт лаунчер, поэтому показаны круг и скруглённый квадрат.
- Изображения: `dev/screens/shots/app-icon-rounded-2026-11-25/`
  (`icon-square.png`, `icon-rounded.png`, `adaptive-circle.png`,
  `adaptive-squircle.png`, `icon-64/32/16.png`).

Границы: установленные APK и exe ещё содержат прежний значок — нужна пересборка
и выпуск; живой лаунчер телефона и панель задач Windows в этой сессии не снимались.
Кадр `adaptive-*` — сборка слоёв с маской, а не снимок лаунчера.

## Проверки

- `./gradlew :app:prepareWyndIcon` — успешно; `:app:processStandardDebugResources` —
  ресурсы с adaptive-иконкой линкуются.
- `npm run build` — манифест PWA с новыми иконками собран, `check`-форматирование
  и eslint `vite.config.ts` чисто.
- `go test ./internal/desktop -run 'Icon|Toast'`, `go test -tags desktop
  ./cmd/wynd-desktop -run BuildAssets` — зелёные (значок и PNG-слои `.ico`).
- Полная сборка APK не завершалась: владелец прервал прогон, задача иконки
  закрыта проверкой ресурсов.

## Ответ владельца

- Где применять: «Везде» (Android APK, окна Windows, PWA) — 25.11.2026.
- Вёрстка и скругление 22%: принято 25.11.2026 («Да.»).
