# Local CI checks — keep in sync with .github/workflows/ci.yml
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found: $Name"
    }
}

Write-Host "==> go vet"
Require-Command go
go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> govulncheck"
if (-not (Get-Command govulncheck -ErrorAction SilentlyContinue)) {
    go install golang.org/x/vuln/cmd/govulncheck@latest
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
govulncheck ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> go test"
go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if (Test-Path "composer.json") {
    Write-Host "==> composer audit (root)"
    Require-Command composer
    composer install --no-interaction --prefer-dist --no-progress
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    composer audit
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if (Test-Path "php/composer.json") {
    Write-Host "==> composer audit (php/)"
    Require-Command php
    Require-Command composer
    Push-Location php
    try {
        composer install --no-interaction --prefer-dist --no-progress
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        composer audit
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }

    Write-Host "==> phpunit (php/)"
    Push-Location php
    try {
        & .\vendor\bin\phpunit
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

Write-Host "All local CI checks passed."
