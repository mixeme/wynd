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

## Граница проверки

Живой телефонный прогон fdroid-сборки (нет полосы обновления при fixture-выпуске, вход/чтение/шеринг/пуш работают) — шаг 2 плана 67, в этот шаг не входит. Матрица CI на GitHub — при следующем пуше.

## Решения

- Receiver оставлен общим (не исключён флейвором) — причина: lint MissingClass на манифестной ссылке; подавление линта сочтено хуже безобидного no-op.
- fdroid-APK не публикуется в GitHub-релизе — f-droid.org собирает из исходников сам (план 67, «Отклонённые варианты»).
