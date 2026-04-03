# Security Report – sconfig

This document is updated with each security check. New chapters are prepended
so the newest audit is at the top; existing content is never deleted or
overwritten.

---

## Security Check 2026-04-02 22:25:01

### Self-Improvement (geprüfte Quellen)

- Geprüft:
  [OWASP Secure by Design Framework](https://owasp.org/www-project-secure-by-design-framework/)
  (Draft 0.5, Design-Phase; Verweis ASVS für Implementierung),
  [CNIL GDPR developer's guide](https://www.cnil.fr/en/gdpr-developers-guide)
  (Sheets 0–16),
  [EDPB Startseite](https://www.edpb.europa.eu/edpb_en)
  (Aktuelles zu OSS-Fällen, Leitlinien; keine neue technische
  Prüfliste für diese Bibliothek),
  [gdpr-info.eu](https://gdpr-info.eu/) (Artikel-Struktur, Verknüpfung
  Rechte/Prinzipien),
  [Manifestly GDPR Software-Checkliste](https://www.manifest.ly/use-cases/software-development/gdpr-compliance-checklist)
  (Überblick Prinzipien, Verweis auf interaktive Checkliste).
- Ergebnis: Ich habe die genannten URLs/Seiten geprüft und keine neuen
  Informationen gefunden, die die bestehenden Prüfschritte dieses Audits
  wesentlich ändern würden. OWASP SbD betont weiterhin frühe
  Architekturentscheidungen; CNIL/EDPB betonen Datensparsamkeit,
  Abhängigkeitskontrolle und Tests.

### Umfang

- **Codebasis:** Go `sconfig.go`, `key_rand_go123.go`, `i18n.go`, Tests;
  PHP `php/src/EnvLoader.php`, `I18n.php`, Hilfsfunktionen.
- **Risiko-Mapping:** Pfade zu Config/.env (nun relativ zum
  ausführbaren/Script-Verzeichnis begrenzt), Hardware-ID-Ermittlung
  (WMI/shell_exec/Dateien), AES-256-GCM, Debug-Ausgaben und Debug-Logdatei,
  keine Netzwerk-Listener in der Bibliothek.
- **Tooling:** `go vet ./...`, `go test ./...`, `govulncheck ./...`;
  `composer audit` im Projektroot und in `php/`.

### Befunde (nach Schwere)

- **Critical – Deterministische hardwaregebundene Schlüsselableitung (Go &
  PHP):** Wie in [SECURITY.md](SECURITY.md) beschrieben: derselbe Rechner
  erzeugt denselben Schlüssel (stabile Entschlüsselung der Dateien). **Status:**
  anerkannt (by design). **Nächster Schritt:** Keiner; Integratoren in
  Doku/Threat Model einbeziehen.

- **High – Debug-Modus gibt den Verschlüsselungsschlüssel preis (Go):** Bei
  `debugOutput == true` schreibt `config_init` den 32-Byte-Schlüssel in
  Hex auf stderr (`sconfig.go`, ca. Zeilen 1145–1147). **Status:** offen.
  **Nächster Schritt:** Ausgabe entfernen oder strikt hinter ein separates
  Opt-in mit Warnung; in Produktion nie aktivieren.

- **High – Sensible Daten in Debug-Log (Go):** `writeDebugLog` hängt
  Hardware-ID und Identifier-Strings an `sconfig.debug.txt` neben der
  Binärdatei (Modus `0600`). **Status:** offen. **Nächster Schritt:**
  Produktionsbetrieb ohne Debug; optional Pfad/Feature einschränken.

- **Medium – Pfadvalidierung Config/.env (Go & PHP):** Frühere Audits:
  fehlende Einschränkung. **Aktuell:** `resolveConfigPath` in Go und
  `resolveEnvFilePath` in PHP erzwingen Pfade unter dem
  Anwendungs-/Executable-Verzeichnis (`filepath.Rel` bzw. Prefix-Prüfung).
  **Status:** behoben (Stand dieses Audits). **Nächster Schritt:** API-Doku
  beibehalten: nur relative Pfade unter der Basis bzw. absolute Pfade, die
  nach Auflösung innerhalb der Basis liegen.

- **Medium – Verweis in SECURITY.md:** `SECURITY.md` verweist auf
  `docs/security-audit-checklist.md`; die Datei ist im Repository nicht
  vorhanden (gelöscht/verschoben). **Status:** offen. **Nächster Schritt:**
  Datei wiederherstellen, Link anpassen oder Verweis entfernen.

- **Low – Kommentar vs. Dateiname Debug-Log:** Kommentar zu
  `writeDebugLog` nennte `sconfig.debug.json`, Konstante
  `debugLogFilename` ist `sconfig.debug.txt`. **Status:** behoben
  (Kommentare in `sconfig.go` auf `sconfig.debug.txt` und tatsächliches
  Zeilenformat angepasst, Stand 2026-04-02).

- **Low – Abhängigkeiten Go:** `govulncheck ./...`: keine
  Vulnerabilities gemeldet. Module: `go-i18n/v2`, `golang.org/x/text`
  (siehe `go.mod`).

### Abhängigkeiten (Risiko-Zusammenfassung)

- **Go:** `govulncheck ./...` – keine Einträge (Ausführung 2026-04-02).
- **Composer (Projektroot):** `composer audit` – „No packages – skipping
  audit“ (keine `require`-Pakete).
- **Composer (`php/`):** `composer audit` – keine Security-Advisories.

### DSGVO (Kurzbeurteilung)

- **Art. 5 Prinzipien — Not evident:** Bibliothek ohne eigene
  Verarbeitungszwecke; Verantwortung beim einbindenden Produkt.
- **Art. 25 Privacy by Design — Partial:** Verschlüsselung sensibler Felder;
  Hardware-IDs können personenbezogen sein; Minimierung liegt beim
  Anbieter.
- **Art. 32 Sicherheit — Partial:** AES-GCM, Pfadkontainment; Debug schwächt
  Vertraulichkeit wenn aktiv.
- **Betroffenenrechte — Not evident:** Keine Endnutzer-API in dieser
  Bibliothek.

### Empfohlene nächste Schritte

1. Debug-Schlüsselausgabe auf stderr entfernen oder hart absichern.
2. `SECURITY.md`-Link zur Audit-Checkliste reparieren.
3. Debug-Modus und `sconfig.debug.txt` nur in Entwicklungsumgebungen
  dokumentieren/erzwingen.

### Bestätigte bestehende Sicherheitsmaßnahmen

- AES-256-GCM (Go und PHP); zufällige Nonces/IVs bei Verschlüsselung.
- Config- und .env-Pfade auf das Basisverzeichnis der Anwendung begrenzt
  (Go/PHP).
- Keine hartkodierten Secrets im geprüften Code.
- `go vet ./...` ohne Befund; `go test ./...` erfolgreich.

---

## Security Check 2026-02-28 (Vollständiger Audit)

### Self-Improvement (geprüfte Quellen)

- Geprüft: [OWASP Secure by Design](https://owasp.org/www-project-secure-by-design-framework/)
  (Fokus Design-Phase; Verweis ASVS für Implementierung),
  [CNIL GDPR guide](https://www.cnil.fr/en/gdpr-developers-guide)
  (Sheets 0–16), [EDPB](https://www.edpb.europa.eu/edpb_en)
  (Übersicht), [gdpr-info.eu](https://gdpr-info.eu/) (Rechtstext),
  [Manifestly GDPR-Checkliste](https://www.manifest.ly/use-cases/software-development/gdpr-compliance-checklist)
  (Software-Entwicklung).
- Ergebnis: Ich habe die genannten URLs/Seiten geprüft und keine neuen
  Informationen gefunden, die die bestehenden Prüfschritte dieses Audits
  ändern. OWASP SbD bestätigt die Trennung Design vs. Implementierung; CNIL/EDPB
  bestätigen Datensparsamkeit, Verschlüsselung und Dokumentation.

### Umfang

- **Codebasis:** Go-Bibliothek `github.com/janmz/sconfig` (sconfig.go,
  key_rand_go123.go, i18n.go, version.go, doc.go) und PHP in `php/`
  (EnvLoader.php, I18n.php, helpers.php, example.php).
- **Risiko-Mapping:** Exponierte Eingaben (Config-Pfad Go, .env-Pfad PHP),
  Datei-I/O (os.ReadFile/WriteFile/OpenFile, file/file_put_contents), keine DB,
  kein HTTP-Server; Verschlüsselungs-Key aus Hardware-ID; Debug-Ausgaben;
  Abhängigkeiten (Go stdlib + go-i18n; PHP ext-openssl, Composer php/).
- **Prüfungen:** Scope, Injection/Pfad-Traversal/Secrets/Verschlüsselung,
  Abhängigkeiten (govulncheck, Composer), GDPR-Relevanz.

### Befunde (nach Schwere)

- **Critical – Schlüsselableitung (Go & PHP):** Der Verschlüsselungsschlüssel wird
  deterministisch aus einer hardwarebasierten ID abgeleitet (Go: Go-1.23-kompatibler
  RNG in key_rand_go123.go; PHP: mt_srand($hardwareId) + mt_rand()). **Status:**
  anerkannt (by design). **Nächster Schritt:** Keiner; SECURITY.md und
  securityreport.md dokumentieren dies als akzeptierte Design-Entscheidung.

- **High – Debug-Modus gibt Schlüssel preis (Go):** Bei `debugOutput == true`
  wird der 32-Byte-Verschlüsselungsschlüssel in Hex auf stderr ausgegeben
  (sconfig.go ca. 1073–1075). **Status:** offen. **Nächster Schritt:** Ausgabe
  entfernen oder strikt hinter ein opt-in „Key-Export“-Flag mit deutlichem
  Hinweis legen; nie in Produktion aktivieren.

- **High – Sensible Daten in Debug-Log (Go):** `writeDebugLog` schreibt
  Hardware-Identifikatoren (MAC, machine-id, UUIDs usw.) in
  `sconfig.debug.txt` im Verzeichnis der ausführbaren Datei. **Status:** offen.
  **Nächster Schritt:** Dokumentieren, dass Debug in Produktion verboten ist;
  ggf. Debug-Logs nur außerhalb web-öffentlicher oder weltlesbarer Pfade
  schreiben.

- **High – Config-Pfad nicht validiert (Go):** `LoadConfig(..., path string, ...)`
  und `UpdateConfig(..., path string, ...)` nutzen `path` in `os.Stat`,
  `os.ReadFile`, `os.WriteFile` ohne Bereinigung oder Einschränkung. Nutzer-
  gesteuerter `path` ermöglicht Path-Traversal / beliebiges Lesen/Schreiben.
  **Status:** offen. **Nächster Schritt:** Mit `filepath.Clean` bereinigen, auf
  ein erlaubtes Basisverzeichnis einschränken (z. B. `filepath.Rel`), Pfade
  außerhalb ablehnen.

- **High – .env-Pfad nicht validiert (PHP):** `EnvLoader::load(string $filePath)`
  und `updateEnv(string $filePath, ...)` nutzen `$filePath` in
  `file_exists`, `file()`, `file_put_contents` ohne Normalisierung oder
  Einschränkung. Nutzer-gesteuerter Pfad ermöglicht Path-Traversal. **Status:**
  offen. **Nächster Schritt:** Pfad auflösen (z. B. `realpath` nach
  Einschränkung auf Basis-Pfad), außerhalb liegende Pfade ablehnen;
  dokumentieren, dass `$filePath` nicht aus unvertrauenswürdiger Eingabe
  stammen darf.

- **Medium – Encrypt/Decrypt-Fehlerbehandlung (Go):** Cipher- und GCM-Fehler
  werden in `encrypt()`/`decrypt()` geprüft; Base64 und Länge in `decrypt()`.
  **Status:** behoben.
- **Medium – Config-Dateiberechtigungen (Go):** Schreiben mit `0644`.
  **Status:** anerkannt. **Nächster Schritt:** Optional `0600` oder
  konfigurierbare Rechte dokumentieren.

- **Low – Go-Standardbibliothek:** `govulncheck` konnte in dieser Umgebung
  nicht ausgeführt werden (Tool mit Go 1.24 gebaut, Projekt benötigt go1.25).
  Vorherige Audits meldeten GO-2025-3956 (LookPath), GO-2025-3750 (Windows
  O_CREATE|O_EXCL). **Status:** offen. **Nächster Schritt:** govulncheck mit
  aktuellem Go neu bauen, `govulncheck ./...` lokal ausführen; Go-Toolchain
  ggf. aktualisieren.
- **Low – I18n-/PHP-Pfade und shell_exec:** I18n nutzt feste/relativ-Pfade; PHP
  nutzt `shell_exec()` nur mit festen Befehlen (kein Nutzerinput). **Status:**
  anerkannt.

### Abhängigkeiten (Risiko-Zusammenfassung)

- **Go:** `govulncheck ./...` schlug wegen Versions-Mismatch fehl (Anwendung mit
  Go 1.24, Projekt go1.25). Keine manuell geprüften Drittanbieter-Module
  außer go-i18n (keine bekannten CVEs in diesem Audit). Empfehlung: govulncheck
  mit aktuellem Go neu installieren und lokal ausführen.
- **PHP (Projektroot):** `composer install` und `composer audit` ausgeführt.
  Keine Pakete in require; Audit übersprungen.
- **PHP (php/):** `composer install` und `composer audit` ausgeführt. Keine
  Sicherheitsadvisories gefunden.

### Empfohlene nächste Schritte

1. Schlüssel-Logging in Go entfernen oder strikt absichern; Debug-Key-Export
   nie in Produktion aktivieren.
2. Pfadvalidierung für Config-Pfad (Go) und .env-Pfad (PHP): auflösen,
   auf erlaubtes Basisverzeichnis einschränken, Traversal ablehnen.
3. govulncheck lokal mit passendem Go ausführen und Go-Toolchain ggf. aktualisieren.
4. Schlüsselableitung und Berechtigungen wie dokumentiert/anerkannt beibehalten.

### Bestätigte bestehende Sicherheitsmaßnahmen

- Passwörter in Config/.env verschlüsselt (AES-256-GCM in Go und PHP);
  Klartext nur im Speicher nach Entschlüsselung.
- Go-Debug-Logdatei mit Modus `0600` erstellt.
- PHP: `openssl_encrypt`/`openssl_decrypt` mit AES-256-GCM,
  `openssl_random_pseudo_bytes` für IV; Base64- und Längenprüfung sowie
  Fehlerbehandlung in `decrypt()` vorhanden.
- Keine hartkodierten Secrets oder Credentials im geprüften Code.
- SECURITY.md dokumentiert unterstützte Versionen und private
  Schwachstellenmeldung; hardwaregebundener Schlüssel ist beschrieben.

### GDPR (DSGVO) – Kurzbewertung

- Die Bibliothek verarbeitet Konfigurations- und Umgebungsdaten (inkl.
  Passwörter). Sie implementiert keine Nutzerkonten, Einwilligung oder
  Betroffenenrechte.
- **Rechtmäßigkeit, Fairness, Transparenz:** Nicht evident (Bibliothek
  definiert keine Verarbeitungszwecke; Anwendung verantwortlich).
- **Zweckbindung / Datensparsamkeit:** Compliant: nur für die Anwendung
  nötige Config/Env-Daten werden gelesen und entschlüsselt.
- **Integrität und Vertraulichkeit (Art. 5, 32):** Compliant in der
  Auslegung: Verschlüsselung ruhender Passwörter; keine Klartext-Passwörter
  in Logs; Debug-Schlüssel/Identifikatoren in Produktion deaktivieren.
- **Speicherbegrenzung / Richtigkeit:** Auf Bibliotheksebene nicht evident.
- **Betroffenenrechte (Art. 12–22):** Nicht evident: keine Schnittstellen
  für Auskunft, Berichtigung, Löschung, Übertragbarkeit; Anwendung
  verantwortlich.
- **Rechenschaftspflicht:** Partial: Sicherheits- und Verschlüsselungsansatz
  dokumentiert; Pfadvalidierung und Debug-Modus-Policy für Betreiber
  dokumentieren.

---

## Security Check 2025-02-21 (Full Audit)

### Scope

- **Codebase:** Go library `github.com/janmz/sconfig` (sconfig.go, i18n.go, version.go,
  doc.go) and PHP components in `php/` (EnvLoader.php, I18n.php, helpers.php,
  example.php).
- **Focus:** Handling of secrets/passwords, encryption and key derivation,
  config/INI and JSON handling, file access and permissions, input validation,
  exposure of sensitive data, and dependency vulnerabilities.
- **Tools:** Manual code review, `govulncheck ./...` for Go modules.

### Findings (ordered by severity)

- **Critical – Encryption key derivation (Go & PHP):** The encryption key is
  derived using a non-cryptographic PRNG seeded with the hardware ID. Go uses
  `math/rand.NewSource(hardwareID)` and PHP uses `mt_srand($hardwareId)` plus
  `mt_rand()`. Both are predictable and not suitable for key generation.
  **Status:** open. **Next:** Derive the 32-byte key with a proper KDF (e.g.
  HKDF-SHA256 or similar) from the hardware-ID-derived seed and a fixed
  context string; do not use `math/rand`/`mt_rand` for key bytes.

- **High – Debug mode exposes encryption key (Go):** When `debugOutput` is true,
  the full 32-byte encryption key is printed to stderr in hex (sconfig.go
  around 866–867). Logs or redirected stderr can persist this secret.
  **Status:** open. **Next:** Remove key logging in production code, or gate it
  behind a separate, opt-in “key export” debug flag with a clear warning and
  ensure it is never enabled in production.

- **High – Sensitive data in debug log file (Go):** `writeDebugLog` writes
  hardware identifiers (MAC, machine-id, UUIDs, etc.) to `sconfig.debug.txt` in
  the executable directory when debug is on. This can aid an attacker in
  reproducing or guessing hardware-bound keys.
  **Status:** open. **Next:** Document that debug mode must not be used in
  production; consider writing debug logs only to a path that is not
  world-readable or not under a web-accessible directory.

- **High – Config path not validated (Go):** `LoadConfig(..., path string, ...)`
  uses `path` directly in `os.Stat`, `os.ReadFile`, and `os.WriteFile`. No
  path sanitization, resolution, or containment (e.g. under a config
  directory). If `path` is user-controlled, this allows reading or
  overwriting arbitrary files (path traversal / arbitrary file write).
  **Status:** open. **Next:** Validate and restrict the config path: resolve
  with `filepath.Clean` and ensure it lies under an allowed base directory
  (e.g. application config dir); reject paths containing `..` outside that
  base or use `filepath.Rel` to enforce containment.

- **High – .env path not validated (PHP):** `EnvLoader::load(string $filePath)`
  uses `$filePath` in `file_exists`, `file`, `file_put_contents` without
  normalization or containment checks. User-controlled `$filePath` (e.g. from
  web input) can lead to path traversal (read/write files outside intended
  directory).
  **Status:** open. **Next:** Resolve and validate path (e.g. `realpath` after
  restricting to a base path), reject paths outside the allowed base, and
  document that `$filePath` must not be taken from untrusted input.

- **Medium – Decrypt panic on invalid/short input (Go):** `decrypt()` uses
  `data, _ := base64.StdEncoding.DecodeString(text)` and then
  `data[:nonceSize], data[nonceSize:]`. Failed base64 decode or length &lt;
  nonceSize can cause panic or wrong output.
  **Status:** fixed. **Next:** (none.) Base64 decode error is checked;
  `len(data) >= gcm.NonceSize()` is enforced before slicing; clear errors
  returned.

- **Medium – Encrypt/decrypt error handling (Go):** `encrypt()` and `decrypt()`
  ignore errors from `aes.NewCipher(encryptionKey)` and `cipher.NewGCM(block)`.
  Wrong key length (e.g. not 32 bytes) could lead to panic or undefined
  behavior.
  **Status:** fixed. **Next:** (none.) Both `encrypt()` and `decrypt()` now
  check and return errors from `aes.NewCipher` and `cipher.NewGCM`.

- **Medium – Config file permissions (Go):** Config is written with `0644`
  (world-readable). Content is encrypted, but in locked-down environments
  config files are often expected to be owner-only.
  **Status:** acknowledged. **Next:** Consider writing with `0600` by default,
  or make permissions configurable (e.g. via option or umask) and document
  the choice.

- **Low – Go standard library vulnerabilities:** `govulncheck` reported two
  issues: GO-2025-3956 (LookPath in os/exec) and GO-2025-3750 (O_CREATE|O_EXCL
  on Windows). Call sites include `exec.Command` in hardware-ID logic and
  `os.ReadFile`/`os.WriteFile`/`os.OpenFile` in config and i18n.
  **Status:** open. **Next:** Upgrade to Go 1.23.12+ (or the version that
  includes the fixes for these CVEs) when available and re-run
  `govulncheck ./...`.

- **Low – I18n external translations path (Go):** `loadExternalTranslations()`
  uses `filepath.Glob(filepath.Join("locales", "*.json"))` and then
  `os.ReadFile(file)`. The base is fixed ("locales") and files are limited to
  `*.json`, so risk is low. No path traversal from user input in current
  design.
  **Status:** acknowledged.

- **Low – PHP shell_exec usage:** `EnvLoader` uses `shell_exec()` with
  fixed-format commands (no user input in the command string). Commands are
  appropriate for hardware detection. Risk of command injection is low.
  **Status:** acknowledged.

### Dependency Risk Summary

- **Go:** `govulncheck ./...` reported 2 vulnerabilities in the Go standard
  library (GO-2025-3956, GO-2025-3750). Fix: upgrade to a Go toolchain that
  includes the fixes (e.g. 1.23.12+ where applicable). No vulnerable
  third-party Go packages were reported in the call graph.
- **PHP:** Depends on `ext-openssl`. Composer audit is now required: see
  SECURITY.md and the security-auditor subagent definition (Composer audit
  required when applicable). CI runs `composer audit` for root and `php/` when
  composer.json is present.

### Recommended Next Actions

1. Remove or strictly guard any logging of the encryption key; do not enable
   debug key export in production.
2. Add path validation for config path (Go) and .env path (PHP): resolve,
   restrict to an allowed base directory, and reject traversal.
3. Upgrade Go to a version that fixes GO-2025-3956 and GO-2025-3750 and
   re-run `govulncheck`.
4. (Done) Key derivation: use of `math/rand`/`mt_rand` is by design; see
   “Encryption key derivation” finding (acknowledged). (Done) decrypt/encrypt
   error handling and config file permissions (preserve existing mode) are
   addressed.

### Existing Security Measures Confirmed

- Passwords in config/.env are stored encrypted (AES-256-GCM in both Go and
  PHP); plaintext is only in memory after decryption.
- Go debug log file is created with mode `0600` (owner-only).
- PHP uses `openssl_encrypt`/`openssl_decrypt` with AES-256-GCM and
  `openssl_random_pseudo_bytes` for IV; base64 and length checks in
  `decrypt()` are present.
- No hardcoded secrets or credentials found in the reviewed code.
- SECURITY.md documents supported versions and private vulnerability
  reporting.

### GDPR (DSGVO) – Brief Assessment

- The library processes configuration and environment data (including
  passwords). It does not implement user accounts, consent, or data subject
  rights flows itself.
- **Integrity and confidentiality (Art. 5, 32):** Compliant in design:
  encryption at rest for passwords, no logging of plaintext passwords;
  debug mode key/identifier logging should be disabled in production.
- **Data minimisation / purpose limitation:** Compliant: only config/env
  data necessary for the application is read and decrypted.
- **Rights of data subjects (Art. 12–22):** Not evident: the library does not
  provide access, rectification, erasure, or portability interfaces; that
  remains the responsibility of the application using sconfig.
- **Accountability:** Partial: security and encryption approach are
  documented; recommend documenting path validation and debug-mode policy
  for deployers.

---

## Security Check 2025-02-21 12:00:00

### Scope

- **Codebase:** Go library `github.com/janmz/sconfig` (sconfig.go, i18n.go,
  version.go, doc.go) and PHP in `php/` (php/src/EnvLoader.php, I18n.php,
  helpers.php, example.php). Design documented in SECURITY.md (hardware-bound
  encryption key).
- **Risk mapping:** Exposed inputs (config path, .env path), file I/O
  (ReadFile/WriteFile/OpenFile, file/file_put_contents), no DB or HTTP
  server; encryption key derivation; debug logging; dependencies (Go
  stdlib + go-i18n; PHP ext-openssl + dev tools in php/).
- **Checks performed:** Scope and risk mapping, injection/path
  traversal/secrets/encryption review, dependency checks (Composer
  mandatory; govulncheck attempted), GDPR relevance.

### Findings (ordered by severity)

- **Critical – Encryption key derivation (Go & PHP):** Key is derived with
  non-cryptographic PRNG (Go: `math/rand.NewSource(hardwareID)`; PHP:
  `mt_srand($hardwareId)` + `mt_rand()`). **Status:** acknowledged (by
  design). **Next:** None; SECURITY.md and securityreport.md document this
  as an accepted design choice; security relies on hardware ID secrecy, not
  PRNG strength.

- **High – Debug mode exposes encryption key (Go):** With `debugOutput ==
  true`, the 32-byte encryption key is printed to stderr in hex (sconfig.go
  ~981–982). **Status:** open. **Next:** Remove or strictly gate behind an
  opt-in “key export” flag with explicit warning; never enable in
  production.

- **High – Sensitive data in debug log file (Go):** `writeDebugLog` writes
  hardware identifiers (MAC, machine-id, UUIDs, etc.) to
  `sconfig.debug.txt` in the executable directory when debug is on.
  **Status:** open. **Next:** Document that debug must not be used in
  production; consider writing debug logs only outside web-accessible or
  world-readable paths.

- **High – Config path not validated (Go):** `LoadConfig(..., path string,
  ...)` uses `path` in `os.Stat`, `os.ReadFile`, and `os.WriteFile`
  without sanitization or containment. User-controlled `path` enables path
  traversal / arbitrary file read and write. **Status:** open. **Next:**
  Resolve with `filepath.Clean`, restrict to an allowed base directory
  (e.g. `filepath.Rel`), reject paths outside base.

- **High – .env path not validated (PHP):** `EnvLoader::load(string
  $filePath)` uses `$filePath` in `file_exists`, `file()`,
  `file_put_contents` without normalization or containment. User-controlled
  `$filePath` can lead to path traversal. **Status:** open. **Next:**
  Resolve path (e.g. `realpath` after restricting to a base path), reject
  outside base; document that `$filePath` must not come from untrusted
  input.

- **Medium – Decrypt/encrypt error handling (Go):** Base64 and length checks
  and cipher/GCM error returns are present. **Status:** fixed.
- **Medium – Config file permissions (Go):** Config written with `0644`.
  **Status:** acknowledged. **Next:** Consider `0600` or configurable
  permissions and document.

- **Low – Go standard library vulnerabilities:** Previous audit reported
  GO-2025-3956 (LookPath), GO-2025-3750 (O_CREATE|O_EXCL Windows). **Status:**
  open. **Next:** Upgrade Go to a version that includes fixes; run
  `govulncheck ./...` locally (see Dependency Risk Summary).
- **Low – I18n / PHP paths and shell_exec:** I18n uses fixed or
  package-relative paths; PHP uses `shell_exec()` with fixed commands only.
  **Status:** acknowledged.

### Dependency Risk Summary

- **Go:** `govulncheck ./...` could not be run in this execution environment
  (tool reported “Cursor Sandbox is unsupported” in sandbox; with full
  permissions `govulncheck` was not in PATH). Previous audit (2025-02-21)
  reported two issues in the Go standard library: GO-2025-3956, GO-2025-3750.
  **Recommendation:** Run locally: `go install golang.org/x/vuln/cmd/govulncheck@latest`
  then `govulncheck ./...` (if using vendor, run `go mod vendor` first or use
  `GOFLAGS=-mod=mod`). (with `-mod=mod` or after
  `go mod vendor` if using vendoring). Upgrade Go toolchain when fixes are
  available.
- **PHP (root):** `composer install` and `composer audit` run. No packages
  in require; audit skipped (no advisories to check).
- **PHP (php/):** `composer install` and `composer audit` run. 37 packages
  installed (require-dev: php_codesniffer, php-cs-fixer and dependencies).
  **Result:** No security vulnerability advisories found.

### Recommended Next Actions

1. Remove or strictly gate logging of the encryption key; ensure debug key
   export is never enabled in production.
2. Add path validation for config path (Go) and .env path (PHP): resolve,
   restrict to an allowed base directory, reject traversal.
3. Run `govulncheck ./...` locally and upgrade Go when a fixing version is
   available.
4. Keep key derivation as documented design; maintain existing encrypt/
   decrypt and permission behaviour as acknowledged.

### Existing Security Measures Confirmed

- Passwords in config/.env stored encrypted (AES-256-GCM in Go and PHP);
  plaintext only in memory after decryption.
- Go debug log file created with mode `0600`.
- PHP uses `openssl_encrypt`/`openssl_decrypt` with AES-256-GCM and
  `openssl_random_pseudo_bytes` for IV; base64 and length checks in
  `decrypt()`.
- No hardcoded secrets or credentials in reviewed code.
- SECURITY.md documents supported versions and private vulnerability
  reporting; hardware-bound key design is explained.

### GDPR (DSGVO) – Brief Assessment

- Library processes configuration and environment data (including
  passwords). It does not implement user accounts, consent, or data subject
  rights flows.
- **Lawfulness, fairness, transparency:** Not evident (library does not
  define processing purposes or notices; application responsibility).
- **Purpose limitation / Data minimisation:** Compliant: only config/env
  data needed for the application is read and decrypted.
- **Integrity and confidentiality (Art. 5, 32):** Compliant in design:
  encryption at rest for passwords; no logging of plaintext passwords;
  debug mode key/identifier logging must be disabled in production.
- **Storage limitation / Accuracy:** Not evident at library level.
- **Rights of data subjects (Art. 12–22):** Not evident: no access,
  rectification, erasure, or portability interfaces; application
  responsibility.
- **Accountability:** Partial: security and encryption approach documented;
  path validation and debug-mode policy for deployers should be documented.
