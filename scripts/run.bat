@echo off
setlocal EnableExtensions EnableDelayedExpansion

if /i "%~1"=="after_start" goto :after_start

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "BIN=%WYND_ROOT%\dist\wynd.exe"
set "DATA_DIR=%WYND_ROOT%\dev\data"
set "PUBLIC_URL=http://127.0.0.1:7676"
set "WYND_DATA_DIR=%DATA_DIR%"

call :stop_wynd
if errorlevel 1 exit /b 1

set "BIN_EXISTS=0"
if exist "%BIN%" set "BIN_EXISTS=1"

call :needs_rebuild
set "NEEDS_REBUILD=!ERRORLEVEL!"

if !NEEDS_REBUILD! neq 0 (
  if "!BIN_EXISTS!"=="0" (
    echo Binary missing - rebuilding...
  ) else (
    echo Binary outdated - rebuilding...
  )
  call "%~dp0build.bat"
  if errorlevel 1 exit /b 1
)

if not exist "%BIN%" (
  echo ERROR: Binary not found after build: %BIN%
  exit /b 1
)

echo Starting Wynd from %BIN%
echo Data dir: %DATA_DIR%
echo Close this window to stop the server.
echo.

set "FIRST_RUN=0"
if not exist "%DATA_DIR%\wynd.db" set "FIRST_RUN=1"

start "" /b "%ComSpec%" /v:on /c call "%~f0" after_start !FIRST_RUN!

"%BIN%"
exit /b %ERRORLEVEL%

:after_start
for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "DATA_DIR=%WYND_ROOT%\dev\data"
set "PUBLIC_URL=http://127.0.0.1:7676"
set "FIRST_RUN=%~2"
if not "%FIRST_RUN%"=="1" set "FIRST_RUN=0"

call :wait_health 60
if errorlevel 1 exit /b 1

call :post_start_actions
exit /b 0

:stop_wynd
tasklist /FI "IMAGENAME eq wynd.exe" 2>nul | find /I "wynd.exe" >nul
if errorlevel 1 exit /b 0

echo Stopping existing Wynd...
taskkill /IM wynd.exe /F >nul 2>&1

set /a WAIT_LEFT=15
:wait_port_free
call :port_in_use 7676
if errorlevel 1 exit /b 0
timeout /t 1 /nobreak >nul
set /a WAIT_LEFT-=1
if !WAIT_LEFT! leq 0 (
  echo ERROR: Port 7676 is still in use.
  exit /b 1
)
goto :wait_port_free

:needs_rebuild
if not exist "%BIN%" exit /b 1
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0binary-stale.ps1" -Binary "%BIN%" -Root "%WYND_ROOT%"
exit /b %ERRORLEVEL%

:port_in_use
netstat -an | findstr /I "LISTENING" | findstr /R /C:":%~1 " /C:":%~1]" >nul 2>&1
exit /b %ERRORLEVEL%

:wait_health
set /a WAIT_LEFT=%~1
if !WAIT_LEFT! leq 0 set /a WAIT_LEFT=60
:wait_health_loop
curl.exe -s -o nul -w "%%{http_code}" "%PUBLIC_URL%/health" 2>nul | findstr /x "200" >nul
if not errorlevel 1 exit /b 0
set /a WAIT_LEFT-=1
if !WAIT_LEFT! leq 0 exit /b 1
timeout /t 1 /nobreak >nul
goto :wait_health_loop

:post_start_actions
set "INSTANCE_JSON=%TEMP%\wynd-instance-%RANDOM%.json"
set "TOKEN_FILE=%DATA_DIR%\keys\bootstrap"
curl.exe -s "%PUBLIC_URL%/api/v1/instance" > "%INSTANCE_JSON%" 2>nul

set "SHOW_BOOTSTRAP=1"
if exist "%INSTANCE_JSON%" (
  findstr /C:"bootstrapped.:true" "%INSTANCE_JSON%" >nul 2>&1
  if not errorlevel 1 set "SHOW_BOOTSTRAP=0"
)

if "!SHOW_BOOTSTRAP!"=="1" (
  call :show_bootstrap_banner
  if defined TOKEN (
    powershell -NoProfile -Command "Start-Process '%PUBLIC_URL%/admin/bootstrap?token=!TOKEN!'"
  ) else (
    echo ERROR: Bootstrap token not found. See the server log ^(bootstrap URL:^).
  )
) else (
  start "" "%PUBLIC_URL%/"
)
exit /b 0

:show_bootstrap_banner
set "TOKEN="
set /a TOKEN_WAIT=15
:wait_token
if exist "%TOKEN_FILE%" (
  set /p TOKEN=<"%TOKEN_FILE%"
)
if defined TOKEN goto :print_bootstrap
set /a TOKEN_WAIT-=1
if !TOKEN_WAIT! leq 0 goto :print_bootstrap
timeout /t 1 /nobreak >nul
goto :wait_token

:print_bootstrap
echo.
echo ============================================================
if "!FIRST_RUN!"=="1" (
  echo   First run - create the admin account
) else (
  echo   Bootstrap not finished - create the admin account
)
echo ============================================================
echo.
if defined TOKEN (
  echo Bootstrap URL: %PUBLIC_URL%/admin/bootstrap?token=!TOKEN!
) else (
  echo Bootstrap URL is in the server log above ^(bootstrap URL:^).
)
echo.
echo ============================================================
echo.
exit /b 0
