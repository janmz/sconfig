# sconfig

Werkzeuge zum Laden und Pflegen von JSON-Konfigurationsdateien mit sicherer Passwortbehandlung in Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/janmz/sconfig.svg)](https://pkg.go.dev/github.com/janmz/sconfig)
[![Go Report Card](https://goreportcard.com/badge/github.com/janmz/sconfig)](https://goreportcard.com/report/github.com/janmz/sconfig)
[![CI](https://github.com/janmz/sconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/janmz/sconfig/actions/workflows/ci.yml)

## Installation

```bash
go get github.com/janmz/sconfig
```

## Funktionen

- Befüllung von Standardwerten über Struct-Tags (z. B. `default:"value"`).
- Automatische Synchronisierung eines `Version`-Feldes.
- Transparente Passwortbehandlung mit Paaren `<Name>Password` und `<Name>SecurePassword`.
- Eingebaute i18n-Fehlermeldungen (Englisch als Fallback, Deutsch unterstützt).

## Schnellstart

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

## Internationalisierung

Die Bibliothek bettet Übersetzungen aus `locales/` ein. Eigene Übersetzungen können durch externe Dateien im Verzeichnis `locales` neben dem Binary überschrieben werden.

## Sicherheitshinweise

- Die Verschlüsselung wird aus Hardware-IDs abgeleitet und bindet die Konfigurationsdatei standardmäßig an den Rechner.
- Setze `cleanConfig = true` nur für Migration/Inspektion ein; dadurch werden Passwörter im Klartext in die Datei geschrieben.

## Versionierung

Versionsvariablen können via `-ldflags` zur Build-Zeit überschrieben werden:

```bash
go build -ldflags "-X github.com/janmz/sconfig.Version=1.2.3 -X github.com/janmz/sconfig.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/janmz/sconfig.GitCommit=$(git rev-parse --short HEAD)"
```

## Spenden (Donationware)

Dieses Projekt ist Donation-Ware. Wenn es dir hilft, spende bitte an die CFI Kinderhilfe und unterstütze damit Kinder und in Not:

- Jetzt spenden: [https://cfi-kinderhilfe.de/jetzt-spenden/?q=VAYASCFG](Spendenseite von CFI Kinderhilfe)

## Mitwirken

Siehe [CONTRIBUTING.md](CONTRIBUTING.md) und unseren [Code of Conduct](CODE_OF_CONDUCT.md).

## Lizenz

[MIT](LICENSE)
