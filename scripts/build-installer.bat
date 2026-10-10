@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Builds the desktop installer (plan 45): cmd\wynd-installer\build\bin\wynd-installer.exe
rem Needs Go, Node and the Wails CLI:
rem   go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
rem Extra arguments go to "wails build" as is, e.g.  build-installer.bat -clean

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "APP_DIR=%WYND_ROOT%\cmd\wynd-installer"

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

echo Building installer (wails build)...
pushd "%APP_DIR%"
"%WAILS%" build %*
set "BUILD_EXIT=!ERRORLEVEL!"
popd

if !BUILD_EXIT! neq 0 (
  echo wails build failed.
  exit /b !BUILD_EXIT!
)

echo.
echo Build complete: %APP_DIR%\build\bin\wynd-installer.exe
exit /b 0
