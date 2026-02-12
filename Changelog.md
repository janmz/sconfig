# Changelog

All notable changes to this project are documented in this file.

## [1.2.11.24] - 2026-02-12

### Changed

- **Repository-Bereinigung**: `.cursor/`, `*.syso` und `vaya.ico` werden nicht mehr
  versioniert. Einträge in `.gitignore` ergänzt. Bestehende getrackte Vorkommen
  mit `git rm -r --cached .cursor`, `git rm --cached vaya.ico` aus dem Index
  entfernen (einmalig, wenn kein anderes Git-Prozess läuft).

---

## [1.2.10.22] - 2026-02-03

### Added

- **Debug hardware-ID track**: In debug mode, a log of hardware IDs is written to
`sconfig.debug.json` in the executable’s directory. The file is created on first
use. Each line is: date (TAB) time (TAB) hardware ID (TAB) string of identifiers
that were hashed. When decryption fails, an extra entry is written for the current
(changed) ID so the timeline of IDs is visible.

---

## [1.2.10.21] - 2026-02-03

### Fixed

- **Decrypt error message formatting**: The message for decryption failure
  (`config.decrypt_failed`) used the i18n format string "failed to decrypt %s
  password: %v" but only passed the password field name to `t()`, not the
  error. That led to `%v(MISSING)` and malformed output like
  `&{%!!(string=cipher: message authentication failed)}v(MISSING)`. The code
  now passes both the field name and the error to `t()` and uses
  `fmt.Errorf("%s", t(...))` so the full message is formatted correctly
  (e.g. "failed to decrypt Root password: cipher: message authentication
  failed").

---

## [1.2.9.18] - 2025-02-03

### Added

- **Debugging hardware ID / key changes**: Exported `DebugHardwareID()` to print
  all hardware identifiers and the final ID to stderr without loading a config
  file. Documentation in README.md and README.de.md: causes for changing
  hardware keys (MAC/network, Windows/Linux identifiers) and step-by-step
  debug workflow using `DebugHardwareID()` or `LoadConfig(..., true, ...)`.

### Changed

- None.

### Fixed

- None.

---

Earlier changes are documented in the file header of `sconfig.go` and in the
git history.
