# Android: файлы и доступ к камере/микрофону, 2026-11-16

Поручение владельца: обновления отложены, продолжать остальное. Этот шаг
продолжает этап 1 плана 50. Интерфейс приходит с сервера; вшивания, выпуска
новой версии и работы с Play Защитой в этом шаге нет.

## Реализация

`MediaChromeClient` обслуживает системный файловый диалог и WebView
PermissionRequest. На старте разрешения не запрашиваются. Камера и микрофон
запрашиваются отдельно, только для ресурсов запроса выбранного origin при
открытой странице этого же сервера. Неизвестные ресурсы WebView, чужой origin
и запросы в фоне отклоняются. После ответа Android права и origin проверяются
повторно; отменённый или ушедший со страницы запрос не получает разрешение.

Файлы выбираются через ACTION_OPEN_DOCUMENT: один/несколько, MIME-фильтр.
Возвращаемые URI проверяются до передачи в WebView: только content://,
не собственный provider/UID, без user-info/fragment, с реально доступным
дескриптором чтения. У одиночного выбора несколько URI не принимаются.
Обработана отмена и повторный выбор; проверка провайдера не блокирует UI thread.
Общих READ_EXTERNAL_STORAGE/READ_MEDIA разрешений нет. `allowFileAccess` и
`allowContentAccess` WebView остаются false — выбранный File читается своим
штатным путём, что проверено на телефоне.

CAMERA и RECORD_AUDIO — runtime-разрешения; MODIFY_AUDIO_SETTINGS — normal,
требуется Chromium для аудиоустройства. Без него реальный WebView возвращал
NotReadableError и журнал `Requires MODIFY_AUDIO_SETTINGS and RECORD_AUDIO`.
После добавления запись декодируется. Новых runtime-библиотек нет.

Поворот/размер экрана/клавиатура обрабатываются без пересоздания Activity:
иначе незавершённый файловый callback нельзя перенести в другой WebView.
Полную гибель процесса этот механизм не предотвращает.

