$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    go run ./cmd/fetch-assets
} finally {
    Pop-Location
}
