#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

git config core.hooksPath .githooks

chmod +x .githooks/pre-commit scripts/ci-local.sh scripts/install-hooks.sh 2>/dev/null || true

echo "Installed git hooks (core.hooksPath=.githooks)."
echo "Pre-commit runs: scripts/ci-local.sh"
echo "Run manually: ./scripts/ci-local.sh"
