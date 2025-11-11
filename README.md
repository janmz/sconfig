# sconfig

Utilities for loading and maintaining JSON configuration files with secure password handling in Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/janmz/sconfig.svg)](https://pkg.go.dev/github.com/janmz/sconfig)
[![Go Report Card](https://goreportcard.com/badge/github.com/janmz/sconfig)](https://goreportcard.com/report/github.com/janmz/sconfig)
[![CI](https://github.com/janmz/sconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/janmz/sconfig/actions/workflows/ci.yml)

[🇩🇪 Deutsche Version](README.de.md)

## Install

```bash
go get github.com/janmz/sconfig
```

## Features

- Default value population via struct field tags (e.g., `default:"value"`).
- Automatic version synchronization of a `Version` field.
- Transparent password handling using `<Name>Password` and `<Name>SecurePassword` pairs.
- Embedded i18n strings for errors (English fallback, German supported).

## Quick Start

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
    fmt.Println("listening on", cfg.Port)
}
```

- On first run with a plaintext `DBPassword`, the file is rewritten with an encrypted `DBSecurePassword` and a marker in `DBPassword`.
- In memory, `DBPassword` is automatically decrypted for use (unless `cleanConfig` is set to `true`).

## Internationalization

The package embeds translations from `locales/`. You can override by placing external files in a `locales` directory next to your binary.

## Security Notes

- Encryption is derived from hardware identifiers, making encrypted config files machine-bound by default.
- Use `cleanConfig = true` only for migration or inspection; it writes plaintext passwords to the file.

## Versioning

Version info variables are exposed for convenience and can be overridden via `-ldflags`:

```bash
go build -ldflags "-X github.com/janmz/sconfig.Version=1.2.3 -X github.com/janmz/sconfig.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/janmz/sconfig.GitCommit=$(git rev-parse --short HEAD)"
```

## Donationware

This project is provided as donationware. If it helps you, please consider donating to CFI Kinderhilfe to support children in need:

- Donate: [https://cfi-kinderhilfe.de/jetzt-spenden/?q=VAYASCFG](Donation page of CFI Kinderhilfe - in German!)

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
