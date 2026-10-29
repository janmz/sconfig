# Changelog

All notable changes to this project are documented in this file.

## [3.0.0.65] - 2026-10-29

Release der Änderungen aus 2.1.0.63 und 2.1.0.64 als neue Hauptversion
(2.1.0 wurde nie veröffentlicht).

### Changed

- **Go:** Modulpfad `github.com/janmz/sconfig/v3`. Programme müssen den
  Importpfad ausdrücklich auf `/v3` umstellen; `/v2` bleibt bei Version 2.
- **PHP:** Composer-Version 3.0.0 (Tag `v3.0.0`), Einbindung mit
  `"janmz/sconfig": "^3.0"`.
- **Doku:** Abschnitt „Upgrading from v2 to v3“ / „Upgrade von v2 auf v3“ in
  README.md, README.de.md und php/README.md; `go get`, Import und
  `-ldflags`-Beispiel auf `/v3` (das nicht existierende `GitCommit` entfernt).
  Versionsangaben „2.1.0“ in Doku, Code-Kommentaren und Meldungen auf „3.0.0“.

---

## [2.1.0.64] - 2026-10-27

Umsetzung des Sicherheitsaudits vom 2026-10-27 (`securityreport.md`, 15 von
16 Funden; SA-7 abgelehnt wie CR-20).

### Security

- **PHP:** `set()` lehnt ungültige Variablennamen und Werte mit
  Steuerzeichen ab; `EnvFile::render()` verweigert Zeilenumbrüche – keine
  Zeileninjektion in `.env` mehr (SA-1).
- **Go & PHP:** v2-Werte sind per AAD an den Feldnamen gebunden
  (`sconfig:<feld>`, Go/PHP-kompatibel); Quellenliste muss kanonisch sein
  (sortiert, eindeutig, max. 8 Namen/256 Zeichen) (SA-8, SA-10).
- **Go & PHP:** optionale geheime Schlüsseldatei als zusätzliche Quelle
  `keyfile` (`SetKeyFile` / `EnvLoader::setKeyFile`); vorhandene Werte werden
  beim Laden auf alle Quellen hochgestuft (SA-4).
- **Go & PHP:** strikter Modus gegen Alt-Werte (`SetRejectLegacy` /
  `setRejectLegacy`); Warnungen nach Migration (Passwörter wechseln), bei
  fehlgeschlagener Migration, bei für andere lesbaren Dateien und (PHP) bei
  `.env` im Webroot; Warnungskanal `SetWarningHandler` /
  `setWarningHandler` (SA-2, SA-6, SA-9, SA-16).
- **Go & PHP:** externe Programme mit absoluten Pfaden, ohne `cmd.exe`,
  fester Suchpfad bzw. minimale Umgebung, `NoDefaultCurrentDirectoryInExePath`;
  System32 per `GetSystemDirectoryW` bzw. validiertem `SystemRoot`; Go
  `WaitDelay`, Ausgabelimit 1 MiB; PHP räumt Temp-Dateien nach Timeout auf
  (SA-3, SA-11, SA-12). Die Alt-IDs bleiben identisch (geprüft).
- **Go:** atomares Schreiben behält Symlinks, Unix-Owner und unter Windows
  ACL/Attribute (`ReplaceFileW`); neue Dateien unter Windows mit DACL nur für
  Eigentümer, SYSTEM, Administratoren. **PHP:** Owner/Gruppe werden (falls
  erlaubt) übernommen (SA-6).
- **PHP:** `EnvLoader::setExportPasswords(false)` verhindert den Export
  entschlüsselter Passwörter nach `$_ENV`/`putenv()`; Befehle laufen mit
  minimaler Umgebung (SA-5). `updateEnv()` schreibt nur Schlüssel der
  Zieldatei bzw. der zuletzt geladenen Datei und aus `set()` (SA-14).
- **Go & PHP:** Größenlimits beim Lesen (Config 16 MiB, `.env` und Locales
  1 MiB) (SA-13).
- **Doku:** Optionen, Bedrohungsmodell, Container/LFI, Umgang mit
  Fehlermeldungen in Webanwendungen (SA-15).

---

## [2.1.0.63] - 2026-10-27

Umsetzung des Code-Reviews vom 2026-10-26 (`codereview.md`, 32 von 33
Funden; CR-20 abgelehnt).

### Security

