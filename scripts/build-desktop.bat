@echo off
setlocal EnableExtensions

rem Сборка окна Wynd для компьютера (планы 55 и 61). Подпись и самообновление
rem пока не входят: выпуск — отдельная задача.
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
echo Building dist\wynd-desktop.exe...
go build -trimpath -tags "desktop,production" -ldflags "-H windowsgui -s -w" -o "dist\wynd-desktop.exe" .\cmd\wynd-desktop
if errorlevel 1 exit /b 1
echo Build complete: %WYND_ROOT%\dist\wynd-desktop.exe
exit /b 0
