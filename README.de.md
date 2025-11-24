# sconfig

Werkzeuge zum Laden und Pflegen von Konfigurationsdateien mit sicherer Passwortbehandlung. Verfügbar in **Go** (JSON-Konfigurationsdateien) und **PHP** (.env-Dateien).

[![Go Reference](https://pkg.go.dev/badge/github.com/janmz/sconfig.svg)](https://pkg.go.dev/github.com/janmz/sconfig)
[![Go Report Card](https://goreportcard.com/badge/github.com/janmz/sconfig)](https://goreportcard.com/report/github.com/janmz/sconfig)
[![CI](https://github.com/janmz/sconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/janmz/sconfig/actions/workflows/ci.yml)

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

**Wichtig:** Da das Paket noch nicht auf Packagist veröffentlicht ist, musst du zuerst das Repository in deiner `composer.json` hinzufügen:

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

Es wird jedoch empfohlen, das Repository zuerst in deiner `composer.json` hinzuzufügen, dann zu verwenden:

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
    if err := sconfig.LoadConfig(cfg, 1, "config.json", false); err != nil {
        panic(err)
    }
    fmt.Println("starte auf Port", cfg.Port)
}
```

- Beim ersten Lauf mit einem Klartext-`DBPassword` wird die Datei mit einem verschlüsselten `DBSecurePassword` neu geschrieben und `DBPassword` mit einem Marker ersetzt.
- Im Speicher wird `DBPassword` automatisch entschlüsselt (außer `cleanConfig` ist `true`).

### Versionierung

Versionsvariablen können via `-ldflags` zur Build-Zeit überschrieben werden:

```bash
go build -ldflags "-X github.com/janmz/sconfig.Version=1.2.3 -X github.com/janmz/sconfig.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/janmz/sconfig.GitCommit=$(git rev-parse --short HEAD)"
```

## PHP-Variante

### Funktionen

- Einfach und leichtgewichtig
- **Automatische Passwortverschlüsselung/-entschlüsselung** mit hardwaregebundenen Schlüsseln
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

// .env-Datei laden
EnvLoader::load('.env');

// Werte über die env() Helper-Funktion abrufen
$dbHost = env('DB_HOST');
$dbPort = env('DB_PORT', 3306); // mit Standardwert
$apiKey = env('API_KEY');
```

#### Sichere Passwortbehandlung

Die Bibliothek behandelt automatisch die Passwortverschlüsselung für Felder mit den Namen `<NAME>_PASSWORD` und `<NAME>_SECURE_PASSWORD`:

```php
use Sconfig\EnvLoader;

// .env-Datei laden - Passwörter werden automatisch verschlüsselt
EnvLoader::load('.env');

// Entschlüsseltes Passwort abrufen (transparent im Speicher)
$dbPassword = env('DB_PASSWORD'); // Gibt den entschlüsselten Wert zurück
```

**So funktioniert es:**
1. Wenn `DB_PASSWORD` ein Klartext-Passwort enthält (nicht den Marker), wird es verschlüsselt
2. Der verschlüsselte Wert wird in `DB_SECURE_PASSWORD` gespeichert
3. `DB_PASSWORD` wird durch den Marker "Neues Passwort hier eingeben" ersetzt
4. Im Speicher werden Passwörter automatisch entschlüsselt für transparenten Zugriff über `env()`

**Beispiel .env-Datei:**

```env
# Vor dem ersten Laden
DB_PASSWORD=meingeheimespasswort
DB_SECURE_PASSWORD=

# Nach dem ersten Laden (automatisch aktualisiert)
DB_PASSWORD=Neues Passwort hier eingeben
DB_SECURE_PASSWORD=<verschlüsselte_base64_zeichenkette>
```

Die verschlüsselten Passwörter sind rechnergebunden (abgeleitet von Hardware-Identifikatoren), wodurch sie auf anderen Systemen unbrauchbar sind.

#### Laden von benutzerdefiniertem Pfad

```php
use Sconfig\EnvLoader;

// Von einem benutzerdefinierten Pfad laden
EnvLoader::load('/pfad/zur/deiner/.env');
```

#### Überschreiben bestehender Variablen

Standardmäßig werden bestehende Umgebungsvariablen nicht überschrieben. Um sie zu überschreiben:

```php
use Sconfig\EnvLoader;

EnvLoader::load('.env', true); // override = true
```

#### Clean Config Modus

Um Passwörter vor dem Schreiben zu entschlüsseln (mit Vorsicht verwenden, hauptsächlich für Migration oder Inspektion):

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

Die `.env`-Datei sollte diesem Format folgen. Eine Beispieldatei `example_env` ist im `php/` Verzeichnis enthalten:

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

**Hinweis:** Kopiere `php/example_env` nach `.env` in dein Projektverzeichnis und passe es an.

- Zeilen die mit `#` beginnen werden als Kommentare behandelt
- Leere Zeilen werden ignoriert
- Werte können mit einfachen oder doppelten Anführungszeichen versehen werden (Anführungszeichen werden automatisch entfernt)
- Leerzeichen um `=` werden automatisch entfernt
- **Passwort-Paare**: Felder mit den Namen `<NAME>_PASSWORD` und `<NAME>_SECURE_PASSWORD` werden automatisch verschlüsselt

### API-Referenz

#### EnvLoader::load(string $filePath, bool $override = false, bool $cleanConfig = false)

Lädt Umgebungsvariablen aus einer `.env`-Datei und verarbeitet die Passwortverschlüsselung.

**Parameter:**
- `$filePath` - Pfad zur `.env`-Datei
- `$override` - Ob bestehende Umgebungsvariablen überschrieben werden sollen (Standard: false)
- `$cleanConfig` - Wenn true, werden Passwörter vor dem Schreiben entschlüsselt (Standard: false, mit Vorsicht verwenden)

**Wirft:** `RuntimeException` wenn die Datei nicht gelesen werden kann oder die Verschlüsselung fehlschlägt

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

Die Bibliothek unterstützt mehrere Sprachen und erkennt automatisch die Systemsprache aus Umgebungsvariablen (`LANG`, `LC_ALL`, `LC_MESSAGES`). Übersetzungen werden aus dem `locales/` Verzeichnis geladen (gleiche JSON-Dateien wie die Go-Variante).

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

Beide Varianten unterstützen mehrere Sprachen und erkennen automatisch die Systemsprache. Übersetzungen werden aus dem `locales/` Verzeichnis geladen.

### Go

Die Bibliothek bettet Übersetzungen aus `locales/` ein. Eigene Übersetzungen können durch externe Dateien im Verzeichnis `locales` neben dem Binary überschrieben werden.

### PHP

Siehe Abschnitt [PHP Internationalisierung](#php-internationalisierung) oben.

## Sicherheitshinweise

- **Rechnergebundene Verschlüsselung**: Passwörter werden mit Schlüsseln verschlüsselt, die aus Hardware-Identifikatoren abgeleitet werden, wodurch verschlüsselte Konfigurationsdateien auf anderen Systemen unbrauchbar sind
- **Transparente Entschlüsselung**: Passwörter werden automatisch im Speicher entschlüsselt für einfachen Zugriff
- **`cleanConfig` mit Vorsicht verwenden**: Setzen von `cleanConfig = true` schreibt Klartext-Passwörter in die Datei
- **Hardware-ID**: Der Verschlüsselungsschlüssel wird aus System-Hardware-Identifikatoren generiert (MAC-Adresse, CPU-ID, etc.)

## Spenden (Donationware)

Dieses Projekt ist Donation-Ware. Wenn es dir hilft, spende bitte an die CFI Kinderhilfe und unterstütze damit Kinder in Not:

- Jetzt spenden: [https://cfi-kinderhilfe.de/jetzt-spenden/?q=VAYASCFG](Spendenseite von CFI Kinderhilfe)

## Mitwirken

Siehe [CONTRIBUTING.md](CONTRIBUTING.md) und unseren [Code of Conduct](CODE_OF_CONDUCT.md).

## Lizenz

[MIT](LICENSE)