- **Go & PHP: neues Schlüsselformat v2** (CR-1/2/3/14/15/16). Neue Werte:
  `v2:<Quellen>:<base64(nonce|ct|tag)>`, Schlüssel per HKDF-SHA256 aus
  stabilen Rechnerquellen (Windows: `MachineGuid` + SMBIOS-UUID per CIM,
  Linux: `/etc/machine-id`, macOS: `IOPlatformUUID`, FreeBSD:
  `kern.hostuuid`). Kein `wmic`, keine MAC-Adresse, kein 31/32-Bit-Seed
  mehr. Go und PHP erzeugen identische Schlüssel (gemeinsame Testvektoren).
- **Migration:** Alt-Werte (v1) werden weiter gelesen (Ableitung eingefroren,
  Known-Answer-Tests) und beim Laden automatisch als v2 neu geschrieben.
- **Fehlende Quellen** werden mit Namen gemeldet (`config.v2_source_missing`,
  CR-4); externe Befehle laufen mit Timeout, ein Timeout ist ein Fehler
  (CR-21).
- **Atomares Schreiben** über Temp-Datei + Rename, neue Dateien mit 0600
  (Go & PHP, CR-7/22/23).
- **Pfadprüfung** löst Symlinks und Windows-Junctions auf (CR-25).
- **i18n:** keine automatische Übernahme eines fremden `locales/`-Ordners
  mehr (Go: `SetLocalesDir()`, PHP: `I18n::initialize($pfad)`), kein Panic,
  Passwort-Marker immer aus den eingebauten Texten (CR-12).
- **PHP:** `updateEnv()` schreibt keine Werte der Prozess-Umgebung mehr
  (CR-6); Schreibfehler werfen eine Exception (CR-7).
- `securityreport.md` und `codereview.md` in `.gitignore`;
  `securityreport.md` aus dem Git-Index entfernt (war eingecheckt).

### Fixed

- **Go:** Passwörter in Pointer-Substructs, `[]*T`, Arrays und Maps werden
  verschlüsselt (CR-5); nur das oberste `Version`-Feld wird synchronisiert
  (CR-10); `config_init` beendet den Prozess nicht mehr per `log.Fatalf`,
  Fehler werden zurückgegeben (CR-11); `default`-Tags gelten auch für aus der
  Datei geladene Slice-/Array-/Map-Elemente und Pointer-Ziele, neue Typen
  `uint*`, `int8–32`, `float*`, `time.Duration` (CR-18); Indexgrenze im
  `ipconfig`-Parser (CR-19); Fehler von `rand.Read` geprüft (CR-33);
  `UpdateConfig` setzt ein `uint`-`Version` nicht mehr auf 0.
- **Go:** `LoadConfig`/`UpdateConfig` sind threadsicher (Mutex, CR-24);
  Sprachwahl LC_ALL > LC_MESSAGES > LANG (Go & PHP).
- **PHP:** `.env` behält beim Zurückschreiben Quotes, Leerzeilen,
  `export`-Präfixe und Zeilen ohne `=`; Werte mit Leerzeichen (Marker)
  werden gequotet (CR-8); falscher Schlüssel führt zu `RuntimeException`
  statt `ArgumentCountError` (CR-9).
- **PHP:** eingebaute Übersetzungen werden in `php/locales/` mitgeliefert
  (bisher fehlten sie im Composer-Paket).

### Changed

- **Go:** Code aufgeteilt in `hwid_v1.go` (eingefroren), `hwid_v2.go`,
  `keys.go`, `walk.go`, `fileio.go`, `command.go`; toter Code entfernt,
  Bubble-Sorts durch `slices` ersetzt, ein gemeinsamer Struct-Walker,
  Base64 wird nur einmal geprüft (CR-13/26/28/29/30/32); Kommentare auf
  Englisch (CR-31).
- **PHP:** `EnvLoader` aufgeteilt in `Crypto`, `MachineIdentity`,
  `LegacyHardwareId` (eingefroren), `EnvFile`, `Command`; Testhook
  `EnvLoader::setHardwareIdForTest()`.
- **Doku:** README.md/README.de.md/php/README.md beschreiben v2 und v1
  vollständig; CLAUDE.md-Übersicht aktualisiert.

---

## [2.0.8.62] - 2026-10-26

### Documentation

- **README.md, README.de.md, php/README.md:** Neuer Abschnitt „How the
  hardware ID is created“ / „So entsteht die Hardware-ID“. Beschreibt für Go
  und PHP Schritt für Schritt VM-Erkennung, MAC-Auswahl, alle
  Identifikatoren je Betriebssystem, Fallbacks, Sortierung, Hash,
  Byte-Reihenfolge und Schlüsselerzeugung.
