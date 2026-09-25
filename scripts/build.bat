@echo off
setlocal EnableExtensions EnableDelayedExpansion

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"

call :stop_port 5173
if errorlevel 1 exit /b 1

where go >nul 2>&1
if errorlevel 1 (
  echo ERROR: Go is not on PATH.
  exit /b 1
)

where npm >nul 2>&1
if errorlevel 1 (
  echo ERROR: npm is not on PATH.
  exit /b 1
)

if not exist "%WYND_ROOT%\web\node_modules\" (
  echo Installing web dependencies...
  pushd "%WYND_ROOT%\web"
  call npm ci
  set "NPM_EXIT=!ERRORLEVEL!"
  popd
  if !NPM_EXIT! neq 0 exit /b !NPM_EXIT!
)

echo Building web (vite build)...
pushd "%WYND_ROOT%\web"
call npm run build
if errorlevel 1 (
  popd
  echo vite build failed.
  exit /b 1
)
popd

set "BIN_DIR=%WYND_ROOT%\dist"
set "BIN=%BIN_DIR%\wynd.exe"
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
echo Building Go binary: %BIN%
pushd "%WYND_ROOT%"
go build -ldflags="-s -w" -o "%BIN%" ./cmd/wynd
set "BUILD_EXIT=!ERRORLEVEL!"
popd

if !BUILD_EXIT! neq 0 (
  echo go build failed.
  exit /b !BUILD_EXIT!
)

echo.
echo Build complete: %BIN%
exit /b 0

:stop_port
set "STOP_PORT=%~1"
netstat -ano | findstr /I "LISTENING" | findstr /R /C:":%STOP_PORT% " /C:":%STOP_PORT%]" >nul 2>&1
if errorlevel 1 exit /b 0

echo Stopping process on port %STOP_PORT% ^(Vite dev^)...
for /f "tokens=5" %%P in ('netstat -ano ^| findstr /I "LISTENING" ^| findstr /R /C:":%STOP_PORT% " /C:":%STOP_PORT%]"') do (
  if not "%%P"=="0" taskkill /PID %%P /F >nul 2>&1
)

set /a WAIT_LEFT=10
:stop_port_wait
netstat -ano | findstr /I "LISTENING" | findstr /R /C:":%STOP_PORT% " /C:":%STOP_PORT%]" >nul 2>&1
if errorlevel 1 exit /b 0
timeout /t 1 /nobreak >nul
set /a WAIT_LEFT-=1
if !WAIT_LEFT! leq 0 (
  echo ERROR: Port %STOP_PORT% is still in use.
  exit /b 1
)
goto :stop_port_wait
