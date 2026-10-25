@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem Signing key for Workshop releases (plan 45). Self-update installs only an
rem exe signed with this key: Ed25519 over the whole file, base64 signature in
rem wynd-workshop.exe.sig next to the exe in the GitHub release.
rem
rem   sign-workshop.bat keygen          create the key (once); the private key is
rem                                     stored encrypted with your password, the
rem                                     public one goes to cmd\wynd-installer\signing-key.pub
rem   sign-workshop.bat workshop-vX.Y.Z download that draft release, check its sum
rem                                     and build attestation, sign the exe and
rem                                     upload wynd-workshop.exe.sig to the draft
rem   sign-workshop.bat file PATH.exe   sign a local exe (PATH.exe.sig next to it)
rem
rem gh is called with "call": installed as a .bat wrapper it would not return otherwise.
rem
rem The private key lives in %%USERPROFILE%%\.wynd\workshop-signing.key unless
rem WYND_SIGNING_KEY names another file. Keep a copy of it and of the password
rem somewhere safe: without them no Workshop that is already out there can be
rem updated again. Needs Go; the release command also needs gh (GitHub CLI).

for %%I in ("%~dp0..") do set "WYND_ROOT=%%~fI"
set "PUB=%WYND_ROOT%\cmd\wynd-installer\signing-key.pub"
if not defined WYND_SIGNING_KEY set "WYND_SIGNING_KEY=%USERPROFILE%\.wynd\workshop-signing.key"
set "REPO=mixeme/wynd"

if "%~1"=="" goto :usage

where go >nul 2>&1
if errorlevel 1 (
  echo ERROR: Go is not on PATH.
  exit /b 1
)

set "TOOL=%WYND_ROOT%\dist\workshop-sign.exe"
if not exist "%WYND_ROOT%\dist" mkdir "%WYND_ROOT%\dist"
pushd "%WYND_ROOT%"
go build -o "%TOOL%" ./cmd/workshop-sign
set "BUILD_EXIT=!ERRORLEVEL!"
popd
if !BUILD_EXIT! neq 0 (
  echo ERROR: could not build cmd\workshop-sign.
  exit /b 1
)

if /i "%~1"=="keygen" goto :keygen
if /i "%~1"=="file" goto :file
set "TAG=%~1"
if /i "!TAG:~0,10!"=="workshop-v" goto :release
goto :usage

:keygen
"%TOOL%" keygen "%WYND_SIGNING_KEY%" "%PUB%"
if errorlevel 1 exit /b 1
echo.
echo Next: commit cmd\wynd-installer\signing-key.pub. Every Workshop built after
echo that carries the key and can update itself to releases signed with it.
echo Back up %WYND_SIGNING_KEY% and remember the password.
exit /b 0

:file
if "%~2"=="" goto :usage
if not exist "%~f2" (
  echo ERROR: %~f2 not found.
  exit /b 1
)
call :sign "%~f2"
exit /b !ERRORLEVEL!

:release
where gh >nul 2>&1
if errorlevel 1 (
  echo ERROR: gh ^(GitHub CLI^) is not on PATH.
  exit /b 1
)
set "DIR=%WYND_ROOT%\dist\release-%TAG%"
if not exist "%DIR%" mkdir "%DIR%"
echo Downloading draft %TAG%...
call gh release download "%TAG%" -R %REPO% -D "%DIR%" -p wynd-workshop.exe -p wynd-workshop.exe.sha256 --clobber
if errorlevel 1 (
  echo ERROR: could not download release %TAG%.
  exit /b 1
)
set "EXE=%DIR%\wynd-workshop.exe"
rem The sum written by the build must match the downloaded file.
for /f "usebackq delims=" %%H in (`powershell -NoProfile -Command "(Get-FileHash -Algorithm SHA256 '%EXE%').Hash.ToLower()"`) do set "GOT=%%H"
set /p WANT=<"%DIR%\wynd-workshop.exe.sha256"
for /f "tokens=1" %%W in ("!WANT!") do set "WANT=%%W"
if /i not "!GOT!"=="!WANT!" (
  echo ERROR: downloaded exe does not match the sum in the release.
  exit /b 1
)
echo SHA-256 matches the release: !GOT!
rem The exe must be built by GitHub Actions from this very tag.
call gh attestation verify "%EXE%" --repo %REPO% --source-ref "refs/tags/%TAG%"
if errorlevel 1 (
  echo ERROR: build attestation does not match %TAG% - not signing.
  exit /b 1
)
call :sign "%EXE%"
if errorlevel 1 exit /b 1
call gh release upload "%TAG%" "%EXE%.sig" -R %REPO% --clobber
if errorlevel 1 (
  echo ERROR: could not upload the signature to %TAG%.
  exit /b 1
)
echo.
echo Signed %TAG%: wynd-workshop.exe.sig is in the draft. The draft is NOT
echo published: add "what's new" lines and publish it on GitHub.
exit /b 0

:sign
if not exist "%PUB%" (
  echo ERROR: %PUB% not found - run "sign-workshop.bat keygen" first.
  exit /b 1
)
"%TOOL%" sign "%WYND_SIGNING_KEY%" "%~1"
if errorlevel 1 exit /b 1
rem Check against the public key in the repository: the one builds carry.
"%TOOL%" verify "%PUB%" "%~1"
if errorlevel 1 (
  echo ERROR: the key does not match cmd\wynd-installer\signing-key.pub.
  exit /b 1
)
exit /b 0

:usage
echo Usage:
echo   sign-workshop.bat keygen
echo   sign-workshop.bat workshop-vX.Y.Z
echo   sign-workshop.bat file PATH.exe
exit /b 1