Основания: [FileChooserParams — проверка недоверенных URI](https://developer.android.com/reference/android/webkit/WebChromeClient.FileChooserParams),
[PermissionRequest — только явный список ресурсов](https://developer.android.com/reference/android/webkit/PermissionRequest),
[Chromium AudioManagerAndroid](https://chromium.googlesource.com/chromium/src/media/+/master/base/android/java/src/org/chromium/media/AudioManagerAndroid.java).

## Граница прямой фотосъёмки

Эксперимент с передачей HTML capture во внешнюю камеру OPPO не включён в
итоговый код. Камера открылась; после возврата обнаружены новый процесс Wynd
и отсутствие прежнего диагностического input. Надёжный возврат callback не
подтверждён; причину пересоздания отдельно от гонок CDP не установили.
При отмене остался пустой временный файл; фотография
не делалась. Этот результат нельзя называть успешным возвратом из камеры.
Из реализации удалены внешний capture intent, FileProvider и его cache-path.
Здесь работает базовый выбор файлов; камера QR/видео работает внутри WebView
через getUserMedia. Отдельный устойчивый сценарий фотосъёмки остаётся в этапе 1.

## Проверки

Среда: OPPO CPH1931 / Android 10 API 29. В конце прогона поставщик WebView и
User-Agent сверены: com.google.android.webview 157.0.8103.38 (в первом каркасе
ранее был 130.0.6723.107; его версию автоматически на этот шаг не переносим).
Dev-пакет `mixeme.wynd.debug`, интерфейс `http://127.0.0.1:7696` через adb reverse.
Сборка ставится обычным ADB install -r с сохранением данных; это не проверка
самообновления. Изолированный сервер: `dev/data-android-skeleton`.

- JVM: 19 тестов, из них 4 новых MediaPolicy (ресурсы, MIME и границы URI).
- Строгий lintDebug/lintRelease и assembleDebug проходят.
- Web: 20 тестов record/links/Android bridge, Svelte check без ошибок/предупреждений,
  UI guard, ESLint изменённых файлов, production build проходят. Обычные
  предупреждения qrcode/theme_color/chunk size не связаны с этим шагом.
- Запуск без запроса разрешений; запрос камеры с существующего QR-экрана;
  отказ возвращает понятную ошибку, повторный запрос позволяет дать доступ.
- Камера отдельно: живой поток 480×640, только video track.
- Микрофон: audio/webm;codecs=opus, 2298 байт, декодировано 20160 mono samples.
- Камера с микрофоном: video/webm;codecs=vp8,opus, 50834 байта;
  записанный Blob декодирован в видеокадр 480×640. Дорожки audio+video.
- Тестовый TXT (24 байта) выбран в DocumentsUI и прочитан через File.text()
  с точным совпадением `WYND_ANDROID_MEDIA_TEST\n`.
- Повторный выбор и отмена завершаются callback; Back внутри вложенной папки
  сначала поднимает папку, отмена проверяется после закрытия всего диалога.
- Отдельный scaffold smoke прошёл: вход, переход к приглашению и системная
  «Назад», отмена чужого главного документа, сохранение localStorage после
  force-stop/холодного старта, отсутствие необработанных JS ошибок.

В инструментах проверки исправлены гонки: ожидание нового CDP loader/URL вместо
старого готового документа, ожидание PID при холодном старте, пробуждение перед
вводом, повтор чтения uiautomator при пустом root. Неуспешные ранние запуски
этих инструментов не записываются как сбой функции приложения.

Короткие аудио/видеозаписи оставались только в памяти, затем потоки остановлены,
Blob URL освобождены. Никаких входов, записей в круг и загрузки медиа на сервер
не выполнялось. JSON с одними параметрами результата:
`D:\android-env\test-media\capture-result.json`.

Окончательный debug APK (0.1.0-debug) имеет SHA-256
`eeaf30b35b8bd2945729fd48d5986287d8542c9a4aa823fc20a65460dbaffcf8`;
`MediaChromeClient.java` —
`50692790a860cf2d6bd18f747345ada3cf860da9345b420661b8f6c51b584b94`.
После итогового прогона собственный TXT и пустой файл отменённого эксперимента
с камерой удалены с телефона с проверкой ожидаемого содержимого/нулевого размера.

Повтор:

```powershell
.\scripts\build-android.ps1 -ServerUrl http://127.0.0.1:7696
node scripts/test-android-media.mjs SERIAL capture
node scripts/test-android-media.mjs SERIAL file-open
# В Downloads выбрать папку wynd-android-media-test; если она уже открыта:
node scripts/test-android-media.mjs SERIAL file-select
node scripts/test-android-media.mjs SERIAL file-result
node scripts/test-android-media.mjs SERIAL file-open
# Закрыть диалог системной «Назад» до возвращения в Wynd, затем:
node scripts/test-android-media.mjs SERIAL file-cancel
```

Диагностический input для проверки файлов не является новым экраном продукта
или снимком для согласования вёрстки. Результат не подменяет проверку всех
пользовательских сценариев compose/аватара/архива и realme 8.

## Визуальная сверка

Основа: `docs/visual/screens.html`, #e7-45 (разрешение по действию),
коммит `825921cc100f5cd8b793b5a245fa916d14ba426f`; существующий QR Scanner —
`web/src/lib/components/forms/QrScanner.svelte`, основа `da4749182c94fa22219bea004f5d291a3cd7707e`.
Общий cameraFailure — `web/src/lib/media/record.ts`, основа `5d205b267b6112eb85aa50ea954d9243b25fe4bb`.

Меняются только Android-подсказки: вместо настройки браузера названы Wynd и
настройки телефона. Компоненты, геометрия, шрифты и навигация не менялись.
Фраза браузера сохраняется при отсутствии Android-моста.

Реальный WebView, светлая тема, 720×1600 px; CSS 360×728, DPR 2, масштаб 1.
Кадры `dev/screens/shots/` сняты ADB screencap, оба осмотрены:

- `android-media-before-qr-denied.png`, SHA-256
  `ee155e72600f0a4fcdbf7116f65a1837d8a109940bf7f1beac50d12674f59df3`.
- `android-media-after-qr-denied.png`, SHA-256
  `7a383f2cc2835d0259636bc4b25c1cea5d185a9db42d1b8a757c128e9b929777`.
- `android-media-camera-permission.png` — настоящий системный вопрос Android 10;
  его вид задаёт ОС, не HTML эскиза 7.45.
- `android-media-test-file-picker.png` — только собственная тестовая папка,
  диагностическое свидетельство, не материал утверждения Wynd UI.
- `android-media-regression/android-scaffold-login.png` — итоговый холодный
  запуск; SHA-256 `f1804472b16522b912a7ba695e19e8410a0c6065b6c3f00eaa283af105b6cdbe`.
  Осмотрен отдельно; исходный кадр каркаса не перезаписывался.

На паре QR сохранены шапка, назад, видеоблок, поля и нижняя область; новая
подсказка занимает те же две строки, наложений нет. Пустой видеоблок с большим
play значком существовал до этого шага и не перерабатывался. Видеозаписи с
камеры в снимки не включались. cameraFailure в VideoRecorder отдельно не снят.
Новой приёмки вёрстки владельцем эта запись не объявляет.

SHA-256 исходников, соответствующих снятому Android-тексту:
QrScanner `ba00e3fb057d1e8f2c6db2c66dcd9c6aa219a0de8feae305116a538d23db1155`,
record.ts `e8daa280e2ff094d4aca61c97cca6459c9359215a951b1258d273a20a6641d50`.

## Осталось

Первый запуск/выбор сервера, устойчивая фотосъёмка, сохранение/открытие скачанного,
цвет системных полос, автозаполнение и остальные проверки этапа 1 остаются
открытыми. API 21, Android 13+ и realme 8 здесь не проверены. Обновления и
вшивание отложены по текущему решению владельца.
