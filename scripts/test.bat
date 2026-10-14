@echo off
setlocal EnableExtensions

rem Release gate (plan 42, section B): run before every commit that changes VERSION,
rem and only after the whole gate run has passed on the same tree.

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
cd /d "%WYND_ROOT%"

echo [test] go vet ./...
go vet ./...
if errorlevel 1 exit /b 1

echo [test] go test ./...
go test ./...
if errorlevel 1 exit /b 1

echo [test] go build ./cmd/wynd
go build -o "%TEMP%\wynd-gate.exe" ./cmd/wynd
if errorlevel 1 exit /b 1
del /q "%TEMP%\wynd-gate.exe" 2>nul

echo [test] web check, check:ui, test, build
cd /d "%WYND_ROOT%\web"
call npm run check
if errorlevel 1 exit /b 1
call npm run check:ui
if errorlevel 1 exit /b 1
call npm run test
if errorlevel 1 exit /b 1
call npm run build
if errorlevel 1 exit /b 1

echo [test] OK
exit /b 0
