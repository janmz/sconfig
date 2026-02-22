# Security Policy

## Supported Versions

The latest release and the `main` branch receive security updates.

## Design: hardware-bound encryption key

The encryption key is derived **deterministically** from a hardware-derived ID
(Go: `math/rand.NewSource(hardwareID)`, PHP: `mt_srand($hardwareId)` +
`mt_rand()`). This is **intentional**: the same machine must always produce the
same key so that config files remain decryptable. Security is provided by the
fact that the input (hardware identifiers) cannot be guessed by anyone without
full access to the machine the code runs on; the PRNG is used only to expand
that secret seed into 32 key bytes, not for cryptographic randomness. Audits
should treat this as an accepted design choice (see securityreport.md).

## Security audits

When a security audit is performed (e.g. by the security-auditor subagent or
manually), follow the **required** steps in
[docs/security-audit-checklist.md](docs/security-audit-checklist.md). In
particular:

- **Composer (PHP):** If the project contains any `composer.json`, run
  `composer install` and then `composer audit` in each such directory and
  include the results in the audit report. Do not omit this step when
  Composer is used.

Other audit steps (e.g. Go `govulncheck`, dependency review) apply as defined
in the checklist and audit process.

## Reporting a Vulnerability

Please report security issues privately to `jan@vaya-consulting.de`.
We will acknowledge receipt within 72 hours and provide a timeline for a fix.
