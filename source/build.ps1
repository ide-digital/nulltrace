$ErrorActionPreference = "Stop"

$Source = $PSScriptRoot
$Root = Split-Path $Source -Parent
$App = Join-Path $Root "app"
New-Item -ItemType Directory -Force -Path $App | Out-Null

$env:CGO_ENABLED = "0"
if (Test-Path "C:\Program Files\Go\bin") {
    $env:PATH = "C:\Program Files\Go\bin;" + $env:PATH
}

function Build($ldflags, $out, $pkg) {
    go build -trimpath -ldflags $ldflags -o $out $pkg
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Set-Location $Source
Build "-H windowsgui -s -w" (Join-Path $Root "NullTrace.exe") ./cmd/launch
Build "-s -w" (Join-Path $App "nulltrace.exe") ./cmd/nulltrace
Build "-s -w" (Join-Path $App "nulltraced.exe") ./cmd/nulltraced

Write-Host "Built:"
Write-Host "  $(Join-Path $Root 'NullTrace.exe')"
Write-Host "  $(Join-Path $App 'nulltrace.exe')"
Write-Host "  $(Join-Path $App 'nulltraced.exe')"
