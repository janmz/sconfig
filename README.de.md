# sconfig

Werkzeuge zum Laden und Pflegen von Konfigurationsdateien mit sicherer
Passwortbehandlung. Verfügbar in **Go** (JSON-Konfigurationsdateien) und
**PHP** (.env-Dateien).

[![Go Reference](https://pkg.go.dev/badge/github.com/janmz/sconfig.svg)](https://pkg.go.dev/github.com/janmz/sconfig)
[![Go Report Card](https://goreportcard.com/badge/github.com/janmz/sconfig)](https://goreportcard.com/report/github.com/janmz/sconfig)
[![CI](https://github.com/janmz/sconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/janmz/sconfig/actions/workflows/ci.yml)

[🇬🇧 GB/EN version](README.md)

## Installation

### Go

```bash
go get github.com/janmz/sconfig
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

## Go-Variante

### Funktionen

- Befüllung von Standardwerten über Struct-Tags (z. B. `default:"value"`).
- Automatische Synchronisierung eines `Version`-Feldes.
- Transparente Passwortbehandlung mit Paaren `<Name>Password` und `<Name>SecurePassword`.
- Eingebaute i18n-Fehlermeldungen (Englisch als Fallback, Deutsch unterstützt).

### Schnellstart

```go
package main

import (
    "fmt"
    "github.com/janmz/sconfig"
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
der ausführbaren Datei oder unterhalb des aktuellen Arbeitsverzeichnisses** liegen
(das Aufrufsystem setzt dieses). Relative Pfade werden gegen das **aktuelle
Arbeitsverzeichnis** aufgelöst (wie Go `filepath.Abs`). Den `debugOutput`-Parameter nur bei Fehleranalyse
nutzen, wenn die ausgegebenen Angaben (Hardware-ID, Schlüsselmaterial, Pfade)
nötig sind; im Normalbetrieb ausgeschaltet lassen.

### Versionierung

Versionsvariablen können via `-ldflags` zur Build-Zeit überschrieben werden:

```bash
go build -ldflags "-X github.com/janmz/sconfig.Version=1.2.3 -X github.com/janmz/sconfig.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/janmz/sconfig.GitCommit=$(git rev-parse --short HEAD)"
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
- Übersetzungen werden aus dem `locales/` Verzeichnis geladen (gleich wie Go-Variante)

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

Die verschlüsselten Passwörter sind rechnergebunden (abgeleitet von Hardware-
Identifikatoren), wodurch sie auf anderen Systemen unbrauchbar sind.

#### `.env`-Pfade

Pfade werden bereinigt und müssen **unterhalb des PHP-Einstiegsskripts, unterhalb
des aktuellen Arbeitsverzeichnisses** oder unter der mit
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
Systemsprache aus Umgebungsvariablen (`LANG`, `LC_ALL`, `LC_MESSAGES`).
Übersetzungen werden aus dem `locales/` Verzeichnis geladen (gleiche
JSON-Dateien wie die Go-Variante).

Unterstützte Sprachen:

- Englisch (en) - Standard/Fallback
- Deutsch (de)

Du kannst die Sprache manuell setzen:

```php
use Sconfig\I18n;

I18n::setLanguage('de'); // Zu Deutsch wechseln
$message = I18n::t('config.password_message'); // Übersetzte Nachricht abrufen
```

Die Bibliothek findet das `locales/` Verzeichnis automatisch durch Suche in:

1. Übergeordnetes Verzeichnis des PHP-Pakets (`../locales`)
2. Projekt-Root (`./locales`)
3. Aktuelles Arbeitsverzeichnis (`./locales`)

## Internationalisierung

Beide Varianten unterstützen mehrere Sprachen und erkennen automatisch die
Systemsprache. Übersetzungen werden aus dem `locales/` Verzeichnis geladen.

### Go

Die Bibliothek bettet Übersetzungen aus `locales/` ein. Eigene Übersetzungen
können durch externe Dateien im Verzeichnis `locales` neben dem Binary
überschrieben werden.

### PHP

Siehe Abschnitt [PHP Internationalisierung](#php-internationalisierung) oben.

## Hardware-ID / Schlüsseländerungen debuggen

Wenn sich der hardware-abgeleitete Verschlüsselungsschlüssel ändert (Passwörter
lassen sich auf derselben Maschine nicht mehr entschlüsseln), hilft Folgendes.

### Warum sich der Schlüssel ändern kann

Der Schlüssel ist ein Hash aus mehreren Identifikatoren. Jede Änderung in Menge
oder Reihenfolge ändert den Schlüssel:

- **Netzwerk (MAC-Adresse)**  
  - **Windows:** Der „aktive“ Adapter ist der mit Standardgateway
   (`ipconfig /all`). Wechsel (VPN an/aus, anderer NIC, WLAN vs. LAN) oder
   fehlgeschlagene Erkennung → Fallback „erste MAC (sortiert)“; die Reihenfolge
   der Interfaces kann sich ändern → andere MAC.  
  - **Linux:** Es wird die Schnittstelle für `ip route get 8.8.8.8` genutzt.
    Ändert sich das (z. B. VPN, zweite NIC) oder greift der Fallback
    „erste MAC (sortiert)“, kann sich die MAC ändern.
- **Windows:** MachineGuid (VMs), SMBIOS-UUID (VMs), Baseboard-Seriennummer/
  Produkt, Festplatten-Seriennummer (erste nach Sortierung), CPU ProcessorId
  (nur physisch). Neuinstallation, Sysprep oder geänderte Laufwerke/Reihenfolge
  können diese ändern.
- **Linux:** `machine-id`, `product_uuid` (VMs), `board_serial`, CPU-Seriennummer
  (nur physisch). Klonen, Neuinstallation oder anderes `/etc/machine-id` ändert
  den Schlüssel.

### So debuggen Sie

1. **Aktuelle Hardware-ID und alle Eingaben ausgeben (ohne Config-Datei):**

   ```go
   id, err := sconfig.DebugHardwareID()
   // Alle Identifikatoren und die finale ID gehen nach stderr.
   ```

2. **Oder** `LoadConfig` mit Debug aufrufen:

   ```go
   err := sconfig.LoadConfig(&cfg, version, path, false, true) // 5. Parameter = debugOutput
   ```

   Dieselben Debug-Zeilen gehen nach stderr.

3. Einmal ausführen, wenn der Schlüssel „falsch“ ist, und stderr speichern. Mit
   einem Lauf vergleichen, in dem der Schlüssel „richtig“ war (oder auf einer
   anderen Maschine). Die Zeilen `[sconfig DEBUG]` zeigen VM-Erkennung,
   verwendete MAC und jeden Identifikator vor dem Hashing; die abweichende(n)
   Zeile(n) sind die Ursache.

4. **Hardware-ID-Track**: Bei aktiviertem Debug wird jede Hardware-ID-Berechnung
   und jeder Entschlüsselungsfehler als Zeile in `sconfig.debug.txt` im
   Verzeichnis des Executables geschrieben. Format:
   `YYYY-MM-DD HH:MM:SS<TAB>Hardware-ID (hex)<TAB>Identifikatoren`. So entsteht eine
   Chronik der IDs (z. B. nach einem fehlgeschlagenen Entschlüsseln).

## Sicherheitshinweise

- **Rechnergebundene Verschlüsselung**: Passwörter werden mit Schlüsseln
  verschlüsselt, die aus Hardware-Identifikatoren abgeleitet werden, wodurch
  verschlüsselte Konfigurationsdateien auf anderen Systemen unbrauchbar sind
- **Transparente Entschlüsselung**: Passwörter werden automatisch im Speicher
  entschlüsselt für einfachen Zugriff
- **`cleanConfig` mit Vorsicht verwenden**: Setzen von `cleanConfig = true`
  schreibt Klartext-Passwörter in die Datei
- **Hardware-ID**: Der Verschlüsselungsschlüssel wird aus System-Hardware-
  Identifikatoren generiert (MAC-Adresse, CPU-ID, etc.)

## Spenden (Donationware)

Dieses Projekt ist Donation-Ware. Wenn es dir hilft, spende bitte an die
CFI Kinderhilfe und unterstütze damit Kinder in Not:

- Jetzt spenden: [Spendenseite von CFI Kinderhilfe](https://cfi-kinderhilfe.de/jetzt-spenden/?q=VAYASCFG)

## Mitwirken

Siehe [CONTRIBUTING.md](CONTRIBUTING.md) und unseren [Code of Conduct](CODE_OF_CONDUCT.md).

## Lizenz

[MIT](LICENSE)
