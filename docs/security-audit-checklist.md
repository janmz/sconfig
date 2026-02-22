# Security audit checklist

When performing a security audit on this project, the following steps are
**required** (not optional).

## Composer (PHP)

If the project contains any `composer.json`:

1. In **each** directory that has a `composer.json` (e.g. project root and
   `php/`):
   - Run `composer install` (or ensure dependencies are installed).
   - Run `composer audit`.
2. Include the output and any findings in the security report (e.g. under
   “Dependencies” or “PHP”).
3. Do not skip this step or only “consider” it; it is mandatory when
   Composer is used.

## Other steps

- Go: run `govulncheck ./...` and include results.
- Follow any additional steps defined in the audit process or in
  SECURITY.md.
