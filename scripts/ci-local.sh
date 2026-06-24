#!/usr/bin/env bash
# Local CI checks — keep in sync with .github/workflows/ci.yml
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "error: required command not found: $1" >&2
		exit 1
	fi
}

echo "==> go vet"
require_command go
go vet ./...

echo "==> govulncheck"
if ! command -v govulncheck >/dev/null 2>&1; then
	go install golang.org/x/vuln/cmd/govulncheck@latest
fi
govulncheck ./...

echo "==> go test"
go test ./...

if [ -f composer.json ] && [ -f composer.lock ]; then
	echo "==> composer audit (root)"
	require_command composer
	package_count="$(php -r '$l=json_decode(file_get_contents("composer.lock"),true); echo count($l["packages"]??[])+count($l["packages-dev"]??[]);')"
	if [ "$package_count" -eq 0 ]; then
		echo "No Composer packages in root lock file; skipping audit."
	else
		composer audit --locked
	fi
fi

if [ -f php/composer.json ]; then
	echo "==> composer audit (php/)"
	require_command php
	require_command composer
	(
		cd php
		composer install --no-interaction --prefer-dist --no-progress
		composer audit --locked
	)

	echo "==> phpunit (php/)"
	(
		cd php
		./vendor/bin/phpunit
	)
fi

echo "All local CI checks passed."
