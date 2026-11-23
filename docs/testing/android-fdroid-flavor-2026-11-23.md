# Android: флейвор fdroid для F-Droid — 23 ноября 2026

Шаг 1 плана [67](../plans/67-fdroid-android.plan.md): редакция приложения без кода самообновления — под политику f-droid.org; редакция владельца `standard` не меняется.

## Реализация

- `android/app/build.gradle`: product-флейворы `standard` и `fdroid` (dimension `distribution`), без изменения package (`mixeme.wynd`) и версии.
- Реализация обновления — `UpdateController`, `UpdatePolicy`, `UpdateTransport`, `ReleaseCatalog`, `ApkVerifier` — переехала в `app/src/standard/java`; fdroid-редакция её не собирает.
- `InstallResultReceiver` остался в `main` в обеих редакциях: lint MissingClass ругается на манифестную ссылку при исключении класса, а receiver безобиден — только пересылает результат `PackageInstaller` контроллеру; заглушке добавлен no-op `installationResult`.
- `app/src/fdroid/java/mixeme/wynd/UpdateController.java` — заглушка с тем же API: `status()` отвечает `capabilities.updates: false` (веб-клиент сам не включает полосу обновления, `web/src/lib/android/update.svelte.ts:97`), `check`/`prepare`/`setImmediately` бросают `IllegalStateException`, install/resume/pause — no-op. Парность API держит компиляция: общие `AndroidBridge` и `MainActivity` собираются в обеих редакциях без веток.
- `app/src/fdroid/AndroidManifest.xml` убирает `REQUEST_INSTALL_PACKAGES` из слитого манифеста (`tools:node="remove"`).
- Тесты обновления — в `app/src/testStandard/java`; охраняющий тест заглушки — `app/src/testFdroid/java/mixeme/wynd/UpdateControllerStubTest.java`: состав ключей `status` (контракт веба), `updates: false`, отказ мутаций без смены состояния.
- CI `.github/workflows/android.yaml`: полный флейворный набор (`testStandardDebugUnitTest testFdroidDebugUnitTest lintStandardDebug lintFdroidDebug lintStandardRelease lintFdroidRelease assembleStandardRelease assembleFdroidRelease`); fdroid-APK идёт в Actions artifact, но не в черновик релиза. `scripts/build-android.ps1` — стандартные задачи по умолчанию. README — новые задачи, пути, раздел «Редакции standard и fdroid».

## Ворота

Локальный прогон всей матрицы CI (gradlew из `D:\android-env`, JDK 17): `BUILD SUCCESSFUL`, 166 задач. Тесты: `UpdatePolicyTest` 6/6 и `UpdateTransportTest` 4/4 (standard), `UpdateControllerStubTest` 3/3 (fdroid), остальные 8 наборов в обеих редакциях — без провалов; lint четырёх вариантов и сборка обеих release — зелёные. Проверка на GitHub Actions пройдёт при следующем пуше в main (зеркало запускает workflow).

## Проверка собранных APK

- `aapt dump permissions` (`build-tools 35.0.0`): fdroid-APK **без** `REQUEST_INSTALL_PACKAGES`; standard-APK — с ним, состав прочих разрешений одинаков (package `mixeme.wynd` у обоих).
- `dexdump` классов: в fdroid-APK нет `UpdatePolicy`/`UpdateTransport`/`ReleaseCatalog`/`ApkVerifier`; `UpdateController` и `InstallResultReceiver` присутствуют (заглушка и общий receiver). В standard-APK все пять на месте.
- Путь выхода: `app/build/outputs/apk/{standard,fdroid}/release/app-{standard,fdroid}-release-unsigned.apk` — как в CI-копировании.

## Живая проверка на OPPO (шаг 2 плана 67)

23 ноября, OPPO CPH1931 (Android 10), отладочный пакет `mixeme.wynd.debug`, стенд «Дом Ани» `http://127.0.0.1:7676` через `adb reverse`, локальный fixture-каталог выпусков `127.0.0.1:7697` со сценарием `mature` (выпуск 0.1.1 старше 72 ч, подпись и сумма совпадают). Обе редакции собраны из этого же дерева с `-PwyndUpdateFixtureOrigin=http://127.0.0.1:7697`; установка `adb install -r -d` (пакет и подпись общие, редакции заменяют друг друга, данные приложения сохраняются). Драйвер — `dev/flavor-live.mjs` (в git не входит), запросы через тот же origin-bound мост, что у клиента.

- **standard-debug — доказательство среды.** `status` → `updates:true`, `protocol:1`; `check` → `available:true`, `nextVersion:"0.1.1"`. Клиент сам показывает полосу **«Вышла версия 0.1.1 · Обновить приложение»** на экране входа: [standard-banner.png](fdroid-flavor/standard-banner.png).
- **fdroid-debug — тот же fixture, та же страница.** `status` → `updates:false`, `state:"idle"`, `available:false`, `nextVersion:""`; ручной `check` из страницы отклонён (`IllegalStateException` заглушки, ответ снова `updates:false`, `error:""`). Полосы нет ни на входе: [fdroid-login.png](fdroid-flavor/fdroid-login.png), ни в ленте: [fdroid-circles.png](fdroid-flavor/fdroid-circles.png); вход Ани кодом из `dev/data/dev-auth-codes.log` и список кругов («Дача», «Семья») работают — интерфейс живой.

Границы прогона: шеринг и пуш не перезапускались — их код общий и в этом шаге не менялся (dexdump выше: в fdroid-APK отсутствуют только классы обновления); push и «Поделиться» проверены на standard-сборках 22–23.11 (реестр §5). Установку фото-обновления в fixture шаг не повторял: само обновление доказано `test-android-update.mjs` 16–18.11, смысл шага — отсутствие канала в fdroid-редакции. После проверки на телефон возвращена standard-сборка; стенд, fixture и `adb reverse` погашены.

## Граница проверки

Живой телефонный прогон — выполнен выше на OPPO CPH1931. Остаётся: матрица CI на GitHub (проверится при следующем пуше в main — зеркало запускает workflow), realme 8/API 21 и кадры вёрстки fdroid-редакции не снимались (вёрстка не менялась — редакция отличается только отсутствием полосы обновления, что и показано кадрами).

## Решения

- Receiver оставлен общим (не исключён флейвором) — причина: lint MissingClass на манифестной ссылке; подавление линта сочтено хуже безобидного no-op.
- fdroid-APK не публикуется в GitHub-релизе — f-droid.org собирает из исходников сам (план 67, «Отклонённые варианты»).
