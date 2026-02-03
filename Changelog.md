# Changelog

All notable changes to this project are documented in this file.

## [1.2.8.18] - 2025-02-03

### Added

- **Debugging hardware ID / key changes**: Exported `DebugHardwareID()` to print all hardware identifiers and the final ID to stderr without loading a config file. Documentation in README.md and README.de.md: causes for changing hardware keys (MAC/network, Windows/Linux identifiers) and step-by-step debug workflow using `DebugHardwareID()` or `LoadConfig(..., true, ...)`.

### Changed

- None.

### Fixed

- None.

---

Earlier changes are documented in the file header of `sconfig.go` and in the git history.
