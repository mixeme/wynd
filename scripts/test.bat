@echo off
setlocal EnableExtensions

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
cd /d "%WYND_ROOT%"

echo [test] go test ./...
go test ./...
if errorlevel 1 exit /b 1

echo [test] web check, check:ui, test
cd /d "%WYND_ROOT%\web"
call npm run check
if errorlevel 1 exit /b 1
call npm run check:ui
if errorlevel 1 exit /b 1
call npm run test
if errorlevel 1 exit /b 1

echo [test] OK
exit /b 0
