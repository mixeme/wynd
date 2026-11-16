param(
    [Parameter(Mandatory = $true)][ValidatePattern('^android-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')][string]$Tag,
    [Parameter(Mandatory = $true)][string]$KeyStore,
    [string]$KeyAlias = 'wynd-android',
    [string]$EnvironmentRoot = 'D:\android-env',
    [switch]$Upload,
    [switch]$Publish
)

$ErrorActionPreference = 'Stop'
$repo = 'mixeme/wynd'
$version = $Tag.Substring('android-v'.Length)
$parts = $version.Split('.') | ForEach-Object { [long]$_ }
$expectedCode = $parts[0] * 1000000 + $parts[1] * 1000 + $parts[2]
if ($parts[1] -gt 999 -or $parts[2] -gt 999 -or $expectedCode -le 0 -or $expectedCode -gt 2100000000) { throw 'Android version overflow' }
if (-not (Test-Path -LiteralPath $KeyStore -PathType Leaf)) { throw 'Android keystore not found' }
$jdk = Get-ChildItem -LiteralPath (Join-Path $EnvironmentRoot 'jdk') -Directory |
    Where-Object { $_.Name -like 'jdk-17*' -and (Test-Path -LiteralPath (Join-Path $_.FullName 'bin\java.exe')) } |
    Sort-Object Name -Descending | Select-Object -First 1
if (-not $jdk) { throw 'Portable JDK 17 not found' }
$tools = Join-Path $EnvironmentRoot 'sdk\build-tools\35.0.0'
$directory = Join-Path $EnvironmentRoot ("releases\$Tag-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $directory -Force | Out-Null
$savedJava = $env:JAVA_HOME
$savedPath = $env:PATH
try {
    $env:JAVA_HOME = $jdk.FullName
    $env:PATH = (Join-Path $jdk.FullName 'bin') + ';' + $env:PATH
    $releaseJson = & gh release view $Tag --repo $repo --json isDraft,assets
    if ($LASTEXITCODE -ne 0) { throw 'Could not read GitHub release' }
    $release = ($releaseJson -join "`n") | ConvertFrom-Json
    if (-not $release.isDraft) { throw 'Only draft Android releases may be signed by this command' }
    if (@($release.assets | Where-Object { $_.name -eq 'wynd-android.apk' }).Count -gt 0) { throw 'Signed APK already exists; refusing to replace it' }
    & gh release download $Tag --repo $repo --dir $directory --pattern wynd-android-unsigned.apk --pattern wynd-android-unsigned.apk.sha256
    if ($LASTEXITCODE -ne 0) { throw 'Release download failed' }
    $unsigned = Join-Path $directory 'wynd-android-unsigned.apk'
    $wanted = (Get-Content -LiteralPath ($unsigned + '.sha256') -Raw).Trim()
    if ($wanted -notmatch '^[a-fA-F0-9]{64}$' -or (Get-FileHash -LiteralPath $unsigned -Algorithm SHA256).Hash -ne $wanted) { throw 'Unsigned APK checksum mismatch' }
    $attestation = & gh attestation verify $unsigned --repo $repo --source-ref "refs/tags/$Tag" --signer-workflow "$repo/.github/workflows/android.yaml" --format json
    if ($LASTEXITCODE -ne 0) { throw 'APK build provenance verification failed' }
    ($attestation -join "`n") | Set-Content -LiteralPath (Join-Path $directory 'attestation.json') -Encoding utf8
    $metadata = & (Join-Path $tools 'aapt.exe') dump badging $unsigned
    if ($LASTEXITCODE -ne 0) { throw 'Could not read APK metadata' }
    $badging = $metadata -join "`n"
    if ($badging -notmatch "(?m)^package: name='mixeme\.wynd' " -or $badging -notmatch ("versionName='" + [regex]::Escape($version) + "'") -or $badging -notmatch ("versionCode='" + $expectedCode + "'") -or $badging -match '(?m)^application-debuggable') { throw 'APK package/version/debug flags do not match the release' }
    & (Join-Path $tools 'zipalign.exe') -c 4 $unsigned
    if ($LASTEXITCODE -ne 0) { throw 'Unsigned APK is not aligned' }
    $signed = Join-Path $directory 'wynd-android.apk'
    Write-Host 'Введите пароль Android keystore локально в apksigner; пароль не передаётся скрипту.'
    & (Join-Path $tools 'apksigner.bat') sign --ks $KeyStore --ks-key-alias $KeyAlias --out $signed $unsigned
    if ($LASTEXITCODE -ne 0) { throw 'APK signing failed' }
    $certificates = & (Join-Path $tools 'apksigner.bat') verify --verbose --print-certs $signed
    if ($LASTEXITCODE -ne 0) { throw 'Signed APK verification failed' }
    if (($certificates -join "`n") -match 'CN=Android Debug') { throw 'Debug signing key is not permitted for public releases' }
    $certificates | Write-Output
    $hash = (Get-FileHash -LiteralPath $signed -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath ($signed + '.sha256') -Value $hash -Encoding ascii
    Write-Host "Signed APK: $signed"
    Write-Host "SHA-256: $hash"
    $rawNotes = & gh api "repos/$repo/contents/android/whatsnew.md?ref=$Tag" --header 'Accept: application/vnd.github.raw+json'
    if ($LASTEXITCODE -ne 0) { throw 'Could not read tagged Android release notes' }
    $section = [regex]::Match(($rawNotes -join "`n"), ('(?ms)^## ' + [regex]::Escape($version) + '\s*\r?\n(.*?)(?=^## |\z)'))
    if (-not $section.Success -or -not $section.Groups[1].Value.Trim()) { throw 'Release notes for this version are missing' }
    $notesFile = Join-Path $directory 'release-notes.md'
    Set-Content -LiteralPath $notesFile -Value $section.Groups[1].Value.Trim() -Encoding utf8
    $readiness = & gh api "repos/$repo/contents/android/version.properties?ref=$Tag" --header 'Accept: application/vnd.github.raw+json'
    if ($LASTEXITCODE -ne 0) { throw 'Could not read tagged release readiness' }
    if ($Publish -and (($readiness -join "`n") -notmatch '(?m)^releaseReady=true\s*$')) { throw 'First-run/bootstrap is not release-ready; publication is blocked' }
    if ($Upload -or $Publish) {
        & gh release upload $Tag --repo $repo $signed ($signed + '.sha256')
        if ($LASTEXITCODE -ne 0) { throw 'Signed APK upload failed' }
    }
    if ($Publish) {
        & gh release edit $Tag --repo $repo --draft=false --title "Wynd для Android $version" --notes-file $notesFile
        if ($LASTEXITCODE -ne 0) { throw 'Release publication failed' }
    }
    if (-not $Upload -and -not $Publish) { Write-Host 'Файлы подготовлены локально. Выпуск на GitHub не изменён.' }
} finally {
    $env:JAVA_HOME = $savedJava
    $env:PATH = $savedPath
}
