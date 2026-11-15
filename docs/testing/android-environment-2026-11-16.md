# Android: переносная среда — 2026-11-16

Установлена по поручению владельца в `D:\android-env`, без установщиков
и изменений системных переменных или существующего Liberica JDK 11.

| Инструмент | Версия | Путь от `D:\android-env` |
|---|---|---|
| Liberica JDK | 17.0.21.0.3+5 LTS, Windows x64 | `jdk/jdk-17.0.21.0.3` |
| SDK Command-line Tools | 19.0, архив 13114758 | `sdk/cmdline-tools/19.0` |
| Android SDK Platform | API 36, revision 2 | `sdk/platforms/android-36` |
| SDK Build Tools | 35.0.0 | `sdk/build-tools/35.0.0` |
| Gradle | 8.13 | `gradle/gradle-8.13` |
| Platform Tools | 37.0.1 | `sdk/platform-tools` |

Совместимость AGP 8.13.2, Gradle 8.13 и JDK 17 сверена по
[официальной таблице Android](https://developer.android.com/build/releases/agp-8-13-0-release-notes).
Эмулятор, NDK и Android Studio не устанавливались. Для SDK Platform и Build
Tools принята стандартная лицензия Android SDK во время установки sdkmanager.

## Происхождение архивов

- JDK — официальный BellSoft Product Discovery API, запрос `version-feature=17`,
  `version-modifier=latest`, `os=windows`, `arch=x86`, `bitness=64`,
  `package-type=zip`, `bundle-type=jdk`; выбран публичный PSU.
  Архив: `https://github.com/bell-sw/Liberica/releases/download/17.0.21.0.3+5/bellsoft-jdk17.0.21.0.3+5-windows-amd64.zip`.
  SHA-1 сверена с API: `6c9839bdfafa923cb1f70f6df0a1734fa15b5e3f`.
  SHA-256 загруженного архива: `7605b0e9c15d1b740b3b80ca25121c50dc60f5069c0f890cee7b48d2111fc486`.
- Command-line Tools — `https://dl.google.com/android/repository/commandlinetools-win-13114758_latest.zip`.
  SHA-1 сверена с официальным `repository2-1.xml` для `cmdline-tools;19.0`:
  `54a582f3bf73e04253602f2d1c80bd5868aac115`.
  SHA-256 загруженного архива: `98b565cb657b012dae6794cefc0f66ae1efb4690c699b78a614b4a6a3505b003`.
- Gradle — `https://services.gradle.org/distributions/gradle-8.13-bin.zip`.
  SHA-256 сверена с официальным `.sha256`:
  `20f1b1176237254a6fc204d8434196fa11a4cfb387567519c61556e8710aed78`.
- SDK Platform и Build Tools установлены официальным `sdkmanager` из репозитория Google.

Архивы и метаданные находятся в `downloads/`. SHA-256 JDK и Tools выше —
локальные идентификаторы фактически полученных файлов; проверка по серверу
для этих двух архивов выполнена по опубликованной SHA-1, а не по SHA-256.

## Проверки и расположение данных

`java -version`, `sdkmanager --version` и `gradle --version` выполнены успешно;
файлы `source.properties` подтверждают API 36 и Build Tools 35.0.0.

- Пользовательские данные SDK: `user/`.
- Кэш и загруженные wrapper-дистрибутивы Gradle: `gradle-user-home/`.
- Кэш Android-проекта: `cache/wynd-project/`.
- Отдельный отладочный ключ: `keys/wynd-debug.keystore`.
- Ключ разрешённой USB-отладки остаётся в `%USERPROFILE%\.android`, как в
  [первичной проверке ADB](android-usb-setup-2026-11-16.md).

Ни архивы, ни кэши, ни ключи не добавляются в репозиторий. Ключ выпуска
владельца ещё не выбран и не создавался. Протокол подготовки среды сам по
себе не подтверждает сборку APK, работу интерфейса или завершение этапа 1.
