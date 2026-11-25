# 67 — Выпуск Android-приложения в F-Droid

**Тип:** новая задача (обсуждение 23.11.2026): «Что нужно, чтобы выложить наше приложение в F-Droid?»
**Статус:** план. Владелец 23.11.2026: репозиторий на GitHub публичный; интерфейс будет вшит (в перспективе); отдельную редакцию для F-Droid сделать можно. 25.11 владелец спросил, можно ли запускать процесс сейчас — черновики шагов 3–4 подготовлены; подача MR упирается в тег с fastlane-метаданными и проверку на телефоне.
**Обновлено:** 2026-11-25 (шаги 3–4: черновики метаданных; проверена сборка fdroid + вшитый клиент).

## Контекст и границы

- Цель — главный репозиторий f-droid.org: их сервер собирает из исходников и подписывает своим ключом; метаданные подаются MR в `gitlab.com/fdroid/fdroiddata`; очередь ревью первого приложения — месяцы.
- Источник для F-Droid — публичное GitHub-зеркало (read-only); канон остаётся Gitea, правило «пуш только в Gitea» не меняется.
- f-droid.org-сборка подписана ключом F-Droid: пользователи прямого APK (android-v0.1.0+) не перейдут без сноса и переустановки. Это надо сказать в описании приложения при подаче.
- Вшивание интерфейса в APK — отдельная будущая работа владельца. **23.11 решено делать по схеме Настольного (план 50): клиент в APK, по умолчанию серверный интерфейс, переключение — после стабилизации; подготовка не меняет текущие выпуски.** **Подготовка сделана 24.11** — [протокол](../testing/android-embedded-ui-2026-11-24.md). Сейчас интерфейс — JS с сервера; ревьюеры трактуют загружаемый JS как исполняемый код, а приложение — как «обёртку сайта». Подача в f-droid.org — после вшивания; шаги 1–3 от вшивания не зависят.

## Что уже в порядке (обследовано 23.11)

- LICENSE — AGPL-3.0.
- Зависимости APK свободные: androidx.webkit 1.14.0 и UnifiedPush connector 3.3.5 — Apache-2.0; `org.json` только в JVM-тестах (MIT с 2023).
- FCM/Play-сервисов нет — пуш через UnifiedPush.
- versionCode строится из versionName детерминированно; minSdk 21, targetSdk 36.
- Веб-клиент уже умеет `capabilities.updates: false` (`web/src/lib/android/update.svelte.ts:97` — полоса обновления не включается), поэтому заглушка обновления в fdroid-редакции не требует правок веба.

## Порядок работ

