param(
    [string]$EnvironmentRoot = 'D:\android-env',
    [string]$KeyStore = 'D:\Local\Git\myenv\wynd\wynd-android.p12',
    [string]$KeyAlias = 'wynd-android'
)

$ErrorActionPreference = 'Stop'
if (Test-Path -LiteralPath $KeyStore) { throw "Keystore already exists; refusing to overwrite: $KeyStore" }
$jdk = Get-ChildItem -LiteralPath (Join-Path $EnvironmentRoot 'jdk') -Directory |
    Where-Object { $_.Name -like 'jdk-17*' -and (Test-Path -LiteralPath (Join-Path $_.FullName 'bin\keytool.exe')) } |
    Sort-Object Name -Descending | Select-Object -First 1
if (-not $jdk) { throw 'Portable JDK 17 not found' }
New-Item -ItemType Directory -Path (Split-Path -Parent $KeyStore) -Force | Out-Null
Write-Host 'Введите пароль локально в keytool. Пароль не передаётся в аргументах и не записывается скриптом.'
& (Join-Path $jdk.FullName 'bin\keytool.exe') -genkeypair -storetype PKCS12 -keystore $KeyStore `
    -alias $KeyAlias -keyalg RSA -keysize 3072 -sigalg SHA256withRSA -validity 10000 -dname 'CN=Wynd Android'
if ($LASTEXITCODE -ne 0) { throw 'Android key generation failed' }
Write-Host 'Сохраните keystore, alias и пароль в надёжную резервную копию. Это постоянный ключ Android APK.'
