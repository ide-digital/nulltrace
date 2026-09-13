$ErrorActionPreference = "Stop"

$Source = $PSScriptRoot
$Root = Split-Path $Source -Parent
$App = Join-Path $Root "app"
New-Item -ItemType Directory -Force -Path $App | Out-Null

$env:CGO_ENABLED = "0"
if (Test-Path "C:\Program Files\Go\bin") {
    $env:PATH = "C:\Program Files\Go\bin;" + $env:PATH
}

Set-Location $Source
go mod tidy
go build -trimpath -ldflags "-H windowsgui -s -w" -o (Join-Path $Root "NullTrace.exe") ./cmd/launch
go build -trimpath -ldflags "-s -w" -o (Join-Path $App "nulltrace.exe") ./cmd/nulltrace
go build -trimpath -ldflags "-s -w" -o (Join-Path $App "nulltraced.exe") ./cmd/nulltraced

Write-Host "Built:"
Write-Host "  $(Join-Path $Root 'NullTrace.exe')"
Write-Host "  $(Join-Path $App 'nulltrace.exe')"
Write-Host "  $(Join-Path $App 'nulltraced.exe')"
