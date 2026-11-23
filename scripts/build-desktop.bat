@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Сборка окна "Wynd Настольный" (планы 55, 61 и 63): dist\wynd-desktop.exe.
rem Открытый ключ экосистемы вшивается в сборку: самообновление ставит только
rem подписанные выпуски.
rem
rem Wails v3 не читает wails.json, а "wails3 build" требует Taskfile, поэтому
rem шаги явные (план 64): сборка frontend (её dist вшивает main.go), ресурсы exe
rem (иконка, версия, манифест), затем go build.
rem Нужны Go, Node и Wails v3 CLI:
rem   go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
rem Лишние аргументы уходят в "go build" как есть, например  build-desktop.bat -a

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "APP_DIR=%WYND_ROOT%\cmd\wynd-desktop"
set "FRONTEND_DIR=%APP_DIR%\frontend"
set "SYSO=%APP_DIR%\wynd-desktop.syso"
set "BIN_DIR=%WYND_ROOT%\dist"

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
set "WAILS3=wails3"
where wails3 >nul 2>&1
if errorlevel 1 (
  for /f "delims=" %%G in ('go env GOPATH') do set "WAILS3=%%G\bin\wails3.exe"
  if not exist "!WAILS3!" (
    echo ERROR: Wails v3 CLI not found. Install it:
    echo   go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
    exit /b 1
  )
)

rem The frontend build runs out of memory on a small machine without a heap limit.
if not defined NODE_OPTIONS set "NODE_OPTIONS=--max-old-space-size=3072"

rem The windows compile shared web components; their TypeScript config extends
rem web/.svelte-kit, which is generated. On a clean clone generate it (and
rem install web dependencies if missing) before the frontend build.
if not exist "%WYND_ROOT%\web\node_modules\" (
  pushd "%WYND_ROOT%\web"
  call npm install --no-audit --no-fund
  set "BUILD_EXIT=!ERRORLEVEL!"
  popd
  if !BUILD_EXIT! neq 0 (
    echo Web dependencies install failed.
    exit /b !BUILD_EXIT!
  )
)
pushd "%WYND_ROOT%\web"
call npx svelte-kit sync
set "BUILD_EXIT=!ERRORLEVEL!"
popd
if !BUILD_EXIT! neq 0 (
  echo svelte-kit sync failed.
  exit /b !BUILD_EXIT!
)

echo Building frontend (npm install, npm run build)...
pushd "%FRONTEND_DIR%"
call npm install --no-audit --no-fund
set "BUILD_EXIT=!ERRORLEVEL!"
if !BUILD_EXIT! equ 0 (
  call npm run build
  set "BUILD_EXIT=!ERRORLEVEL!"
)
popd
if !BUILD_EXIT! neq 0 (
  echo Frontend build failed.
  exit /b !BUILD_EXIT!
)

rem main.go embeds frontend\dist; only a fallback for a build without one.
if not exist "%FRONTEND_DIR%\dist\" (
  mkdir "%FRONTEND_DIR%\dist"
  echo placeholder> "%FRONTEND_DIR%\dist\placeholder.txt"
)

rem web/embed.go вшивает клиент сервера (план 66, функция 9): exe собирается
rem только с готовым web/dist — ставим зависимости и собираем клиент здесь же.
pushd "%WYND_ROOT%\web"
if not exist "node_modules\" (
  echo Installing web dependencies for the embedded client...
  call npm ci
  if !ERRORLEVEL! neq 0 (
    popd
    echo web npm ci failed.
    exit /b 1
  )
)
echo Building web client (embedded)...
call npm run build
set "WEB_EXIT=!ERRORLEVEL!"
popd
if !WEB_EXIT! neq 0 (
  echo Web client build failed.
  exit /b !WEB_EXIT!
)

set "SIGNING_KEY="
if exist "%WYND_ROOT%\signing-key.pub" set /p SIGNING_KEY=<"%WYND_ROOT%\signing-key.pub"

echo Generating Windows resources (wails3 generate syso)...
pushd "%APP_DIR%"
"%WAILS3%" generate syso -arch amd64 -icon build\windows\icon.ico -manifest build\windows\wails.exe.manifest -info build\windows\info.json -out "%SYSO%"
set "BUILD_EXIT=!ERRORLEVEL!"
popd
if !BUILD_EXIT! neq 0 (
  if exist "%SYSO%" del /q "%SYSO%"
  echo wails3 generate syso failed.
  exit /b !BUILD_EXIT!
)

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"

echo Building Wynd Desktop (go build)...
pushd "%WYND_ROOT%"
if defined SIGNING_KEY (
  echo Signing key: !SIGNING_KEY!
  go build -trimpath -buildvcs=false -tags "desktop,production" -ldflags "-w -s -H windowsgui -X gitea.mixdep.ru/mix/wynd/internal/desktop.SigningKey=!SIGNING_KEY!" -o "%BIN_DIR%\wynd-desktop.exe" ./cmd/wynd-desktop %*
) else (
  echo No signing key: self-update will be off in this build.
  go build -trimpath -buildvcs=false -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o "%BIN_DIR%\wynd-desktop.exe" ./cmd/wynd-desktop %*
)
set "BUILD_EXIT=!ERRORLEVEL!"
popd

rem The .syso belongs to this build only: keep the tree clean afterwards.
if exist "%SYSO%" del /q "%SYSO%"

if !BUILD_EXIT! neq 0 (
  echo go build failed.
  exit /b !BUILD_EXIT!
)

echo.
echo Build complete: %BIN_DIR%\wynd-desktop.exe
exit /b 0
