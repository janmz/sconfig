# Changelog

All notable changes to this project are documented in this file.

## [1.2.16.41] - 2026-02-27

### Added

- **UpdateConfig (Go):** Exportierte Funktion `UpdateConfig(config, path, cleanConfig...)` zum
  Zurückschreiben der Config-Datei nach Änderungen (z. B. Theme in der UI). Secure-Felder
  werden standardmäßig verschlüsselt; optional `cleanConfig=true` für Klartext in der Datei.
  Voraussetzung: Mindestens einmal `LoadConfig` muss vorher aufgerufen worden sein. Die
  Config kann unter einem anderen Pfad als dem Lade-Pfad geschrieben werden (z. B. Backup).
- **updateEnv und set (PHP):** Öffentliche Methoden `EnvLoader::set($key, $value)` und
  `EnvLoader::updateEnv($filePath, $cleanConfig = false)` zum Setzen von Werten und
  Zurückschreiben der .env-Datei (z. B. nach Theme-Änderung). Voraussetzung: Zuerst `load()`
  aufrufen; Schreiben unter anderem Pfad ist erlaubt.

---

## [1.2.16.40] - 2026-02-27

### Added

- **Tests für die PHP-Variante:** PHPUnit-Testsuite für `php/` ergänzt:
  - `tests/bootstrap.php` setzt Autoload und Locales-Pfad
  - `tests/EnvLoaderTest.php`: Tests für `EnvLoader::load()` (Datei anlegen, parsen,
    Anführungszeichen, Kommentare, override, cleanConfig), `get()`/`has()`/`clear()`/
    `isLoaded()`, Passwort-Verschlüsselung (Marker auf Disk, Klartext in Memory),
    `env()`-Helper; Test für nicht lesbare Datei (unter Linux, unter Windows
    übersprungen)
  - `phpunit.xml.dist` und PHPUnit 9.5 als Dev-Abhängigkeit in `composer.json`
  - CI-Job erweitert: PHPUnit läuft nach Composer install für `php/`

### Fixed

- **PHP EnvLoader:** Schreiben der .env-Datei erfolgt vor dem Entschlüsseln der
  Passwörter im Speicher, damit auf Disk nur Marker und verschlüsselte Werte
  stehen (kein Klartext).
- **PHP EnvLoader:** Beim erneuten Laden mit `override=false` bleibt der Cache
  für bereits gesetzte Keys erhalten; nur entschlüsselte Passwort-Keys ersetzen
  den Marker im Cache.

---

## [1.2.14.37] - 2026-02-12

### Added

- **Diagnose: Mit welchem Key wurde verschlüsselt?** Neue Datei `diagnose_decrypt.go`
  mit exportierten Hilfsfunktionen, um bei bekanntem Ciphertext und bekanntem
  Klartext den verwendeten Schlüssel zu identifizieren: `KeyFromHardwareID(hwID)`
  leitet den 32-Byte-Key ab (go123-RNG), `DecryptWithKey(ciphertextBase64, key)`
  entschlüsselt mit einem beliebigen Key, `TryDecryptWithCandidates(ciphertext,
  expectedPlaintext, candidateHardwareIDs)` probiert Kandidaten durch und
  liefert die passende Hardware-ID bzw. false. So kann man z. B. prüfen, ob
  mit anderer Identifier-Reihenfolge oder anderer Maschine verschlüsselt wurde.

---

## [1.2.14.36] - 2026-02-12

### Added

