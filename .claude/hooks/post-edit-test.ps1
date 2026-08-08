#!/usr/bin/env pwsh
# PostToolUse hook (Edit): runs the test package covering the file just
# edited, scoped to that package only (not the whole service) so it stays
# fast enough to run after every Edit. See harness plan
# "Thay đổi 2 — Hook chạy test sau mỗi lần Edit (B1)".

$ErrorActionPreference = 'Stop'

$raw = [Console]::In.ReadToEnd()
if (-not $raw) { exit 0 }

try {
    $payload = $raw | ConvertFrom-Json
} catch {
    exit 0
}

$filePath = $payload.tool_input.file_path
if (-not $filePath) { exit 0 }

$filePath = $filePath -replace '\\', '/'

if ($filePath -notmatch '(^|/)services/([^/]+)/(.+)$') {
    exit 0
}

$serviceName = $Matches[2]
$relPath = $Matches[3]

$repoRoot = $env:CLAUDE_PROJECT_DIR
if (-not $repoRoot) {
    $repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
}
$serviceRoot = Join-Path $repoRoot "services/$serviceName"

if ($filePath -match '\.go$') {
    $pkgDir = Split-Path $relPath -Parent
    $pkgDir = if ($pkgDir) { ($pkgDir -replace '\\', '/') } else { '' }
    $pattern = if ($pkgDir) { "./$pkgDir/..." } else { './...' }

    Push-Location $serviceRoot
    try {
        Write-Host "post-edit-test: go test $pattern (service: $serviceName)"
        & go test $pattern 2>&1 | Write-Host
        $exitCode = $LASTEXITCODE
    } finally {
        Pop-Location
    }

    if ($exitCode -ne 0) {
        Write-Error "go test failed: $serviceName $pattern"
        exit 2
    }
    exit 0
}

if ($filePath -match '\.py$') {
    $testDir = Join-Path $serviceRoot (Split-Path $relPath -Parent)

    Write-Host "post-edit-test: pytest $testDir (service: $serviceName)"
    & pytest $testDir 2>&1 | Write-Host
    $exitCode = $LASTEXITCODE

    if ($exitCode -ne 0) {
        Write-Error "pytest failed: $testDir"
        exit 2
    }
    exit 0
}

exit 0
