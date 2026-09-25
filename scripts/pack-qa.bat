@echo off
setlocal EnableExtensions EnableDelayedExpansion

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "VERSION_FILE=%WYND_ROOT%\VERSION"
set "MANUAL_SRC=%WYND_ROOT%\docs\testing\qa-manual.md"
set "STAGE=%WYND_ROOT%\dist\qa-stage"
set "OUT_DIR=%WYND_ROOT%\dist"

if not exist "%VERSION_FILE%" (
  echo ОШИБКА: не найден VERSION
  exit /b 1
)
set /p PRODUCT_VERSION=<"%VERSION_FILE%"
if not defined PRODUCT_VERSION (
  echo ОШИБКА: VERSION пуст
  exit /b 1
)

if not exist "%MANUAL_SRC%" (
  echo ОШИБКА: не найден %MANUAL_SRC%
  exit /b 1
)

echo Сборка бинарника...
call "%~dp0build.bat"
if errorlevel 1 exit /b 1

set "BIN=%OUT_DIR%\wynd.exe"
if not exist "%BIN%" (
  echo ОШИБКА: после сборки нет %BIN%
  exit /b 1
)

set "ZIP_NAME=wynd-qa-%PRODUCT_VERSION%.zip"
set "ZIP_PATH=%OUT_DIR%\%ZIP_NAME%"

if exist "%STAGE%" rmdir /s /q "%STAGE%"
mkdir "%STAGE%\scripts" >nul 2>&1
mkdir "%STAGE%\dist" >nul 2>&1
mkdir "%STAGE%\dev\data" >nul 2>&1
if errorlevel 1 (
  echo ОШИБКА: не удалось создать %STAGE%
  exit /b 1
)

copy /y "%BIN%" "%STAGE%\dist\wynd.exe" >nul
copy /y "%WYND_ROOT%\scripts\run.bat" "%STAGE%\scripts\run.bat" >nul
copy /y "%WYND_ROOT%\scripts\binary-stale.ps1" "%STAGE%\scripts\binary-stale.ps1" >nul
copy /y "%MANUAL_SRC%" "%STAGE%\qa-manual.md" >nul

if exist "%ZIP_PATH%" del /f /q "%ZIP_PATH%"

echo Упаковка %ZIP_NAME%...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "Compress-Archive -LiteralPath '%STAGE%\scripts','%STAGE%\dist','%STAGE%\dev','%STAGE%\qa-manual.md' -DestinationPath '%ZIP_PATH%' -Force"
if errorlevel 1 (
  echo ОШИБКА: не удалось создать архив
  exit /b 1
)

rmdir /s /q "%STAGE%"

echo.
echo Готово: %ZIP_PATH%
echo Структура как в репозитории: scripts\run.bat, dist\wynd.exe, dev\data\, qa-manual.md
exit /b 0
