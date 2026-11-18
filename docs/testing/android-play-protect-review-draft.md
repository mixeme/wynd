# Черновик обращения в Play Защиту

18 ноября 2026 года владелец подтвердил отправку обращения. Результат
пересмотра неизвестен. Ниже сохранён исходный черновик для заблокированного
**debug APK** `mixeme.wynd.debug` версии 0.1.1-debug. В форме для описания
действует ограничение в 1000 символов.

Официальный порядок пересмотра:
[Google Play Protect — Appeals](https://developers.google.com/android/play-protect/warning-dev-guidance#appeals).
[Форма Google](https://support.google.com/googleplay/android-developer/contact/protectappeals)
запрашивает контактный email, имя пакета, SHA-256 APK, загруженного в VirusTotal,
и описание. Точный файл: `D:\android-env\test-releases\candidate-0.1.1.apk`.

## Текст

Subject: Request to review Play Protect classification of a Wynd Android development APK

Please review the Play Protect classification of the following local development build:

- App label: Wynd
- Package: mixeme.wynd.debug
- Version: 0.1.1-debug (versionCode 1001)
- APK SHA-256: c794b05c6368e37a4e7b6356fb591d4a3e066fd694880b31d33276006f7e0e47
- Signing certificate SHA-256: db4db546603ca5e950535681ba9469173f9b112e839cfb6acc573071c9193017
- Observed device: OPPO CPH1931, Android 10 / API 29
- Observed verdict after consenting to the recommended scan on 16 November 2026:
  POTENTIALLY_UNWANTED, threatType generic_malware.

Wynd is an open-source client for private shared journals. This build is a
development WebView wrapper used to test application updates. Its manifest
declares INTERNET and REQUEST_INSTALL_PACKAGES. It does not declare SMS,
accessibility, notification-listener or device-administration access.

REQUEST_INSTALL_PACKAGES is used to update this same application through
Android PackageInstaller after the user selects the update action. The
application checks the APK checksum, signing certificate, package name and
higher version before requesting installation. Android retains its installation
confirmation. This development test uses a local release-format fixture through
ADB reverse; the target interface and fixture addresses are loopback addresses.

The APK signature was verified with Android apksigner. Inspection of this APK
found no nested APK/JAR files, assets directory or native libraries. These checks
have not identified the specific reason for the classification. The warning
and its expanded explanation only state that the application may be dangerous.
Could you review the classification and, where possible, identify the behavior
that needs to be corrected?

Development source repository: https://github.com/mixeme/wynd
The submitted APK was a local development build. The current repository also
contains later updater fixes and should not be treated as an exact source
snapshot of the submitted binary. The APK hash above identifies the file to review.

## Протокол прогона

Описание проверки, снимок экрана и сведения о подписи сохранены в
[протоколе](android-updates-2026-11-16.md).
