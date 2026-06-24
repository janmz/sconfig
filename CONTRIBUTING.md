# Contributing to sconfig

Thanks for your interest in contributing!

## Development

- Go version: see `go.mod`.
- PHP 8.2+ and Composer for PHP checks (see `php/INSTALL.md`).
- Run tests locally:

```bash
go test ./... -v
```

### Local CI (same checks as GitHub Actions)

```bash
./scripts/ci-local.sh
```

On Windows PowerShell:

```powershell
.\scripts\ci-local.ps1
```

### Pre-commit hook

Install once after cloning:

```bash
./scripts/install-hooks.sh
```

On Windows PowerShell:

```powershell
.\scripts\install-hooks.ps1
```

This sets `core.hooksPath` to `.githooks` and runs `scripts/ci-local.sh`
before each commit (`go vet`, `govulncheck`, `go test`, `composer audit`,
PHPUnit).

## Pull Requests

1. Fork the repo and create a feature branch.
2. Add tests for any changes.
3. Ensure `go test ./...` passes.
4. Update documentation if needed.
5. Submit a PR describing your changes and motivation.

## Commit Messages

- Use clear, descriptive messages.
- Reference issues where relevant (e.g., `Fixes #123`).

## Code of Conduct

By participating, you agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md).
