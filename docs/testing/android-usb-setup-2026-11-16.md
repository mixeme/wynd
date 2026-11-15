# Android: подключение по USB — 2026-11-16

## Результат

Владелец разрешил подготовить ADB и подтвердил USB-отладку на телефоне.
`adb devices -l` показал статус `device`; команды `shell getprop` выполнились.

- Производитель и модель: OPPO CPH1931 (`OP4B79L1`, продукт `CPH1931RU`).
- Android 10, API 29.
- В плане 50 указан realme 8. Текущее подключение OPPO не закрывает проверки на realme.
- Драйвер не устанавливался: ADB обнаружил телефон сразу.

## Инструменты

- Официальный архив: https://dl.google.com/android/repository/platform-tools-latest-windows.zip
- Сведения о выпуске: https://developer.android.com/tools/releases/platform-tools
- Архив сохранён в `D:\android-env\downloads\platform-tools-latest-windows.zip`.
- SHA-256: `45F4D63113E895EBDE0C90F194099A4676B6AC653BD28D54314A9E022BBC1A99`.
- Распакован в `D:\android-env\sdk\platform-tools`.
- ADB: 1.0.41, Platform Tools 37.0.1 (`15733141`).
- Ключ USB-отладки создан ADB в `%USERPROFILE%\.android`; это разрешённое планом исключение для USB-отладки. Приватный ключ в репозиторий не копировать.
- Системные переменные среды и существующая Java не менялись.
- JDK 17, SDK для компиляции и Gradle пока не установлены. APK не собирался и не устанавливался; приложение и его интерфейс не проверялись.

## Повторное подключение

В обычном PowerShell от имени того же пользователя:

```powershell
$env:ANDROID_USER_HOME = 'D:\android-env\user'
$env:ANDROID_HOME = 'D:\android-env\sdk'
& 'D:\android-env\sdk\platform-tools\adb.exe' devices -l
```

Выбирать устройство по серийному номеру из актуального вывода `devices -l`,
если подключено несколько телефонов. Нормальное состояние — `device`.
При `unauthorized` разблокировать телефон и подтвердить запрос USB-отладки.

В ограниченной оболочке Codex вызов ADB без повышения завершился ошибкой
`Cannot mkdir '\.android': Permission denied`; повторный вызов с разрешением
выполнения вне песочницы подтвердил соединение. Эта ошибка сама по себе
не означает потерю разрешения на телефоне или необходимость драйвера.
