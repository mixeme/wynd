@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Сборка окна "Wynd Настольный" (планы 55 и 61): dist\wynd-desktop.exe
rem (Wails пишет его в cmd\wynd-desktop\build\bin, скрипт кладёт рядом с сервером).
rem Собирать именно через wails build: он берёт иконку и описание из
rem cmd\wynd-desktop\build и вшивает их в exe — обычный go build этого не делает.
rem Нужны Go, Node и Wails CLI:
rem   go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
rem Лишние аргументы уходят в "wails build" как есть, например  build-desktop.bat -clean

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "APP_DIR=%WYND_ROOT%\cmd\wynd-desktop"

where go >nul 2>&1
if errorlevel 1 (
  echo ERROR: Go is not on PATH.
  exit /b 1
)

where npm >nul 2>&1
if errorlevel 1 (
  if exist "%ProgramFiles%\nodejs\npm.cmd" (
    set "PATH=%ProgramFiles%\nodejs;!PATH!"
  ) else (
    echo ERROR: npm is not on PATH.
    exit /b 1
  )
)

rem Wails is usually installed into GOPATH\bin, which is often not on PATH.
set "WAILS=wails"
where wails >nul 2>&1
if errorlevel 1 (
  for /f "delims=" %%G in ('go env GOPATH') do set "WAILS=%%G\bin\wails.exe"
  if not exist "!WAILS!" (
    echo ERROR: Wails CLI not found. Install it:
    echo   go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
    exit /b 1
  )
)

rem The frontend build runs out of memory on a small machine without a heap limit.
if not defined NODE_OPTIONS set "NODE_OPTIONS=--max-old-space-size=3072"

rem main.go embeds frontend\dist, which is not in git. Wails compiles the Go
rem code for its bindings before it builds the frontend, so on a fresh clone
rem (GitHub Actions) the embed has nothing to match. A placeholder is enough:
rem the frontend build replaces the folder's contents.
if not exist "%APP_DIR%\frontend\dist\" (
  mkdir "%APP_DIR%\frontend\dist"
  echo placeholder> "%APP_DIR%\frontend\dist\placeholder.txt"
)

echo Building Wynd Desktop (wails build)...
pushd "%APP_DIR%"
"%WAILS%" build -trimpath %*
set "BUILD_EXIT=!ERRORLEVEL!"
popd

if !BUILD_EXIT! neq 0 (
  echo wails build failed.
  exit /b !BUILD_EXIT!
)

rem Wails always writes into build\bin of its project; the result goes to dist
rem next to the server binary (scripts\build.bat).
set "BIN_DIR=%WYND_ROOT%\dist"
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
copy /y "%APP_DIR%\build\bin\wynd-desktop.exe" "%BIN_DIR%\wynd-desktop.exe" >nul
if errorlevel 1 (
  echo ERROR: could not copy wynd-desktop.exe to %BIN_DIR%.
  exit /b 1
)

echo.
echo Build complete: %BIN_DIR%\wynd-desktop.exe
exit /b 0
