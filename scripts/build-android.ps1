param(
    [string]$EnvironmentRoot = 'D:\android-env',
    [string]$ServerUrl = 'http://127.0.0.1:7676',
    [string]$UpdateFixtureOrigin = '',
    [string]$VersionName = '',
    [switch]$EmbeddedUi,
    [string[]]$Tasks = @('testStandardDebugUnitTest', 'lintStandardDebug', 'assembleStandardDebug')
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$jdkRoot = Join-Path $EnvironmentRoot 'jdk'
$jdk = Get-ChildItem -LiteralPath $jdkRoot -Directory |
    Where-Object { $_.Name -like 'jdk-17*' -and (Test-Path -LiteralPath (Join-Path $_.FullName 'bin\javac.exe')) } |
    Sort-Object Name -Descending | Select-Object -First 1
if (-not $jdk) { throw "Portable JDK 17 not found in $jdkRoot" }
$wrapper = Join-Path $projectRoot 'android\gradlew.bat'
if (-not (Test-Path -LiteralPath $wrapper)) { throw "Gradle wrapper not found: $wrapper" }

$variables = @('JAVA_HOME', 'PATH', 'ANDROID_HOME', 'ANDROID_USER_HOME', 'GRADLE_USER_HOME')
$saved = @{}
foreach ($variable in $variables) {
    $saved[$variable] = [Environment]::GetEnvironmentVariable($variable, 'Process')
}
try {
    $env:JAVA_HOME = $jdk.FullName
    $env:ANDROID_HOME = Join-Path $EnvironmentRoot 'sdk'
    $env:ANDROID_USER_HOME = Join-Path $EnvironmentRoot 'user'
    $env:GRADLE_USER_HOME = Join-Path $EnvironmentRoot 'gradle-user-home'
    $env:PATH = (Join-Path $jdk.FullName 'bin') + ';' + $env:PATH
    $versionArguments = @()
    if ($VersionName) { $versionArguments += "-PwyndVersionName=$VersionName" }
    if ($EmbeddedUi) { $versionArguments += '-PwyndEmbeddedUi=true' }
    & $wrapper --no-daemon --console=plain -p (Join-Path $projectRoot 'android') `
        --project-cache-dir (Join-Path $EnvironmentRoot 'cache\wynd-project') `
        "-PwyndEnvironmentRoot=$EnvironmentRoot" "-PwyndDevServerUrl=$ServerUrl" `
        "-PwyndUpdateFixtureOrigin=$UpdateFixtureOrigin" @versionArguments @Tasks
    if ($LASTEXITCODE -ne 0) { throw "Android build failed (exit $LASTEXITCODE)" }
} finally {
    foreach ($variable in $variables) {
        [Environment]::SetEnvironmentVariable($variable, $saved[$variable], 'Process')
    }
}