- Klargestellt: Die „Hardware-ID“ enthält auch Nicht-Hardware-Daten
  (`MachineGuid`, `/etc/machine-id`, Netzwerkzustand, PHP-Fallback
  Hostname + PHP-Version); ohne `wmic` (aktuelles Windows 11) bzw. unter
  macOS (Go) besteht sie nur aus der MAC-Adresse; Go und PHP erzeugen
  unterschiedliche IDs/Schlüssel und sind nicht kompatibel; der effektive
  Schlüsselraum beträgt nur ca. 2^31 (Go) bzw. 2^32 (PHP).
- **CLAUDE.md:** falsche Aussage „Go und PHP liefern dieselbe ID“ korrigiert.
- Code-Review durchgeführt (`codereview.md`, 33 Funde).

---

## [2.0.7.60] - 2026-10-10

### Fixed

- **CI:** `govulncheck` auf v1.7.0 gepinnt (Workflow und
  `scripts/ci-local.*`). `@latest` (v1.8.0) setzt Go 1.26 voraus und brach den
  Workflow mit Go 1.25 ab.
- **PHP:** Alle `phpcs`-Funde (PSR-12) in `src/`, `tests/` und `example.php`
  behoben.
- **Doku:** Alle `markdownlint`-Funde in den Markdown-Dateien behoben
  (Zeilenlänge, Leerzeilen um Listen/Codeblöcke/Überschriften, nackte URL).

---

## [2.0.7.59] - 2026-10-10

### Changed

- **Go & PHP:** Config-/`.env`-Pfade dürfen jetzt zusätzlich unterhalb des
  Benutzerverzeichnisses liegen (`%USERPROFILE%` unter Windows, `~` unter
  Unix), z. B. `%USERPROFILE%\AppData\Roaming\<app>` oder `~/.config/<app>`.
  Bisher wurden solche Pfade mit „path outside executable“ abgelehnt.
  Neue Testhooks `SetHomeDirForTest()` (Go) und
  `EnvLoader::setHomeDirForTest()` (PHP); Fehlermeldung
  `config.path_outside_executable` in en/de ergänzt.

---

## [2.0.6.58] - 2026-10-10

### Security

- **Go:** `golang.org/x/text` von v0.28.0 auf v0.41.0 angehoben
  (GO-2026-5970, GO-2026-6629). v0.41.0 ist die kleinste Version mit Fix, die
  noch Go 1.25 unterstützt.
- **PHP (dev):** `squizlabs/php_codesniffer` von 3.13.5 auf 3.13.6 angehoben
  (CVE-2026-67434, OS Command Injection); `composer audit` im Pre-Commit-Hook
  und in der CI ist wieder grün.

---

## [2.0.6.57] - 2026-10-10

### Fixed

- **Windows:** Die Befehle zur Ermittlung der Hardware-ID (`wmic`, `reg`,
  `ipconfig`, `route`, `netsh`) laufen jetzt ohne Konsolenfenster
  (`CREATE_NO_WINDOW`). In GUI-Programmen (`-H windowsgui`) blitzten beim
  Laden/Speichern der Konfiguration sonst mehrere Fenster auf. Die ermittelte
  Hardware-ID bleibt unverändert, verschlüsselte Werte bleiben lesbar.

---

## [2.0.4.55] - 2026-06-24

### Fixed

- **CI:** Root-`composer audit` wird übersprungen, wenn `composer.lock` keine
  Pakete enthält (ältere Composer-Versionen in Actions beenden sonst mit Exit
  1 trotz `--locked`). Composer in Actions auf `composer:v2` gepinnt.

---

## [2.0.3.53] - 2026-06-24

### Fixed

- **CI:** Go-Version im Workflow auf `1.25.x` angehoben (entspricht `go.mod`).
- **CI:** `composer audit --locked` für Root- und PHP-Lockfile; behebt Fehler
  „No installed packages found“ beim leeren Root-`composer.lock`.

---

## [2.0.2.51] - 2026-06-24

### Added

- **Pre-commit:** Git-Hook unter `.githooks/pre-commit` und Skripte
  `scripts/ci-local.sh` / `scripts/ci-local.ps1` führen vor jedem Commit
  dieselben Prüfungen wie der CI-Workflow aus (`go vet`, `govulncheck`,
  `go test`, `composer audit`, PHPUnit). Installation über
  `scripts/install-hooks.sh` bzw. `scripts/install-hooks.ps1`.

