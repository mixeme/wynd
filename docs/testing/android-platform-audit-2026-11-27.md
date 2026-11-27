# Аудит оболочки Android по документации платформы (C159)

27 ноября 2026. Вопрос владельца после C158: «Есть ли ещё какие-то аспекты, где
нам следовало в поведении опираться на документацию Android?» Проведён разбор
`android/app/src/main/java/mixeme/wynd` и веб-клиента против документации
Android: [Back и задачи](https://developer.android.com/guide/components/activities/tasks-and-back-stack),
[WebViewClient](https://developer.android.com/reference/android/webkit/WebViewClient),
[WebChromeClient](https://developer.android.com/reference/android/webkit/WebChromeClient),
[Web Share в MDN BCD](https://github.com/mdn/browser-compat-data).

## Найдено

| № | Что | Документация | Было |
| --- | --- | --- | --- |
| 1 | Гибель рендерера WebView | `onRenderProcessGone`: без `true` «app will crash… or be killed» | не переопределён — приложение закрывалось |
| 2 | Геолокация на карте | `onGeolocationPermissionsShowPrompt` + разрешения места | нет ни обработчика, ни разрешений: запрос отклонялся |
| 3 | Полноэкранное видео | `onShowCustomView`/`onHideCustomView`; без них WebView сообщает, что fullscreen не поддержан | кнопка полного экрана в альбоме ничего не делала |
| 4 | «Поделиться» приглашением | `navigator.share` в WebView не реализован (BCD: `webview_android: false`, [crbug 40540400](https://crbug.com/40540400)) | всегда копировало ссылку, системного листа не было |
| 5 | Ссылки `tel:`, `mailto:` и подобные | `shouldOverrideUrlLoading` приходит и для не-HTTP(S) схем | молча отменялись |

## Сделано 27.11 (C159)

- **№1, восстановление после гибели рендерера** (`MainActivity`):
  `WebViewClient.onRenderProcessGone` убирает мёртвый WebView и собирает новый
  на том же адресе (`recoverPage` → `destroyPage` + `createPage`). Сессия,
  очередь и кэш принадлежат профилю WebView и остаются. Если собрать заново
  не удалось — приложение закрывается штатно, а не падает. Без `true` система
  убивает процесс; API 26+, на более старых callback не приходит.
- **№5, системные схемы** (`ServerOrigin.isExternalAppLink`): по жесту
  пользователя `tel:`, `mailto:`, `sms:`, `mms:`, `geo:` открываются через
  `ACTION_VIEW`; http(s) — как раньше в браузере с `CATEGORY_BROWSABLE`.
  Редиректы по-прежнему не запускают чужие приложения (только жест).
- **№4, системный лист «Поделиться»** (`ShareBridge`, `$lib/android/share.ts`):
  origin-bound мост `__wyndShare` принимает только веб-ссылку и показывает
  `ACTION_SEND` chooser. Экраны приглашений (`settings/invite`,
  `settings/invites`) зовут `navigator.share`, затем мост, затем копирование.
  Старый APK без моста и браузер идут прежним путём. Мост сообщает только, что
  лист открыт: подпись «Отправлено» в этом пути не показывается — выбор
  получателя и саму отправку знает только система.
- **№3, полноэкранное видео** (`MediaChromeClient`): `onShowCustomView`
  добавляет вид страницы поверх интерфейса на чёрном фоне и прячет системные
  полосы, `onHideCustomView` возвращает всё назад. Системная «Назад» сначала
  выходит из полного экрана (`MainActivity.goBack` → `exitFullscreen`), второе
  нажатие — обычная навигация. Уход со страницы и уничтожение Activity
  закрывают полный экран.

## Не сделано: геолокация (№2)

Нужно решение владельца: добавление `ACCESS_FINE_LOCATION` и
`ACCESS_COARSE_LOCATION` в манифест меняет карточку приложения в магазинах и
показывает системный диалог. Сейчас веб просит место автоматически при
открытии карты — по документации нужен обработчик
`onGeolocationPermissionsShowPrompt`, разрешения и понятное объяснение до
запроса. В браузере A4 проверялась (реестр), в APK — нет, и без разрешений она
не могла работать. Варианты: (а) включить и спросить разрешение при первом
открытии карты с подсказкой; (б) не включать — карта остаётся «вид на все
точки». Без слова владельца не начинать.

## Проверки

- `web/src/lib/android/share.test.ts` — 3 теста (мост есть/нет, отказ листа,
  таймаут); svelte-check — 0/0; `check:ui` — OK; prettier/eslint по изменённым
  файлам — чисто.
- Android-тест `ServerOriginTest.handsOnlySystemSchemesToOtherApps` — в CI
  (`android.yaml`), локальной сборки нет: на машине нет Android SDK и JDK 17.
- Живые проверки — [реестр](pending-device-checks.md), §26.

## Границы

- Гибель рендерера воспроизводится только на устройстве/эмуляторе:
  `adb shell ps -A | grep sandboxed_process`, затем `kill <pid>`; ожидается
  перезагрузка экрана, а не закрытие приложения.
- Полный экран и системный лист — на живом APK после выпуска (в 0.1.10 этих
  правок нет).
- `tel:`/`mailto:` в интерфейсе пока не встречаются; проверка — на тестовой
  странице dev-стенда.