1. **Флейвор fdroid.** Product-флейворы `standard`/`fdroid` (dimension `distribution`). `standard` — как сейчас (самообновление из GitHub-выпусков). `fdroid` — реализация обновления (`UpdateController`, `UpdatePolicy`, `UpdateTransport`, `ReleaseCatalog`, `ApkVerifier`) не входит в APK: переехала в `src/standard/java`; `InstallResultReceiver` остаётся общим — lint MissingClass на манифестной ссылке, а receiver безобиден (только пересылает результат установщика контроллеру; заглушка отвечает no-op `installationResult`). В `src/fdroid/java` — заглушка `UpdateController` с тем же API, `status()` отвечает `updates: false` (веб сам прячет полосу). Контроль парности API — компилятором: общие `AndroidBridge` и `MainActivity` без правок, расхождение ломает сборку fdroid. Манифест `src/fdroid/AndroidManifest.xml` убирает `REQUEST_INSTALL_PACKAGES`. Тесты обновления — в `src/testStandard/java`; охраняющий тест заглушки — в `src/testFdroid/java` (набор ключей status и `updates: false`). CI `android.yaml` и `scripts/build-android.ps1` — на флейворные задачи; CI кладёт fdroid-APK в артефакт, но в черновик релиза он не попадает (f-droid.org собирает сам). README — новые задачи и пути. **Сделано 23.11** (ворота локально: тесты+lint+сборка обеих редакций — зелёные; [протокол](../testing/android-fdroid-flavor-2026-11-23.md)). Особенность истории: переносы файлов в `src/standard`/`testStandard` и обратный перенос receiver попали в коммиты `35137475` и `d79f25f0` параллельных сессий (общий индекс: их `git commit` забрал staged `git mv`); дерево в этих коммитах корректно, переписывание не потребовалось.
2. **Живая проверка флейворов на телефоне.** fdroid-debug: полосы обновления нет даже на fixture-выпуске, вход/чтение работают; standard-debug — без изменений. Прогон `test-android-update.mjs` остаётся на standard (заглушка fdroid сценарий самообновления не проходит по определению). **Сделано 23.11 на OPPO CPH1931**: обе редакции с одним fixture-каталогом (`mature`, 0.1.1); standard — `updates:true`, `available:true`, полоса «Вышла версия 0.1.1» на экране входа; fdroid — `updates:false`, `check` отклонён, полосы нет на входе и в ленте, вход Ани и список кругов работают; кадры и границы — [протокол](../testing/android-fdroid-flavor-2026-11-23.md#живая-проверка-на-oppo-шаг-2-плана-67). Шеринг и пуш не перезапускались: их код общий и не менялся.
3. **Метаданные fastlane.** `android/fastlane/metadata/android/en-US/`: title, summary, full_description (AGPL; подпись ключом F-Droid и переход с прямого APK через снос/переустановку), changelogs/1000.txt, иконка, скриншоты. Скриншоты снимаются с телефона — кадры на приёмку владельца до подачи. **Черновик 25.11** (`3b5d33d2`): title, short/full description, значок 512×512 из `web/static/icon-512.png`; английский текст ждёт приёмки. Осталось: кадры (нужен телефон — в сессии 25.11 adb пуст), английский текст на приёмку. Changelog в 0.1.3 не попал (тег сделан до черновика) — карточка 0.1.3 будет без истории изменений; `changelogs/1004.txt` добавить со следующим выпуском.
4. **YAML fdroiddata и MR.** В `gitlab.com/fdroid/fdroiddata`: `mixeme.wynd.yml` — источник GitHub, сборка флейвора fdroid. Подача — после вшивания интерфейса; черновик YAML готовить заранее. **Черновик 25.11** (`413b75a8`), **проверен 25.11** на fdroidserver 2.4.5 (venv, Windows): `fdroid lint mixeme.wynd` — без замечаний, `fdroid rewritemeta` — файл уже в каноническом виде (наша копия и есть вывод rewritemeta; комментарии в fdroiddata нельзя — CI перезаписывает файл). Что зафиксировано и почему:
   - `subdir: android`, `gradle: [fdroid]`, `gradleprops: [wyndEmbeddedUi=true]` — редакция fdroid со вшитым клиентом. **Сборка `assembleFdroidRelease -PwyndEmbeddedUi=true` локально 25.11 прошла** (APK 7,46 МБ, 387 файлов клиента в `assets/www`).
   - `output: app/build/outputs/apk/fdroid/release/app-fdroid-release-unsigned.apk` — у AGP APK лежит в модуле `app/`, а fdroidserver ищет только `build/outputs/apk` от `subdir`; без `output` сборка бы упала на «Failed to find any output apks».
   - `prebuild: cd ../web && npm ci` — prebuild выполняется в `subdir` (проверено по коду `prepare_source`), зависимости веб-клиента в исходниках не лежат.
   - `scanignore: web/node_modules` — поставить заранее; на первой сборке проверить, нужен ли (если сканер молчит, убрать).
   - `AutoUpdateMode: None` + `UpdateCheckMode: Tags` были бы красными: номер сборки вычисляется в Gradle из versionName, в манифесте и файлах его нет — `checkupdates` не найдёт версию. Следующий шаг: добавить `versionCode` в `android/version.properties` с тестом на совпадение с формулой и отдельным MR переключить на `UpdateCheckData: android/version.properties|versionCode=(\d+)|android/version.properties|versionName=([0-9.]+)`, `AutoUpdateMode: Version`, `UpdateCheckMode: Tags ^android-v[0-9]+\.[0-9]+\.[0-9]+$`.
   - Трекер — issues GitHub-зеркала: Gitea отдаёт 404 гостям.
   - `commit: android-v0.1.3` — тег уже есть на GitHub-зеркале и содержит fastlane-метаданные (проверено `git ls-remote` и `git ls-tree`).
   **Текст MR для fdroiddata:**
   > **New app: Wynd.** Wynd is a private journal for a close circle; the Android app is a client for the user's own self-hosted Wynd server (server and client are AGPL-3.0 in one repository). There is no service of our own and no default instance.
   > - Only the `fdroid` flavor is submitted: the self-updater and `REQUEST_INSTALL_PACKAGES` are not in this build; the `standard` flavor published on GitHub releases is not submitted.
   > - The flavor is built with `-PwyndEmbeddedUi=true`: the web client is bundled in the APK and served from assets, so the app does not fetch its UI or code from the network — it only talks to the user's server API. Without the flag the app would load its UI from the server, which is not acceptable for inclusion.
   > - `prebuild: cd ../web && npm ci` installs JS build dependencies (Vite/SvelteKit) for the bundled client.
   > - No Google Play services, no FCM, no tracking; push notifications are optional and use UnifiedPush with a distributor chosen by the user.
   > - Version code is derived in Gradle from versionName (0.1.3 → 1003), so auto-update is not enabled yet (`UpdateCheckMode: None`); a follow-up MR will add a versionCode line to `android/version.properties` and switch to Tags/Version.
   **Осталось до подачи:** приёмка английского текста и `AuthorName`, проверка режима embedded у fdroid-сборки на телефоне (adb 25.11 пуст), сам MR (владелец, аккаунт GitLab есть). Кадры для карточки можно добавить отдельным MR в `metadata/mixeme.wynd/<locale>/images/` — они не требуют нового тега.
5. **Слежение.** Очередь ревью f-droid.org — периодический присмотр; вопросы ревьюеров фиксировать здесь. Свой подписанный F-Droid-репозиторий не делаем, пока главный репозиторий не отказал и владелец не попросит.

Выполнение не обязано идти строго по номерам, но 1–2 раньше 3–4; подача (4) — после вшивания интерфейса.

## Отклонённые варианты

- **Переключатель `BuildConfig.SELF_UPDATE`** с оставлением кода обновления в fdroid-APK — отклонён: мёртвый код загрузки APK в сборке для f-droid.org — риск на ревью; исключение классов флейвором чище и безопаснее.
- **Публикация fdroid-APK в GitHub-релизе** — отклонена: f-droid.org собирает сам; второй APK в релизе путает прямых пользователей.

## Правила этого плана

- Каждый законченный шаг — отдельный коммит; посторонние изменения в коммит не входят.
- Ворота шага 1 — локальный прогон `scripts\build-android.ps1` с полным флейворным набором (тесты+lint+сборка обеих редакций), тот же набор, что в CI.
- Незакоммиченная правка `web/src/routes/invite/[token]/+page.svelte` — недоделка плана 66 функции 4 («Открыть в приложении», ждёт кадра и приёмки) — в этот план не входит и в коммиты не берётся.