- **Tests für Schlüssel-/Entschlüsselungs-Varianten:** In `sconfig_test.go` wurden
  Tests ergänzt, die mit `go test -v ./...` auf dem Production-Server klare
  Ausgaben erzeugen: (1) Go-1.23-RNG-Determinismus (gleicher Seed → gleicher
  Key, inkl. Hex-Ausgabe), (2) Identifier-Reihenfolge (A|B vs. B|A liefern
  unterschiedliche Hardware-IDs und Schlüssel; Entschlüsselung mit falscher
  Reihenfolge schlägt erwartungsgemäß fehl), (3) Roundtrip Verschlüsseln/
  Entschlüsseln mit fester Hardware-ID, (4) Ciphertext-Integrität (Base64-Länge,
  decodierte Länge). Zusätzlich: `ResetForTest()` zum Zurücksetzen des
  Package-Zustands, damit Tests mit wechselnder Hardware-ID möglich sind.

---

## [1.2.12.33] - 2026-02-12

### Fixed

- **Schlüsselableitung unabhängig von Go-Version:** Nach dem Wechsel auf neuere
  Go-Toolchains (z. B. 1.25) lieferte die bisherige Nutzung von
  `math/rand.NewSource(hardwareID)` andere Bytes und bestehende Config-Dateien
  konnten nicht mehr entschlüsselt werden. Die Schlüsselableitung verwendet jetzt
  einen eingebetteten, mit Go 1.23 identischen RNG (Mitchell/Reeds) in
  `key_rand_go123.go`. Derselbe Seed (Hardware-ID) erzeugt damit wieder die
  gleichen 32 Key-Bytes unabhängig von der Go-Version; bestehende Configs bleiben
  lesbar.

---

## [1.2.12.32] - 2026-02-12

### Fixed

- **Decrypt-Fehlermeldung:** Einheitliche Sprache pro Locale (DE/EN); technische
  Go-Meldung wird als „Technische Meldung: %v“ getrennt ausgewiesen, um
  Sprachmix zu vermeiden. Passwortfeldname wird immer angezeigt; bei leerem
  Präfix (z. B. Feld „SecurePassword“) erscheint „(Feldname unbekannt)“ bzw.
  „(unknown field)“.

---

## [1.2.11.30] - 2026-02-12

### Changed

- **Security audit:** Composer-audit requirement moved from project rule
  `.cursor/rules/security-audit.mdc` into the security-auditor subagent
  definition (`~/.cursor/agents/security-auditor.md`). The .mdc file was
  removed; SECURITY.md and securityreport.md references updated.

---

## [1.2.11.29] - 2026-02-12

### Changed

- **Encryption key derivation (by design):** Documented in code (Go `config_init`,
  PHP `EnvLoader::initializeEncryption`), SECURITY.md, and securityreport.md
  that the use of `math/rand` and `mt_rand` is intentional: the same
  hardware ID must yield the same encryption key; security is provided by the
  hardware-derived input being unknowable without full machine access. Security
  audit finding set to acknowledged (by design).

---

## [1.2.11.28] - 2026-02-12

### Added

- **Security audit – Composer audit required:** SECURITY.md,
  `docs/security-audit-checklist.md`, and a Cursor rule
  and the security-auditor subagent definition now require that security
  audits include `composer audit` whenever the project has Composer
  (`composer.json`). CI workflow runs `composer audit` for project root and
  `php/` when applicable.

---

## [1.2.11.27] - 2026-02-12

### Changed

- **Config file permissions:** When writing the config file, the existing file’s
  permission bits are preserved instead of enforcing 0644. New files (path did
  not exist) are still created with 0644.

---

## [1.2.11.26] - 2026-02-12

### Fixed

- **Encrypt/decrypt error handling (security):** `encrypt()` now checks and
  returns errors from `aes.NewCipher` and `cipher.NewGCM`; signature changed
  to `(string, error)`. Call site in `updateVersionAndPasswords` propagates
  errors.

---

## [1.2.11.25] - 2026-02-12

### Fixed

- **Decrypt panic on invalid/short input (security):** `decrypt()` now checks
  base64 decode errors and ensures `len(data) >= gcm.NonceSize()` before
  slicing; returns clear errors instead of panicking. Errors from
  `aes.NewCipher` and `cipher.NewGCM` are also handled.

---

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
