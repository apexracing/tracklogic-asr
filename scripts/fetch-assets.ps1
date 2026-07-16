param(
    [ValidateSet('modelscope', 'huggingface')]
    [string]$ModelSource = 'modelscope',
    [ValidateSet('asr', 'tts', 'all')]
    [string]$Kind = 'all'
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    go run ./cmd/fetch-assets -model-source $ModelSource -kind $Kind
} finally {
    Pop-Location
}
