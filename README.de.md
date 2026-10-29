# sconfig

Werkzeuge zum Laden und Pflegen von Konfigurationsdateien mit sicherer
Passwortbehandlung. Verfügbar in **Go** (JSON-Konfigurationsdateien) und
**PHP** (.env-Dateien).

[![Go Reference](https://pkg.go.dev/badge/github.com/janmz/sconfig/v3.svg)](https://pkg.go.dev/github.com/janmz/sconfig/v3)
[![Go Report Card](https://goreportcard.com/badge/github.com/janmz/sconfig)](https://goreportcard.com/report/github.com/janmz/sconfig)
[![CI](https://github.com/janmz/sconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/janmz/sconfig/actions/workflows/ci.yml)

[🇬🇧 GB/EN version](README.md)

## Installation

### Go

```bash
go get github.com/janmz/sconfig/v3
```

### PHP

#### Via Packagist (wenn veröffentlicht)

```bash
composer require janmz/sconfig
```

#### Via Git Repository (Entwicklung/Aktuell)

**Wichtig:** Da das Paket noch nicht auf Packagist veröffentlicht ist, musst du
zuerst das Repository in deiner `composer.json` hinzufügen:

```json
{
    "repositories": [
        {
            "type": "vcs",
            "url": "https://github.com/janmz/sconfig"
        }
    ],
    "require": {
        "janmz/sconfig": "dev-main"
    }
}
```

Dann ausführen:

```bash
composer install
```

**Alternative:** Du kannst auch `composer require` mit dem Repository-Flag verwenden:

```bash
composer require janmz/sconfig:dev-main --prefer-source --repository='{"type":"vcs","url":"https://github.com/janmz/sconfig"}'
```

Es wird jedoch empfohlen, das Repository zuerst in deiner `composer.json`
hinzuzufügen, dann zu verwenden:

```bash
composer require janmz/sconfig:dev-main --prefer-source
```

## Upgrade von v2 auf v3

Version 3 ist eine neue Hauptversion. Bestehende Programme nutzen weiter v2,
bis sie ausdrücklich umstellen:

- **Go:** Importpfad auf `github.com/janmz/sconfig/v3` ändern und
  `go get github.com/janmz/sconfig/v3@v3.0.0` ausführen. Programme mit
  `github.com/janmz/sconfig/v2` erhalten weiter v2.
- **PHP:** in `composer.json` `"janmz/sconfig": "^3.0"` verlangen. Eine
  Angabe wie `^2.0` bleibt bei v2.

Was sich für Anwendungen ändert:

- Neue verschlüsselte Werte nutzen das Schlüsselformat v2 (siehe
  [So entsteht die Hardware-ID](#so-entsteht-die-hardware-id)); alte Werte
  werden beim ersten Laden gelesen und automatisch neu geschrieben. Ältere
  Versionen der Bibliothek können die neuen Werte nicht lesen – alle
  Programme, die eine Config-Datei teilen, gleichzeitig aktualisieren.
- Go: ein `locales/`-Verzeichnis im Arbeitsverzeichnis wird nicht mehr
  automatisch geladen (`SetLocalesDir` verwenden); `LoadConfig` gibt Fehler
  zurück, statt den Prozess zu beenden.
- PHP: `set()` wirft `InvalidArgumentException` bei ungültigen Namen und bei
  Werten mit Zeilenumbrüchen; `updateEnv()` schreibt keine Werte der
  Prozessumgebung mehr.

## Go-Variante

### Funktionen

- Befüllung von Standardwerten über Struct-Tags (z. B. `default:"value"`;
  string, bool, Ganzzahlen mit/ohne Vorzeichen, Gleitkommazahlen,
  `time.Duration`), auch für aus der Datei geladene Slice-/Array-/Map-Elemente
  und Pointer-Ziele.
- Automatische Synchronisierung eines `Version`-Feldes auf oberster Ebene
  (verschachtelte Structs behalten ihr eigenes `Version`).
- Transparente Passwortbehandlung mit Paaren `<Name>Password` und
  `<Name>SecurePassword`, auch in verschachtelten Structs, Pointern, Slices,
  Arrays und Maps.
- Atomares Schreiben der Dateien; neue Dateien erhalten die Rechte 0600.
- Sicher bei parallelen Aufrufen (`LoadConfig`/`UpdateConfig` werden
  serialisiert).
- Eingebaute i18n-Fehlermeldungen (Englisch als Fallback, Deutsch unterstützt).

### Schnellstart

```go
package main

import (
    "fmt"
    "github.com/janmz/sconfig/v3"
)

type AppConfig struct {
    Version           int    `json:"Version"`
    DBPassword        string `json:"DBPassword"`
    DBSecurePassword  string `json:"DBSecurePassword"`
    Port              int    `json:"Port" default:"8080"`
}

func main() {
    cfg := &AppConfig{}
    if err := sconfig.LoadConfig(cfg, 1, "config.json", false, false); err != nil {
        panic(err)
    }
    fmt.Println("starte auf Port", cfg.Port)
}
```

- Beim ersten Lauf mit einem Klartext-`DBPassword` wird die Datei mit einem
  verschlüsselten `DBSecurePassword` neu geschrieben und `DBPassword` mit einem
  Marker ersetzt.
- Im Speicher wird `DBPassword` automatisch entschlüsselt (wenn `cleanConfig`
  `true` ist wird es, z.B. für den Wechsel auf eine andere Hardware, auch in
  der Datei im Klartext gespeichert).

**Config-Pfade:** Pfade werden bereinigt und müssen **unterhalb des Verzeichnisses
der ausführbaren Datei, unterhalb des aktuellen Arbeitsverzeichnisses** (das
Aufrufsystem setzt dieses) **oder unterhalb des Benutzerverzeichnisses** liegen
(`%USERPROFILE%` unter Windows, `~` unter Unix, z. B.
`AppData\Roaming\<app>` oder `~/.config/<app>`). Relative Pfade werden gegen
das **aktuelle Arbeitsverzeichnis** aufgelöst (wie Go `filepath.Abs`). Den
`debugOutput`-Parameter nur bei Fehleranalyse nutzen, wenn die ausgegebenen
Angaben (Hardware-ID, Schlüsselmaterial, Pfade) nötig sind; im Normalbetrieb
ausgeschaltet lassen.

### Versionierung

Versionsvariablen können via `-ldflags` zur Build-Zeit überschrieben werden:

```bash
go build -ldflags "-X github.com/janmz/sconfig/v3.Version=3.0.0.65 \
  -X 'github.com/janmz/sconfig/v3.BuildTime=$(date '+%Y-%m-%d %H:%M:%S')'"
```

### Config nach Änderungen zurückschreiben (UpdateConfig)

Wenn die Anwendung Werte aus der Config ändert (z. B. über die Oberfläche), kann
sie die Struct aktualisieren und mit `UpdateConfig` die Datei wieder schreiben.
Secure-Felder werden dabei je nach Parameter verschlüsselt oder entschlüsselt in
der Datei gespeichert.

**Voraussetzung:** Es muss zuvor mindestens einmal `LoadConfig` aufgerufen worden
sein (Initialisierung/Verschlüsselung); andernfalls gibt `UpdateConfig` einen
Fehler zurück.

Die Config kann unter einem **anderen Pfad** als dem Lade-Pfad geschrieben werden
(z. B. zuerst `LoadConfig(cfg, 1, "config.json", ...)`, dann
`UpdateConfig(cfg, "config.backup.json")`).

**Beispiel (Theme von dark auf light):** Struct mit Feld `Theme string` und Tag
`json:"theme"`; Anwendung lädt die Config mit `LoadConfig`, der Nutzer wechselt
in der UI von „dark“ auf „light“, die Anwendung setzt `cfg.Theme = "light"` und
ruft
`sconfig.UpdateConfig(cfg, "config.json")` auf (ohne dritten Parameter =
verschlüsselte Secure-Felder in der Datei). Nach dem Schreiben bleiben die Passwörter
in der Struct weiterhin entschlüsselt (wie nach LoadConfig).

## PHP-Variante

### Funktionen

- Einfach und leichtgewichtig
- **Automatische Passwortverschlüsselung/-entschlüsselung** mit
  hardwaregebundenen Schlüsseln
- **Mehrsprachige Unterstützung** (Englisch, Deutsch) mit automatischer Spracherkennung
- Automatisches Caching geladener Werte
- Unterstützung für Standardwerte
- Integration mit PHP's nativen `$_ENV` und `getenv()`
- PSR-4 Autoloading
- Keine externen Abhängigkeiten (PHP 7.4+)
- AES-256-GCM Verschlüsselung für sichere Passwortspeicherung
- Eingebaute Übersetzungen in `php/locales/` (Kopien der Go-Dateien in `locales/`)

### Schnellstart

#### Grundlegende Verwendung

```php
<?php

require_once 'vendor/autoload.php';

use Sconfig\EnvLoader;

// .env-Datei laden (Pfad unterhalb des Einstiegsskripts; siehe Abschnitt .env-Pfade)
EnvLoader::load('.env');

// Werte über die env() Helper-Funktion abrufen
$dbHost = env('DB_HOST');
$dbPort = env('DB_PORT', 3306); // mit Standardwert
$apiKey = env('API_KEY');
```

#### Sichere Passwortbehandlung

Die Bibliothek behandelt automatisch die Passwortverschlüsselung für Felder mit
den Namen `<NAME>_PASSWORD` und `<NAME>_SECURE_PASSWORD`:

```php
use Sconfig\EnvLoader;

// .env-Datei laden - Passwörter werden automatisch verschlüsselt
EnvLoader::load('.env');

// Entschlüsseltes Passwort abrufen (transparent im Speicher)
$dbPassword = env('DB_PASSWORD'); // Gibt den entschlüsselten Wert zurück
```

**So funktioniert es:**

1. Wenn `DB_PASSWORD` ein Klartext-Passwort enthält (nicht den Marker), wird es
   verschlüsselt
2. Der verschlüsselte Wert wird in `DB_SECURE_PASSWORD` gespeichert
3. `DB_PASSWORD` wird durch den Marker "Neues Passwort hier eingeben" ersetzt
4. Im Speicher werden Passwörter automatisch entschlüsselt für transparenten
   Zugriff über `env()`

**Beispiel .env-Datei:**

```env
# Vor dem ersten Laden
DB_PASSWORD=meingeheimespasswort
DB_SECURE_PASSWORD=

# Nach dem ersten Laden (automatisch aktualisiert)
DB_PASSWORD=Neues Passwort hier eingeben
DB_SECURE_PASSWORD=<verschlüsselte_base64_zeichenkette>
```

Die verschlüsselten Passwörter sind rechnergebunden (abgeleitet von
System-Identifikatoren, siehe
[So entsteht die Hardware-ID](#so-entsteht-die-hardware-id)), wodurch sie auf
anderen Systemen unbrauchbar sind. Von der PHP-Version verschlüsselte Werte
kann die Go-Version auf demselben Rechner entschlüsseln und umgekehrt
(Format v2; Alt-Werte v1 sind nicht austauschbar).

#### `.env`-Pfade

Pfade werden bereinigt und müssen **unterhalb des PHP-Einstiegsskripts, unterhalb
des aktuellen Arbeitsverzeichnisses, unterhalb des Benutzerverzeichnisses**
(`%USERPROFILE%` unter Windows, `~` unter Unix) oder unter der mit
`EnvLoader::setExecutableRoot()` gesetzten Basis aufgelöst werden. Relative Pfade
beziehen sich auf das **aktuelle Arbeitsverzeichnis**.

```php
use Sconfig\EnvLoader;

EnvLoader::setExecutableRoot(__DIR__); // optional, falls die Standardbasis nicht passt
EnvLoader::load('config/.env');
```

#### Überschreiben bestehender Variablen

Standardmäßig werden bestehende Umgebungsvariablen nicht überschrieben. Um sie
zu überschreiben:

```php
use Sconfig\EnvLoader;

EnvLoader::load('.env', true); // override = true
```

#### Config nach Änderungen zurückschreiben (updateEnv)

Nach `load()` können Werte mit `EnvLoader::set('SCHLUESSEL', 'Wert')` geändert werden.
`updateEnv($filePath)` bzw. `updateEnv($filePath, false)` schreibt die .env mit
verschlüsselten Passwörtern zurück, `updateEnv($filePath, true)` mit entschlüsselten.

**Voraussetzung:** Zuerst muss mindestens einmal `load()` aufgerufen worden sein;
andernfalls löst `updateEnv` einen Fehler aus.

Die .env kann unter einem **anderen Pfad** als dem Lade-Pfad geschrieben werden
(z. B. `load('.env')`, dann `updateEnv('backup.env')`).

**Beispiel (Theme von dark auf light):** `.env` mit `THEME=dark`; nach `load('.env')`
zeigt die App das Theme. Nutzer wählt „light“, Anwendung ruft
`EnvLoader::set('THEME', 'light')` und danach `EnvLoader::updateEnv('.env')` auf
(Default = verschlüsselte Passwörter).

#### Clean Config Modus

Um Passwörter vor dem Schreiben zu entschlüsseln (mit Vorsicht verwenden,
hauptsächlich für Migration oder Inspektion):

```php
use Sconfig\EnvLoader;

EnvLoader::load('.env', false, true); // cleanConfig = true
```

Dies schreibt Klartext-Passwörter zurück in die `.env`-Datei.

#### Direkte Klassenverwendung

Du kannst auch die `EnvLoader`-Klasse direkt verwenden:

```php
use Sconfig\EnvLoader;

// Umgebung laden
EnvLoader::load('.env');

// Wert abrufen
$value = EnvLoader::get('KEY', 'default');

// Prüfen ob Schlüssel existiert
if (EnvLoader::has('KEY')) {
    // ...
}
```

### .env Dateiformat

Die `.env`-Datei sollte diesem Format folgen. Eine Beispieldatei `example_env`
ist im `php/` Verzeichnis enthalten:

```env
# Datenbank-Konfiguration
DB_HOST=localhost
DB_PORT=3306
DB_NAME=myapp
DB_USER=root
DB_PASSWORD=secret123
DB_SECURE_PASSWORD=

# API-Konfiguration
API_KEY=dein-api-schlüssel-hier
API_URL=https://api.example.com

# Anwendungseinstellungen
APP_ENV=production
DEBUG=false
```

**Hinweis:** Kopiere `php/example_env` nach `.env` in dein Projektverzeichnis
und passe es an.

- Zeilen die mit `#` beginnen werden als Kommentare behandelt
- Leere Zeilen werden ignoriert
- Werte können mit einfachen oder doppelten Anführungszeichen versehen werden
  (Anführungszeichen werden automatisch entfernt)
- Leerzeichen um `=` werden automatisch entfernt
- **Passwort-Paare**: Felder mit den Namen `<NAME>_PASSWORD` und
  `<NAME>_SECURE_PASSWORD` werden automatisch verschlüsselt

### API-Referenz

#### EnvLoader::load()

```php
EnvLoader::load(string $filePath, bool $override = false,
bool $cleanConfig = false)
```

Lädt Umgebungsvariablen aus einer `.env`-Datei und verarbeitet die Passwortverschlüsselung.

**Parameter:**

- `$filePath` - Pfad zur `.env`-Datei
- `$override` - Ob bestehende Umgebungsvariablen überschrieben werden sollen
                (Standard: false)
- `$cleanConfig` - Wenn true, werden Passwörter vor dem Schreiben entschlüsselt
                   (Standard: false, mit Vorsicht verwenden)

**Wirft:** `RuntimeException` wenn die Datei nicht gelesen werden kann oder die
Verschlüsselung fehlschlägt

#### env(string $key, mixed $default = null)

Globale Helper-Funktion zum Abrufen eines Umgebungsvariablen-Werts.

**Parameter:**

- `$key` - Der Umgebungsvariablen-Schlüssel
- `$default` - Standardwert wenn Schlüssel nicht gefunden wird

**Gibt zurück:** Den Umgebungsvariablen-Wert oder den Standardwert

#### EnvLoader::get(string $key, mixed $default = null)

Einen Umgebungsvariablen-Wert abrufen.

#### EnvLoader::has(string $key): bool

Prüfen ob eine Umgebungsvariable existiert.

#### EnvLoader::clear(): void

Cache leeren und geladenen Zustand zurücksetzen.

#### EnvLoader::isLoaded(): bool

Prüfen ob die Umgebung geladen wurde.

### PHP Internationalisierung

Die Bibliothek unterstützt mehrere Sprachen und erkennt automatisch die
Systemsprache aus Umgebungsvariablen (`LC_ALL` vor `LC_MESSAGES` vor
`LANG`). Die eingebauten Übersetzungen liegen in `php/locales/` (Kopien der
Go-Dateien in `locales/`).

Unterstützte Sprachen:

- Englisch (en) - Standard/Fallback
- Deutsch (de)

Du kannst die Sprache manuell setzen:

```php
use Sconfig\I18n;

I18n::setLanguage('de'); // Zu Deutsch wechseln
$message = I18n::t('config.password_message'); // Übersetzte Nachricht abrufen
```

Zusätzliche oder überschreibende Übersetzungen werden nur aus einem
Verzeichnis geladen, das die Anwendung ausdrücklich übergibt:
`I18n::initialize('pfad/zu/locales')` (vor der ersten Verwendung). Der
Passwort-Marker bleibt immer der eingebaute Text.

## Internationalisierung

Beide Varianten unterstützen mehrere Sprachen und erkennen automatisch die
Systemsprache. Übersetzungen werden aus dem `locales/` Verzeichnis geladen.

### Go

Die Bibliothek bettet Übersetzungen aus `locales/` ein. Zusätzliche oder
überschreibende Übersetzungen werden nur auf Anforderung mit
`sconfig.SetLocalesDir("pfad/zu/locales")` geladen (Dateien `<Sprache>.json`,
gleiches Format; ungültige Dateien liefern einen Fehler und ändern nichts).
Ein `locales`-Verzeichnis der Anwendung wird nicht mehr automatisch geladen.
Der Passwort-Marker bleibt immer der eingebaute Text.

### PHP

Siehe Abschnitt [PHP Internationalisierung](#php-internationalisierung) oben.

## So entsteht die Hardware-ID

Jeder verschlüsselte Wert beginnt mit seinem Format:

- `v2:<Quellen>:<base64>` – aktuelles Format, geschrieben von Go und PHP
  seit Version 3.0.0.
- reines base64 ohne Präfix – Altformat v1 (vor 3.0.0). Es bleibt lesbar und
  wird beim nächsten `LoadConfig` bzw. `EnvLoader::load()` automatisch als v2
  neu geschrieben.

### Aktuelles Format v2 (identisch in Go und PHP)

Der Name „Hardware-ID“ ist historisch. v2 nutzt Identitätsquellen des
Rechners, die stabil und ohne Administratorrechte lesbar sind.
MAC-Adressen, Netzwerkrouten, `ipconfig`, `wmic`, Laufwerks-, CPU- und
Mainboard-Seriennummern, VM-Erkennung, Rechnername und PHP-Version werden
nicht mehr verwendet.

#### Quellen

- **Windows**
  - `machineguid`: Wert `MachineGuid` aus
    `HKLM\SOFTWARE\Microsoft\Cryptography`, gelesen mit
    `%SystemRoot%\System32\reg.exe query … /reg:64` (64-Bit-Registry-Ansicht,
    auch für 32-Bit-Prozesse). Entsteht bei der Windows-Installation.
  - `smbiosuuid`: `Win32_ComputerSystemProduct.UUID`, gelesen mit
    `powershell.exe -NoProfile -NonInteractive -Command
    "(Get-CimInstance -ClassName Win32_ComputerSystemProduct).UUID"`. Gesetzt
    von der Mainboard-Firmware bzw. dem Hypervisor. Der PowerShell-Start
    dauert einmal pro Prozess etwa 0,5–2 s.
- **Linux:** `machineid` – Inhalt von `/etc/machine-id`, Ersatz
  `/var/lib/dbus/machine-id` (Installations-ID, ohne root lesbar).
- **macOS:** `ioplatformuuid` – `IOPlatformUUID` aus
  `/usr/sbin/ioreg -rd1 -c IOPlatformExpertDevice` (Hardware-UUID).
- **FreeBSD:** `hostuuid` – Ausgabe von `/sbin/sysctl -n kern.hostuuid`.
- **Optionale Schlüsseldatei:** `keyfile` – Hex-Inhalt einer geheimen Datei,
  siehe [Optionen](#optionen).
- **Andere Systeme:** keine Quelle; Verschlüsseln scheitert mit
  `config.hardware_id_failed` (es gibt keinen Ersatz).
- **Tests:** Mit Hardware-ID-Überschreibung (`getHardwareID_func` von
  `LoadConfig`, `EnvLoader::setHardwareIdForTest()`) ist die einzige Quelle
  `override=<Zahl>`.

#### Normalisierung

- Werte werden getrimmt und kleingeschrieben.
- UUIDs: geschweifte Klammern werden entfernt; der Wert muss die Form
  `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` (Hex-Ziffern) haben.
  Platzhalter-UUIDs, die Hersteller auf vielen Geräten ausliefern, werden
  ignoriert: `00000000-0000-0000-0000-000000000000`,
  `ffffffff-ffff-ffff-ffff-ffffffffffff`,
  `03000200-0400-0500-0006-000700080009`,
  `00020003-0004-0005-0006-000700080009`.
- machine-id: genau 32 Hex-Ziffern, nicht nur Nullen.
- Eine nicht lesbare oder ungültige Quelle wird weggelassen. Ein Befehl, der
  nicht rechtzeitig fertig wird (15 s, PowerShell 30 s), ist ein Fehler und
  nie eine fehlende Quelle.

#### Schlüssel und gespeicherter Wert

1. Die verwendeten Quellen werden nach Namen sortiert und als
   `<Name>=<Wert>\n` hintereinander geschrieben (Eingangsmaterial).
2. HKDF-SHA256 (RFC 5869) mit Salt `sconfig-v2` und Info
   `sconfig v2 aes-256-gcm` ergibt den 32-Byte-AES-256-Schlüssel. Die volle
   Entropie der Quellen wird genutzt.
3. AES-256-GCM mit zufälliger 12-Byte-Nonce je Wert. Zusätzlich
   authentifizierte Daten: `sconfig:` + Präfix des Passwortfelds,
   kleingeschrieben und nur `a-z` und `0-9` (Go `DatabaseSecurePassword` und
   PHP `DATABASE_SECURE_PASSWORD` ergeben beide `sconfig:database`). Ein Wert
   lässt sich daher nicht in ein Feld mit anderem Namen verschieben.
4. Gespeicherter Wert: `v2:` + Quellnamen, verbunden mit `,` + `:` + base64
   von `Nonce | Chiffrat | Tag`, z. B. `v2:machineguid,smbiosuuid:PW3g…`. Die
   Namen müssen streng aufsteigend sein (höchstens 8 Namen, 256 Zeichen).

Neue Werte nutzen immer alle in dem Moment verfügbaren Quellen. Beim
Entschlüsseln wird der Schlüssel aus genau den im Wert genannten Quellen
abgeleitet. Ist eine davon nicht mehr lesbar, nennt die Fehlermeldung diese
Quelle (`config.v2_source_missing`) statt eines allgemeinen
Entschlüsselungsfehlers. Werte, die weniger Quellen nutzen als jetzt
verfügbar sind (z. B. nach Hinzufügen einer Schlüsseldatei), werden beim
nächsten Laden mit allen Quellen neu verschlüsselt.

#### Was ist wirklich Hardware?

- Aus Firmware bzw. Hypervisor: `smbiosuuid` (Windows), `ioplatformuuid`
  (macOS), `hostuuid` (FreeBSD; vom Betriebssystem erzeugt, wenn die Firmware
  keine liefert).
- Installations-IDs des Betriebssystems: `machineguid` (Windows) und
  `machineid` (Linux). Unter Linux hängt der Schlüssel daher an der
  Installation: ein kopiertes Plattenimage oder eine geklonte VM behält die
  ID, solange `/etc/machine-id` nicht neu erzeugt wird.
- Alle Quellen sind für jeden lokalen Prozess lesbar. Der Schutz besteht
  darin: Wer nur die Config-Datei hat (Backup, Repository, Support-Ticket),
  kann sie nicht entschlüsseln; wer Zugriff auf den Rechner hat, kann es.
  Gegen Angreifer, die beliebige Dateien des Rechners lesen können
  (Datei-Leselücke), und in Containern (machine-id fehlt oder ist im Image
  eingebrannt) hilft eine Schlüsseldatei außerhalb des erreichbaren
  Bereichs, siehe [Optionen](#optionen).

#### Migration von v1-Werten

Beim Laden wird jeder v1-Wert mit dem Altschlüssel entschlüsselt, als v2
verschlüsselt und die Datei neu geschrieben. Ist keine v2-Quelle verfügbar,
bleibt der v1-Wert erhalten. Vor der Migration erstellte Backups enthalten
weiter v1-Werte mit kleinem Schlüsselraum (siehe unten) – Passwörter wechseln,
falls solche Kopien in fremde Hände gelangt sein können.

#### Optionen

- Go: `sconfig.SetKeyFile(pfad)`, `sconfig.SetRejectLegacy(true)`,
  `sconfig.SetWarningHandler(func(msg string) { … })` (Standard: stderr).
- PHP: `EnvLoader::setKeyFile($pfad)`, `EnvLoader::setRejectLegacy(true)`,
  `EnvLoader::setWarningHandler(fn (string $msg) => …)` (Standard:
  `error_log()`), `EnvLoader::setExportPasswords(false)`.
- **Schlüsseldatei** (`SetKeyFile` / `setKeyFile`): fügt die Quelle
  `keyfile` hinzu. Eine fehlende Datei wird mit 32 Zufallsbytes und den
  Rechten 0600 angelegt (Go unter Windows: Zugriff nur für Eigentümer,
  SYSTEM und Administratoren). Außerhalb der Anwendungs- und Webverzeichnisse
  ablegen und sichern: damit verschlüsselte Werte lassen sich ohne sie nicht
  entschlüsseln. Go und PHP können dieselbe Datei nutzen.
- **Strikter Modus** (`SetRejectLegacy` / `setRejectLegacy`): Alt-Werte v1
  werden abgelehnt statt entschlüsselt und migriert.
- **Warnungen** gibt es nach der Migration von v1-Werten (diese Passwörter
  wechseln), wenn eine Migration nicht möglich war, wenn eine vorhandene
  Config-Datei für andere Benutzer lesbar ist (Unix) und (PHP) wenn die
  `.env`-Datei unter `$_SERVER['DOCUMENT_ROOT']` liegt.
- **Exceptions und Fehlermeldungen** enthalten Dateipfade und Feldnamen.
  Protokollieren, aber Nutzern einer Webanwendung nicht ungefiltert zeigen.

### Altformat v1 (nur lesen)

Werte ohne Präfix wurden von Versionen vor 3.0.0 verschlüsselt. Go und PHP
nutzten unterschiedliche, nicht kompatible Verfahren. Beide sind eingefroren
(Known-Answer-Tests in beiden Testsuiten) und dienen nur noch zum
Entschlüsseln und Migrieren. Seit 3.0.0 lässt ein externer Befehl, der länger
als 15 s hängt, die v1-Berechnung scheitern, statt still eine andere ID zu
liefern.

#### Go (v1)

##### Schritt 1 – VM-Erkennung (Go)

- **Windows:** `wmic computersystem get Manufacturer,Model /value`. VM, wenn
  Hersteller oder Modell (kleingeschrieben) `vmware`, `virtualbox`,
  `microsoft corporation`, `xen`, `parallels`, `qemu`, `kvm`, `innotek` oder
  `bochs` enthält oder das Modell `virtual` enthält. Physische
  Microsoft-Geräte (z. B. Surface, Hersteller „Microsoft Corporation“) gelten
  daher als VM. Ohne `wmic` ist das Ergebnis immer „keine VM“.
- **Linux:** VM, wenn `systemd-detect-virt` etwas anderes als `none` ausgibt
  oder `/sys/class/dmi/id/sys_vendor`, `product_name` bzw. `chassis_vendor`
  `qemu`, `kvm`, `vmware`, `virtualbox`, `xen`, `parallels`, `microsoft`,
  `bochs` oder `bhyve` enthält.
- **Alle anderen Systeme** (macOS, BSD, …): nie VM.

##### Schritt 2 – MAC-Adresse (Go, höchstens eine)

- **Windows:** `ipconfig /all` wird ausgewertet. Jede Zeile, die auf `:`
  endet (außer Zeilen mit `Windows IP` oder `Configuration`), beginnt einen
  neuen Adapterblock. Die MAC stammt aus der Zeile, die mit
  `Physical Address`, `Physische Adresse` oder `Physikalische Adresse`
  beginnt. Ein Adapter gilt als „mit Gateway“, sobald er eine Zeile hat, die
  mit `Default Gateway` oder `Standardgateway` beginnt – egal ob eine Adresse
  (IPv4 oder IPv6) eingetragen ist, denn die Füllpunkte `. . .` enthalten
  immer einen Punkt. Es gewinnt der **letzte** Adapter der Ausgabe mit einer
  solchen Zeile und einer MAC. Nur englische und deutsche
  `ipconfig`-Ausgaben werden erkannt; andere Anzeigesprachen landen immer im
  Fallback.
- **Linux:** die Schnittstelle nach `dev` in der Ausgabe von
  `ip route get 8.8.8.8`; deren MAC aus Gos `net.Interfaces()`.
- **macOS:** die Schnittstelle nach `interface:` in der Ausgabe von
  `route -n get 8.8.8.8`; deren MAC aus `net.Interfaces()`.
- **Fallback** (oben nichts gefunden oder anderes System): alle
  Schnittstellen aus `net.Interfaces()` mit MAC – auch getrennte und
  virtuelle Adapter – werden aufsteigend als Zeichenketten sortiert; die
  erste wird verwendet.
- **Normalisierung:** Kleinbuchstaben, `-` durch `:` ersetzt, Leerzeichen
  entfernt (z. B. `a1:b2:c3:d4:e5:f6`).

##### Schritt 3 – Systemspezifische Identifikatoren (Go)

- **Windows, nur VM:**
  - `MachineGuid` aus
    `reg query HKLM\SOFTWARE\Microsoft\Cryptography /v MachineGuid` (Wert
    nach `REG_SZ`). Sie entsteht bei der Windows-Installation – eine
    Betriebssystem-ID, keine Hardware.
  - SMBIOS-UUID aus `wmic csproduct get UUID /value`; `_` durch `-` ersetzt,
    abschließende `.`, `!` und Leerzeichen entfernt.
- **Windows, immer:**
  - `wmic baseboard get SerialNumber` – zweite Ausgabezeile, getrimmt.
  - `wmic baseboard get Product` – zweite Ausgabezeile, getrimmt
    (Mainboard-Modellname, nicht rechnerindividuell).
  - `wmic diskdrive get SerialNumber` – von allen nicht leeren Werten der in
    Zeichenkettenordnung **größte**. Ein neu angeschlossenes Laufwerk (z. B.
    USB) kann ihn daher ersetzen.
- **Windows, nur physische Rechner:** `wmic cpu get ProcessorId` – der in
  Zeichenkettenordnung kleinste Wert. Die ProcessorId ist CPUID-Signatur und
  Feature-Maske und bei allen CPUs desselben Modells gleich.
- **Linux, nur VM:** Inhalt von `/etc/machine-id` (Installations-ID des
  Betriebssystems) und `/sys/class/dmi/id/product_uuid`.
- **Linux, nur physische Rechner:** alle Zeilen aus `/proc/cpuinfo` mit
  `Serial`, Wert nach `:`; Duplikate entfernt, der kleinste wird verwendet
  (nur auf manchen ARM-Systemen wie dem Raspberry Pi vorhanden).
- **Linux, immer:** `/sys/class/dmi/id/board_serial`.
- **macOS und alle anderen Systeme:** keine weiteren Identifikatoren – die ID
  besteht **nur aus der MAC-Adresse**.

##### Schritt 4 – Hash (Go)

- Gar kein Identifikator: Fehler `config.hardware_id_failed`; `LoadConfig`
  beendet den Prozess per `log.Fatalf`.
- Die Identifikatoren werden aufsteigend (byteweise) sortiert, mit `|`
  verbunden und mit SHA-256 gehasht. Die ersten 8 Bytes des Hashs, gelesen
  als **Little-Endian**, ergeben die `uint64`-Hardware-ID.

##### Schritt 5 – Schlüssel (Go)

Startwert ist die Hardware-ID mit gelöschtem obersten Bit
(`& 0x7fffffffffffffff`) für eine eingefrorene Kopie der Go-1.23-
`math/rand`-Quelle (`key_rand_go123.go`). Wie das Original reduziert diese
Quelle den Startwert intern auf `seed % (2^31 - 1)` (Ergebnis 0 wird zu
`89482311`); es gibt daher nur etwa 2^31 verschiedene Schlüssel, unabhängig
von der 64-Bit-Hardware-ID. 32 Aufrufe von `byte(Int63() >> 16)` ergeben den
256-Bit-Schlüssel für AES-256-GCM (zufällige 12-Byte-Nonce je
Verschlüsselung).

#### PHP (v1)

##### Schritt 1 – VM-Erkennung (PHP)

Gleiche Regeln wie in Go (`wmic`, `systemd-detect-virt`, DMI-Dateien),
ausgeführt als externe Befehle. Geprüft werden nur Windows und Linux; alle
anderen Systeme gelten nie als VM.

##### Schritt 2 – Identifikatoren (in Erfassungsreihenfolge, unsortiert)

- **Windows:**
  1. Nur VM: `MachineGuid` (Regex `MachineGuid\s+REG_SZ\s+(.+)`).
  2. Nur VM: SMBIOS-UUID aus `wmic csproduct get UUID /value`, unverändert.
  3. MAC: erste Spalte der ersten Zeile von `getmac /fo csv /nh`, unverändert
     (Großbuchstaben mit `-`). Das ist der erste von Windows gelistete
     Adapter, **unabhängig davon, ob er verbunden ist**.
  4. Erster nicht leerer Wert von `wmic baseboard get SerialNumber /value`.
  5. Erster nicht leerer Wert von `wmic baseboard get Product /value`.
  6. Erster nicht leerer Wert von `wmic diskdrive get SerialNumber /value`
     (erstes Laufwerk in `wmic`-Reihenfolge, unsortiert).
  7. Nur physische Rechner: erster Wert von
     `wmic cpu get ProcessorId /value`.
- **Alle anderen Systeme** (Linux, macOS, BSD, …):
  1. Nur Linux-VM: `/etc/machine-id` und `/sys/class/dmi/id/product_uuid`.
  2. MAC: `cat /sys/class/net/*/address | head -1` – die alphabetisch erste
     Schnittstelle (kann `docker0`, eine Bridge oder `lo` mit
     `00:00:00:00:00:00` sein). Unter macOS existiert dieser Pfad nicht.
  3. `cat /sys/class/dmi/id/board_serial`.
  4. Nur physische Rechner: die komplette erste Zeile aus `/proc/cpuinfo`
     mit `Serial`, inklusive Bezeichnung (z. B. `Serial : 0000…`).
- Werte, die PHPs `empty()` als leer ansieht (auch `"0"`), werden
  übersprungen.
- **Fallback:** Wurde kein Identifikator gefunden, werden `gethostname()` und
  `PHP_VERSION` verwendet – **überhaupt keine Hardware**. Umbenennen des
  Rechners oder ein PHP-Update ändert den Schlüssel (typisch unter macOS).

##### Schritt 3 – Hash (PHP)

Die Identifikatoren werden in Erfassungsreihenfolge mit `|` verbunden und mit
SHA-256 gehasht. Die ersten 8 Bytes, gelesen als **Big-Endian**, ergeben
eine vorzeichenbehaftete 64-Bit-Ganzzahl.

##### Schritt 4 – Schlüssel (PHP)

`mt_srand(<Hardware-ID>)` – PHPs Mersenne Twister verwendet nur die
**unteren 32 Bit** des Startwerts. 32-mal `(mt_rand() ^ mt_rand()) & 0xff`
ergibt den AES-256-GCM-Schlüssel.

#### Go und PHP in v1

Identifikatoren, Reihenfolge, MAC-Format, Byte-Reihenfolge,
Schlüsselerzeugung und Chiffrat-Aufbau unterscheiden sich (Go v1:
`Nonce | Chiffrat | Tag`, PHP v1: `IV | Tag | Chiffrat`). v1-Werte der einen
Implementierung kann die andere nicht entschlüsseln. v2-Werte sind voll
kompatibel.

#### Hardware und Schwächen von v1

- **Hardware** (vom Hersteller bzw. Hypervisor gesetzt):
  Baseboard-Seriennummer und -Produkt, Laufwerks-Seriennummer,
  CPU-ProcessorId bzw. -Seriennummer, SMBIOS- bzw. Product-UUID, MAC-Adresse
  (Werkswert, aber per Software änderbar; virtuelle Adapter haben generierte
  MACs).
- **Keine Hardware:** `MachineGuid`, `/etc/machine-id`
  (Betriebssysteminstallation), welcher Adapter ein Gateway hat
  (Netzwerkzustand), der PHP-Fallback (Rechnername, PHP-Version) und die
  Go-Überschreibfunktion.
- **Folgen:**
  - Aktuelle Windows-11-Versionen installieren `wmic` standardmäßig nicht
    mehr. Dann verwendet ein physischer Windows-PC **nur die MAC-Adresse**
    (Go: aktiver Adapter; PHP: erster Adapter aus `getmac`); dasselbe gilt
    für macOS in Go. Eine MAC ist für jedes Gerät im lokalen Netzsegment
    sichtbar und ihr Herstellerpräfix öffentlich – der Schutz gegenüber
    jemandem, der die MAC kennt, ist dann gering.
  - Unter Linux sind `board_serial` und `product_uuid` meist nur für root
    lesbar. Ein Lauf als root bzw. als normaler Benutzer ergibt daher
    unterschiedliche IDs.
  - **Kleiner Schlüsselraum:** Go nutzt nur etwa 2^31, PHP 2^32 verschiedene
    Schlüssel. Wer nur die verschlüsselte Datei hat, kann alle in wenigen
    Stunden durchprobieren – ohne die Hardware zu kennen.

## Hardware-ID / Schlüsseländerungen debuggen

Wenn sich der hardware-abgeleitete Verschlüsselungsschlüssel ändert (Passwörter
lassen sich auf derselben Maschine nicht mehr entschlüsseln), hilft Folgendes.

### Warum sich der Schlüssel ändern kann

v2-Werte (siehe [Quellen](#quellen)) ändern sich nur, wenn sich eine Quelle
ändert:

- **Windows:** Windows neu installiert oder Sysprep (neue `MachineGuid`),
  Mainboard getauscht, Firmware oder Hypervisor hat die SMBIOS-UUID geändert.
  Fehlt PowerShell oder ist es gesperrt, ist nur `machineguid` verfügbar;
  Werte, die `smbiosuuid` nennen, melden dann die fehlende Quelle.
- **Linux:** neue `/etc/machine-id` (Neuinstallation,
  `systemd-machine-id-setup`, für das Klonen vorbereitetes Image), Container
  ohne machine-id.
- **macOS / FreeBSD:** Logicboard getauscht / neue `kern.hostuuid`.

Netzwerkänderungen, USB-Laufwerke, `wmic`, VM-Erkennung und Prozessrechte
wirken sich auf v2 nicht mehr aus. Alt-Werte v1 (bis zu ihrer Migration)
hängen weiter an den v1-Identifikatoren:

- **Netzwerk:** Windows (Go): ein anderer Adapter hat eine Gateway-Zeile oder
  steht als letzter in der Ausgabe (VPN an/aus, WLAN vs. LAN,
  Hyper-V-/VirtualBox-Adapter) oder `ipconfig` gibt eine andere Sprache als
  Englisch/Deutsch aus. Linux/macOS (Go): die Route zu `8.8.8.8` nutzt eine
  andere Schnittstelle oder es gibt keine Route (offline) und die
  Fallback-MAC greift. PHP: die Reihenfolge von `getmac` bzw.
  `/sys/class/net` ändert sich.
- **Werkzeuge und Rechte:** `wmic` installiert oder entfernt
  (Windows-Update), Prozess läuft mit anderen Rechten (Linux root vs.
  Benutzer).
- **Hardware:** Mainboard, Laufwerk (unter Windows/Go auch Anstecken eines
  USB-Laufwerks), CPU oder Netzwerkkarte getauscht.
- **Betriebssystem:** Neuinstallation, Sysprep, Klonen, neue
  `/etc/machine-id` (nur VMs), geändertes Ergebnis der VM-Erkennung.
- **PHP-Fallback:** Rechner umbenannt oder PHP aktualisiert.

### So debuggen Sie

1. **Aktuelle Hardware-ID und alle Eingaben ausgeben (ohne Config-Datei):**

   ```go
   id, err := sconfig.DebugHardwareID()
   // v1-Identifikatoren, die v1-ID und die v2-Quellen gehen nach stderr.
   ```

2. **Oder** `LoadConfig` mit Debug aufrufen:

   ```go
   err := sconfig.LoadConfig(&cfg, version, path, false, true) // 5. Parameter = debugOutput
   ```

   Dieselben Debug-Zeilen gehen nach stderr.

3. Einmal ausführen, wenn der Schlüssel „falsch“ ist, und stderr speichern. Mit
   einem Lauf vergleichen, in dem der Schlüssel „richtig“ war (oder auf einer
   anderen Maschine). Die Zeilen `[sconfig DEBUG]` zeigen die v2-Quellen
   (`Name=Wert`) und für Alt-Werte VM-Erkennung, MAC und jeden
   v1-Identifikator; die abweichende(n) Zeile(n) sind die Ursache.

4. **Hardware-ID-Track** (v1): Bei aktiviertem Debug wird jede v1-Berechnung
   und jeder Entschlüsselungsfehler als Zeile in `sconfig.debug.txt` im
   Verzeichnis des Executables geschrieben. Format:
   `YYYY-MM-DD HH:MM:SS<TAB>Hardware-ID (hex)<TAB>Identifikatoren`. So
   entsteht eine Chronik der IDs (z. B. nach einem fehlgeschlagenen Entschlüsseln).

## Sicherheitshinweise

- **Rechnergebundene Verschlüsselung**: Passwörter werden mit AES-256-GCM
  verschlüsselt; der Schlüssel wird per HKDF-SHA256 aus Identitätsquellen des
  Rechners abgeleitet. Verschlüsselte Config-Dateien sind auf anderen Systemen
  unbrauchbar. Siehe
  [So entsteht die Hardware-ID](#so-entsteht-die-hardware-id).
- **Bedrohungsmodell**: Die Verschlüsselung schützt Config-Dateien, die den
  Rechner verlassen (Backups, Repositories, Tickets). Jeder Prozess auf dem
  Rechner kann die Quellen lesen und damit den Schlüssel ableiten.
- **Alt-Werte**: v1-Werte haben einen kleinen Schlüsselraum (ca. 2^31 / 2^32)
  und werden beim Laden zu v2 migriert. Passwörter wechseln, falls alte Kopien
  in fremde Hände gelangt sein können.
- **Transparente Entschlüsselung**: Passwörter werden automatisch im Speicher
  entschlüsselt für einfachen Zugriff
- **`cleanConfig` mit Vorsicht verwenden**: Setzen von `cleanConfig = true`
  schreibt Klartext-Passwörter in die Datei; neue Dateien werden mit den
  Rechten 0600 angelegt und alle Dateien atomar geschrieben.
- **Debug-Ausgabe** (`debugOutput`, `DebugHardwareID`) gibt Identifikatoren
  und Schlüsselmaterial aus; nur zur Diagnose verwenden.
- **Schlüsseldatei** gegen Datei-Leselücken und für Container, siehe
  [Optionen](#optionen).
- **PHP:** `set()` lehnt Variablennamen mit anderen Zeichen als Buchstaben,
  Ziffern, `_` und `.` sowie Werte mit Steuerzeichen ab (Zeilenumbrüche
  würden weitere Variablen in die Datei einschleusen). Entschlüsselte
  Passwörter werden nach `$_ENV`/`putenv()` exportiert, außer mit
  `EnvLoader::setExportPasswords(false)`. Die `.env`-Datei außerhalb des
  Dokumentverzeichnisses des Webservers ablegen (`setExecutableRoot()`).

## Spenden (Donationware)

Dieses Projekt ist Donation-Ware. Wenn es dir hilft, spende bitte an die
CFI Kinderhilfe und unterstütze damit Kinder in Not:

- Jetzt spenden: [Spendenseite von CFI Kinderhilfe](https://cfi-kinderhilfe.de/jetzt-spenden/?q=VAYASCFG)

## Mitwirken

Siehe [CONTRIBUTING.md](CONTRIBUTING.md) und unseren [Code of Conduct](CODE_OF_CONDUCT.md).

## Lizenz

[MIT](LICENSE)
