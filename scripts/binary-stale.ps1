param(
    [Parameter(Mandatory = $true)]
    [string]$Binary,
    [Parameter(Mandatory = $true)]
    [string]$Root
)

$bin = Get-Item -LiteralPath $Binary
$roots = @(
    (Join-Path $Root 'go.mod'),
    (Join-Path $Root 'go.sum'),
    (Join-Path $Root 'VERSION'),
    (Join-Path $Root 'cmd'),
    (Join-Path $Root 'internal'),
    (Join-Path $Root 'web\embed.go'),
    (Join-Path $Root 'web\dist'),
    (Join-Path $Root 'web\src'),
    (Join-Path $Root 'web\package.json'),
    (Join-Path $Root 'web\package-lock.json'),
    (Join-Path $Root 'web\svelte.config.js'),
    (Join-Path $Root 'web\vite.config.ts')
)

$newest = Get-ChildItem -LiteralPath $roots -Recurse -File -ErrorAction SilentlyContinue |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1

if ($newest -and $newest.LastWriteTime -gt $bin.LastWriteTime) {
    exit 1
}
exit 0
