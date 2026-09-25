@echo off
setlocal EnableExtensions

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
cd /d "%WYND_ROOT%"

echo [test-integration] go test -tags=integration ./internal/mail/ -count=1 -timeout 5m
go test -tags=integration ./internal/mail/ -count=1 -timeout 5m
if errorlevel 1 exit /b 1

echo [test-integration] OK
exit /b 0
