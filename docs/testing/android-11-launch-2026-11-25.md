# Android 11: приложение закрывалось при запуске (C133)

Владелец 25.11: «Android 0.1.3 и 0.1.4 на планшете падает при запуске» —
окно закрывалось сразу, без сообщения. Планшет — **Samsung Galaxy Tab S5e
(SM-T725), Android 11 (API 30)**; установлен 0.1.3, первая установка Wynd на
этом устройстве (21:26 25.11).

## Причина

`MainActivity.onCreate` ловит исключение и молча вызывает `finish()` — поэтому
на экране ничего нет, а причина остаётся только в журнале. Журнал снят по USB
(`adb logcat`):

```
E WyndAndroid: Cannot open interface
E WyndAndroid: java.lang.NullPointerException: Attempt to invoke virtual method
  'android.view.WindowInsetsController ...DecorView.getWindowInsetsController()'
  on a null object reference
E WyndAndroid:   at com.android.internal.policy.PhoneWindow.getInsetsController(PhoneWindow.java:4130)
E WyndAndroid:   at mixeme.wynd.SystemBarsFrame.colors(SystemBarsFrame.java:89)
E WyndAndroid:   at mixeme.wynd.SystemBarsFrame.<init>(SystemBarsFrame.java:57)
E WyndAndroid:   at mixeme.wynd.MainActivity.createPage(MainActivity.java:105)
E WyndAndroid:   at mixeme.wynd.MainActivity.onCreate(MainActivity.java:67)
```

`SystemBarsFrame` строится в `createPage` **до** `setContentView`, а его
конструктор сразу вызывает `colors(...)`. Ветка API 30+ брала
`Window.getInsetsController()`; на Android 11 `PhoneWindow.getInsetsController()`
разыменовывает ещё не созданный decor и падает. На Android 12+ этот вызов сам
создаёт decor, на API < 30 работает другая ветка (`View.SYSTEM_UI_FLAG_*`),
поэтому на OPPO (Android 10) и телефонах Android 13 сбоя не было — **это не
регрессия 0.1.3**: Android 11 просто не проверяли, путь общий с прежними
версиями. Версии 0.1.3 и 0.1.4 отличаются только иконкой, поэтому обе падали
одинаково.

## Правка

`SystemBarsFrame.colors`: контроллер берётся у decor-представления —
`window.getDecorView().getWindowInsetsController()`. View-вариант до
прикрепления окна честно возвращает `null`; оформление повторяется в
`onAttachedToWindow`, когда контроллер уже есть. Комментарий в коде объясняет
грабли Android 11.

## Проверка

Отладочная сборка из этого дерева (тот же код, что пойдёт в 0.1.5),
установлена на планшет рядом с 0.1.3. Мост WebView через CDP — страница и
процесс живые:

- **Первый запуск (стартовый экран):** процесс жив, страница
  `https://appassets.androidplatform.net/startup/index.html`, в DOM — «Войти в
  Wynd», поле адреса, «Дальше», «Сканировать QR-код», «Фото с QR-кодом».
- **Выбранный сервер (путь интерфейса):** в каталог отладочной сборки записан
  `http://localhost:7676`, стенд поднят на этом компьютере и проброшен
  `adb reverse`. После запуска — `http://localhost:7676/` с экраном входа
  («Дом Ани», «Открыт для новых», «Получить код»), процесс жив, в журнале
  `Selected interface loaded`, ошибок нет. Это ровно та ветка `createPage`,
  где раньше падало.
- **OPPO (Android 10):** та же отладочная сборка запускается, интерфейс
  открывается — ветка API < 30 не задета.

Ворота: `testStandardDebugUnitTest testFdroidDebugUnitTest lintStandardRelease
lintFdroidRelease assembleStandardRelease assembleFdroidRelease` —
`BUILD SUCCESSFUL` (JDK 17, `ANDROID_HOME=D:/android-env/sdk`,
`GRADLE_USER_HOME=D:/android-env/gradle-user-home`).

**Границы.** Автоматический тест класса на JVM не поставить: в проекте нет
Robolectric, а `SystemBarsFrame` — Android-представление; охранник — живая
проверка на Android 11 и комментарий в коде. Подписанный 0.1.5 на планшете
ещё не проверялся — пункт в [реестре](pending-device-checks.md).

## Уборка

Стенд на 7676 остановлен (процесс `dev/wynd-stand-new.exe`), пробросы
`adb forward`/`reverse` сняты, каталог отладочной сборки на планшете возвращён
к пустому, Wi-Fi планшета снова выключен (включался только для попытки
проверки через внешний сервер — сеть не понадобилась).
