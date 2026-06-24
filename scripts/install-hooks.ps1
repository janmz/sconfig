$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

git config core.hooksPath .githooks

Write-Host "Installed git hooks (core.hooksPath=.githooks)."
Write-Host "Pre-commit runs: scripts/ci-local.sh (via Git Bash)."
Write-Host "Run manually: .\scripts\ci-local.ps1"