---

## [2.0.2.50] - 2026-06-24

### Fixed

- **PHP/CI:** PHPUnit-Test `testMarkerWithoutSecurePasswordThrowsClearError`
  importiert `Sconfig\I18n` korrekt (behebt CI-Job `composer-audit`).
- **CI:** `php/composer.lock` wird versioniert; reproduzierbare
  `composer install`-Läufe in GitHub Actions.

### Changed

- **CI:** GitHub Actions auf `checkout@v5`, `setup-go@v6`, `cache@v5`
  aktualisiert; Node.js-24-Opt-in für Action-Laufzeit.

---

## [2.0.2.49] - 2026-06-12

### Fixed

- **Go:** Passwort-Paare werden vor der Entschlüsselung geprüft: Markierungstext
  im `Password`-Feld ohne Wert im `SecurePassword`-Feld liefert eine klare
  Fehlermeldung statt eines technischen Decrypt-Fehlers; ungültiges Base64,
  zu kurze oder zu lange Ciphertext-Werte werden abgefangen.
- **Go:** Beim Einlesen der Config keine Panics mehr bei nicht-setzbaren Feldern,
  nicht-numerischen `Version`-Typen oder fehlerhaften verschlüsselten Werten.

---

## [2.0.1.46] - 2026-04-03

### Changed

- **Config- und .env-Pfade (Go/PHP):** Zusätzlich zum Verzeichnis der ausführbaren
  Datei ist das **aktuelle Arbeitsverzeichnis** als gültige Pfadwurzel erlaubt;
  relative Pfade werden gegen das CWD aufgelöst (`filepath.Abs` / PHP: Auflösung
  wie bisher relativ zu `getcwd()`). Damit lassen sich Konfigurationsdateien
  außerhalb des Binary-Verzeichnisses nutzen, wenn der Aufrufer das CWD setzt.

---

## [2.0.1.45] - 2026-04-02

### Fixed

- **Go:** Kommentare zu `writeDebugLog` und Debug-Tracking: Dateiname überall
  `sconfig.debug.txt` (entspricht `debugLogFilename`); Zeilenformat und Hinweis
  auf nicht existentes `debugLogMu` bereinigt.

---

## [2.0.1.44] - 2026-04-02

### Added

- **Dokumentation (Go/PHP):** Klarstellung im Code, dass die Kernfunktion von sconfig
  ein maschinengebundener, deterministischer Schlüssel ist (kein
  Zufallsschlüssel pro Start), und dass der Debugmodus nur bei Fehleranalyse
  mit sensiblen Daten eingesetzt werden soll.
- **CI:** `govulncheck ./...` im Go-Build-Job.

### Changed

- **Config- und .env-Pfade (Go `LoadConfig`/`UpdateConfig`, PHP `EnvLoader::load`/
  `updateEnv`):** Pfade werden bereinigt; Dateien müssen unterhalb des
  Verzeichnisses der ausführbaren Datei liegen (relative Pfade beziehen sich darauf).
  PHP: optional `EnvLoader::setExecutableRoot()` für die Anwendungsbasis; in PHPUnit
  `setExecutableRootForTest()` wie bisher.

---

## [1.2.16.41] - 2026-02-27

### Added

- **UpdateConfig (Go):** Exportierte Funktion
  `UpdateConfig(config, path, cleanConfig...)` zum Zurückschreiben der
  Config-Datei nach Änderungen (z. B. Theme in der UI). Secure-Felder werden
  standardmäßig verschlüsselt; optional `cleanConfig=true` für Klartext in der Datei.
  Voraussetzung: Mindestens einmal `LoadConfig` muss vorher aufgerufen worden
  sein. Die   Config kann unter einem anderen Pfad als dem Lade-Pfad geschrieben
  werden (z. B. Backup).
- **updateEnv und set (PHP):** Öffentliche Methoden
  `EnvLoader::set($key, $value)` und
  `EnvLoader::updateEnv($filePath, $cleanConfig = false)` zum Setzen von Werten
  und   Zurückschreiben der .env-Datei (z. B. nach Theme-Änderung).
  Voraussetzung: Zuerst `load()` aufrufen; Schreiben unter anderem Pfad ist erlaubt.

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
