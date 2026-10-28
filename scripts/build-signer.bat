@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Builds "Wynd Signer" (plan 45, item 3a; frames 12a-12d): dist\wynd-signer.exe,
rem the owner's window for the Workshop signing key and release signing.
rem (Wails writes it into cmd\wynd-signer\build\bin, the script copies it to dist).
rem It talks to GitHub through gh (GitHub CLI), like scripts\sign-workshop.bat.
rem Needs Go, Node and the Wails CLI:
rem   go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
rem Extra arguments go to "wails build" as is, e.g.  build-signer.bat -clean

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "APP_DIR=%WYND_ROOT%\cmd\wynd-signer"

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

echo Building Wynd Signer (wails build)...
pushd "%APP_DIR%"
"%WAILS%" build %*
set "BUILD_EXIT=!ERRORLEVEL!"
popd

if !BUILD_EXIT! neq 0 (
  echo wails build failed.
  exit /b !BUILD_EXIT!
)

set "BIN_DIR=%WYND_ROOT%\dist"
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
copy /y "%APP_DIR%\build\bin\wynd-signer.exe" "%BIN_DIR%\wynd-signer.exe" >nul
if errorlevel 1 (
  echo ERROR: could not copy wynd-signer.exe to %BIN_DIR%.
  exit /b 1
)

echo.
echo Build complete: %BIN_DIR%\wynd-signer.exe
exit /b 0
