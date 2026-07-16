param(
    [ValidateSet('modelscope', 'huggingface')]
    [string]$ModelSource = 'modelscope'
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    go run ./cmd/fetch-assets -model-source $ModelSource
} finally {
    Pop-Location
}
