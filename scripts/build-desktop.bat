@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Сборка окна Wynd Настольный (планы 55, 61 и 63). Открытый ключ экосистемы
rem вшивается в сборку: самообновление ставит только подписанные выпуски.
for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"

cd /d "%WYND_ROOT%\cmd\wynd-desktop\frontend"
if not exist "node_modules\" (
  echo Installing desktop frontend dependencies...
  call npm ci
  if errorlevel 1 exit /b 1
)
echo Building desktop shell frontend...
call npm run build
if errorlevel 1 exit /b 1

cd /d "%WYND_ROOT%"
if not exist "dist" mkdir "dist"
set "SIGNING_KEY="
if exist "%WYND_ROOT%\signing-key.pub" set /p SIGNING_KEY=<"%WYND_ROOT%\signing-key.pub"
echo Building dist\wynd-desktop.exe...
if defined SIGNING_KEY (
  echo Signing key: !SIGNING_KEY!
  go build -trimpath -tags "desktop,production" -ldflags "-H windowsgui -s -w -X gitea.mixdep.ru/mix/wynd/internal/desktop.SigningKey=!SIGNING_KEY!" -o "dist\wynd-desktop.exe" .\cmd\wynd-desktop
) else (
  echo No signing key: self-update will be off in this build.
  go build -trimpath -tags "desktop,production" -ldflags "-H windowsgui -s -w" -o "dist\wynd-desktop.exe" .\cmd\wynd-desktop
)
if errorlevel 1 exit /b 1
echo Build complete: %WYND_ROOT%\dist\wynd-desktop.exe
exit /b 0
