# sconfig

Utilities for loading and maintaining configuration files with secure password
handling. Available in **Go** (JSON config files) and **PHP** (.env files).

[![Go Reference](https://pkg.go.dev/badge/github.com/janmz/sconfig/v3.svg)](https://pkg.go.dev/github.com/janmz/sconfig/v3)
[![Go Report Card](https://goreportcard.com/badge/github.com/janmz/sconfig)](https://goreportcard.com/report/github.com/janmz/sconfig)
[![CI](https://github.com/janmz/sconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/janmz/sconfig/actions/workflows/ci.yml)

[🇩🇪 Deutsche Version](README.de.md)

## Installation

### Go

```bash
go get github.com/janmz/sconfig/v3
```

### PHP

#### Via Packagist (when published)

```bash
composer require janmz/sconfig
```

#### Via Git Repository (development/current)

**Important:** Since the package is not yet published on Packagist, you must
first add the repository to your `composer.json`:

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

Then run:

```bash
composer install
```

**Alternative:** You can also use `composer require` with the repository flag:

```bash
composer require janmz/sconfig:dev-main --prefer-source --repository='{"type":"vcs","url":"https://github.com/janmz/sconfig"}'
```

However, it's recommended to add the repository to your `composer.json` first,
then use:

```bash
composer require janmz/sconfig:dev-main --prefer-source
```

## Upgrading from v2 to v3

Version 3 is a new major version. Existing programs keep using v2 until
they switch explicitly:

- **Go:** change the import path to `github.com/janmz/sconfig/v3` and run
  `go get github.com/janmz/sconfig/v3@v3.0.0`. Programs importing
  `github.com/janmz/sconfig/v2` keep getting v2.
- **PHP:** require `"janmz/sconfig": "^3.0"` in `composer.json`. A
  constraint such as `^2.0` keeps v2.

What changes for applications:

- New encrypted values use the v2 key format (see
  [How the hardware ID is created](#how-the-hardware-id-is-created)); old
  values are read and rewritten automatically on the first load. Older
  versions of the library cannot read the new values, so update all
  programs that share a config file at the same time.
- Go: a `locales/` directory in the working directory is no longer loaded
  automatically (use `SetLocalesDir`); `LoadConfig` returns errors instead
  of terminating the process.
- PHP: `set()` throws `InvalidArgumentException` for invalid names and for
  values with line breaks; `updateEnv()` no longer writes values of the
  process environment.

## Go Version

### Features

- Default value population via struct field tags (e.g., `default:"value"`;
  string, bool, signed/unsigned integers, floats, `time.Duration`), also for
  slice/array/map elements and pointer targets loaded from the file.
- Automatic version synchronization of a top-level `Version` field (nested
  structs keep their own `Version`).
- Transparent password handling using `<Name>Password` and
  `<Name>SecurePassword` pairs, also in nested structs, pointers, slices,
  arrays and maps.
- Atomic file writes; new files get permissions 0600.
- Safe for concurrent calls (`LoadConfig`/`UpdateConfig` are serialized).
- Embedded i18n strings for errors (English fallback, German supported).

### Quick Start

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
    fmt.Println("listening on", cfg.Port)
}
```

- On first run with a plaintext `DBPassword`, the file is rewritten with an
  encrypted `DBSecurePassword` and a marker in `DBPassword`.
- In memory, `DBPassword` is automatically decrypted for use (unless
  `cleanConfig` is set to `true`).

**Config paths:** Paths are cleaned and must lie **under the directory of the
running executable, under the process current working directory** (the
caller controls the latter) **or under the user's home directory**
(`%USERPROFILE%` on Windows, `~` on Unix, e.g. `AppData\Roaming\<app>` or
`~/.config/<app>`). Relative paths are resolved against the **current
working directory** (same as Go `filepath.Abs`). Use `debugOutput` only when diagnosing
failures and you need the printed values (hardware ID, key material, paths); keep
it off in normal operation.

### Versioning

Version info variables are exposed for convenience and can be overridden via
`-ldflags`:

```bash
go build -ldflags "-X github.com/janmz/sconfig/v3.Version=3.0.0.65 \
  -X 'github.com/janmz/sconfig/v3.BuildTime=$(date '+%Y-%m-%d %H:%M:%S')'"
```

### Writing back config changes (UpdateConfig)

When the application changes config values (e.g. via the UI), it can update the
struct and write the file back with `UpdateConfig`. Secure fields are encrypted
or decrypted in the file depending on the optional `cleanConfig` parameter.

**Requirement:** `LoadConfig` must have been called at least once beforehand
(initialisation/encryption); otherwise `UpdateConfig` returns an error.

The config can be written to a **different path** than the load path (e.g. first
`LoadConfig(cfg, 1, "config.json", ...)`, then
`UpdateConfig(cfg, "config.backup.json")`).

**Example (theme from dark to light):** Struct with field `Theme string` and tag
`json:"theme"`; application loads config with `LoadConfig`, user switches theme
from "dark" to "light" in the UI, application sets `cfg.Theme = "light"` and calls
`sconfig.UpdateConfig(cfg, "config.json")` (no third parameter = encrypted secure
fields in the file). After writing, passwords in the struct remain decrypted (as
after LoadConfig).

## PHP Version

### Features

- Simple and lightweight
- **Automatic password encryption/decryption** with hardware-bound keys
- **Multilingual support** (English, German) with automatic language detection
- Automatic caching of loaded values
- Support for default values
- Integration with PHP's native `$_ENV` and `getenv()`
- PSR-4 autoloading
- No external dependencies (PHP 7.4+)
- AES-256-GCM encryption for secure password storage
- Built-in translations in `php/locales/` (copies of the Go `locales/` files)

### Quick Start

#### Basic Usage

```php
<?php

require_once 'vendor/autoload.php';

use Sconfig\EnvLoader;

// Load .env file (path must be under the entry script directory; see note below)
EnvLoader::load('.env');

// Access values using the env() helper function
$dbHost = env('DB_HOST');
$dbPort = env('DB_PORT', 3306); // with default value
$apiKey = env('API_KEY');
```

#### Secure Password Handling

The library automatically handles password encryption for fields named
`<NAME>_PASSWORD` and `<NAME>_SECURE_PASSWORD`:

```php
use Sconfig\EnvLoader;

// Load .env file - passwords will be automatically encrypted
EnvLoader::load('.env');

// Access decrypted password (transparent in memory)
$dbPassword = env('DB_PASSWORD'); // Returns decrypted value
```

**How it works:**

1. If `DB_PASSWORD` contains a plaintext password (not the marker), it will be encrypted
2. The encrypted value is stored in `DB_SECURE_PASSWORD`
3. `DB_PASSWORD` is replaced with the marker "Enter new password here"
4. In memory, passwords are automatically decrypted for transparent access via `env()`

**Example .env file:**

```env
# Before first load
DB_PASSWORD=mysecretpassword
DB_SECURE_PASSWORD=

# After first load (automatically updated)
DB_PASSWORD=Enter new password here
DB_SECURE_PASSWORD=<encrypted_base64_string>
```

The encrypted passwords are machine-bound (derived from system identifiers,
see [How the hardware ID is created](#how-the-hardware-id-is-created)),
making them unusable on other systems. Values encrypted by the PHP version
can be decrypted by the Go version on the same machine and vice versa (v2
format; legacy v1 values are not interchangeable).

#### `.env` paths

Paths are cleaned and must resolve **under the directory of the PHP entry script,
under the current working directory, under the user's home directory**
(`%USERPROFILE%` on Windows, `~` on Unix) or under the directory passed to
`EnvLoader::setExecutableRoot()` at bootstrap. Relative paths are resolved
against the **current working directory**.

```php
use Sconfig\EnvLoader;

EnvLoader::setExecutableRoot(__DIR__); // optional, if the default base is wrong
EnvLoader::load('config/.env');
```

#### Override Existing Variables

By default, existing environment variables are not overridden. To override them:

```php
use Sconfig\EnvLoader;

EnvLoader::load('.env', true); // override = true
```

#### Writing back env changes (updateEnv)

After `load()`, values can be changed with `EnvLoader::set('KEY', 'value')`.
`updateEnv($filePath)` or `updateEnv($filePath, false)` writes the .env with
encrypted passwords; `updateEnv($filePath, true)` writes with decrypted passwords.

**Requirement:** `load()` must have been called at least once beforehand; otherwise
`updateEnv` throws an error.

The .env can be written to a **different path** than the load path (e.g.
`load('.env')`, then `updateEnv('backup.env')`).

**Example (theme from dark to light):** .env with `THEME=dark`; after `load('.env')`
the app shows the theme. User selects "light", application calls
`EnvLoader::set('THEME', 'light')` and then `EnvLoader::updateEnv('.env')` (default
= encrypted passwords).

#### Clean Config Mode

To decrypt passwords before writing (use with care, primarily for migration
or inspection):

```php
use Sconfig\EnvLoader;

EnvLoader::load('.env', false, true); // cleanConfig = true
```

This will write plaintext passwords back to the `.env` file.

#### Direct Class Usage

You can also use the `EnvLoader` class directly:

```php
use Sconfig\EnvLoader;

// Load environment
EnvLoader::load('.env');

// Get value
$value = EnvLoader::get('KEY', 'default');

// Check if key exists
if (EnvLoader::has('KEY')) {
    // ...
}
```

### .env File Format

The `.env` file should follow this format. An example file `example_env` is
included in the `php/` directory:

```env
# Database Configuration
DB_HOST=localhost
DB_PORT=3306
DB_NAME=myapp
DB_USER=root
DB_PASSWORD=secret123
DB_SECURE_PASSWORD=

# API Configuration
API_KEY=your-api-key-here
API_URL=https://api.example.com

# Application Settings
APP_ENV=production
DEBUG=false
```

**Note:** Copy `php/example_env` to `.env` in your project directory and
customize it.

- Lines starting with `#` are treated as comments
- Empty lines are ignored
- Values can be quoted with single or double quotes (quotes are
  automatically removed)
- Spaces around `=` are automatically trimmed
- **Password pairs**: Fields named `<NAME>_PASSWORD` and `<NAME>_SECURE_PASSWORD`
  are automatically encrypted

### API Reference

#### EnvLoader::load()

```php
EnvLoader::load(string $filePath, bool $override = false,
bool $cleanConfig = false)
```

Loads environment variables from a `.env` file and processes password encryption.

**Parameters:**

- `$filePath` - Path to the `.env` file
- `$override` - Whether to override existing environment variables (default: false)
- `$cleanConfig` - If true, decrypts passwords before writing (default: false,
  use with care)

**Throws:** `RuntimeException` if the file cannot be read or encryption fails

#### env(string $key, mixed $default = null)

Global helper function to get an environment variable value.

**Parameters:**

- `$key` - The environment variable key
- `$default` - Default value if key is not found

**Returns:** The environment variable value or default

#### EnvLoader::get(string $key, mixed $default = null)

Get an environment variable value.

#### EnvLoader::has(string $key): bool

Check if an environment variable exists.

#### EnvLoader::clear(): void

Clear the cache and reset loaded state.

#### EnvLoader::isLoaded(): bool

Check if environment has been loaded.

### PHP Internationalization

The library supports multiple languages and automatically detects the system
language from environment variables (`LC_ALL` before `LC_MESSAGES` before
`LANG`). The built-in translations are shipped in `php/locales/` (copies of
the Go `locales/` files).

Supported languages:

- English (en) - default/fallback
- German (de)

You can manually set the language:

```php
use Sconfig\I18n;

I18n::setLanguage('de'); // Switch to German
$message = I18n::t('config.password_message'); // Get translated message
```

Additional or overriding translations are only loaded from a directory that
the application passes explicitly: `I18n::initialize('path/to/locales')`
(before the first use). The password marker always stays the built-in text.

## Internationalization

Both versions support multiple languages and automatically detect the system
language. Translations are loaded from the `locales/` directory.

### Go

The package embeds translations from `locales/`. Additional or overriding
translations are loaded only on request with
`sconfig.SetLocalesDir("path/to/locales")` (files `<lang>.json`, same format;
invalid files return an error and change nothing). A `locales` directory of
the application is no longer loaded automatically. The password marker always
stays the built-in text.

### PHP

See [PHP Internationalization](#php-internationalization) section above.

## How the hardware ID is created

Every encrypted value starts with its format:

- `v2:<sources>:<base64>` – current format, written by Go and PHP since
  version 3.0.0.
- plain base64 without prefix – legacy format v1 (before 3.0.0). It is still
  readable and is rewritten as v2 automatically by the next `LoadConfig` or
  `EnvLoader::load()`.

### Current format v2 (identical in Go and PHP)

The name "hardware ID" is historical. v2 uses machine identity sources that
are stable and readable without administrator rights. MAC addresses, network
routes, `ipconfig`, `wmic`, disk/CPU/mainboard serials, VM detection, the
host name and the PHP version are no longer used.

#### Sources

- **Windows**
  - `machineguid`: value `MachineGuid` of
    `HKLM\SOFTWARE\Microsoft\Cryptography`, read with
    `%SystemRoot%\System32\reg.exe query … /reg:64` (64-bit registry view,
    also for 32-bit processes). Created when Windows is installed.
  - `smbiosuuid`: `Win32_ComputerSystemProduct.UUID`, read with
    `powershell.exe -NoProfile -NonInteractive -Command
    "(Get-CimInstance -ClassName Win32_ComputerSystemProduct).UUID"`. Set by
    the mainboard firmware or the hypervisor. The PowerShell start takes
    about 0.5–2 s once per process.
- **Linux:** `machineid` – content of `/etc/machine-id`, fallback
  `/var/lib/dbus/machine-id` (OS installation ID, readable without root).
- **macOS:** `ioplatformuuid` – `IOPlatformUUID` from
  `/usr/sbin/ioreg -rd1 -c IOPlatformExpertDevice` (hardware UUID).
- **FreeBSD:** `hostuuid` – output of `/sbin/sysctl -n kern.hostuuid`.
- **Optional key file:** `keyfile` – hex of a secret file, see
  [Options](#options).
- **Other systems:** no source; encrypting fails with
  `config.hardware_id_failed` (there is no fallback).
- **Tests:** with a hardware-ID override (`getHardwareID_func` of
  `LoadConfig`, `EnvLoader::setHardwareIdForTest()`) the only source is
  `override=<number>`.

#### Normalization

- Values are trimmed and converted to lower case.
- UUIDs: braces are removed; the value must have the form
  `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` (hex digits). Placeholder UUIDs
  that vendors ship on many machines are ignored:
  `00000000-0000-0000-0000-000000000000`,
  `ffffffff-ffff-ffff-ffff-ffffffffffff`,
  `03000200-0400-0500-0006-000700080009`,
  `00020003-0004-0005-0006-000700080009`.
- machine-id: exactly 32 hex digits, not all zero.
- A source that cannot be read or is invalid is left out. A command that
  does not finish in time (15 s, PowerShell 30 s) is an error, never a
  missing source.

#### Key and stored value

1. The used sources are sorted by name and written as `<name>=<value>\n`
   one after another (input keying material).
2. HKDF-SHA256 (RFC 5869) with salt `sconfig-v2` and info
   `sconfig v2 aes-256-gcm` yields the 32-byte AES-256 key. The full entropy
   of the sources is used.
3. AES-256-GCM with a random 12-byte nonce per value. Additional
   authenticated data: `sconfig:` + the password field prefix in lower case
   with only `a-z` and `0-9` (Go `DatabaseSecurePassword` and PHP
   `DATABASE_SECURE_PASSWORD` both give `sconfig:database`). A value can
   therefore not be moved to a field with another name.
4. Stored value: `v2:` + source names joined by `,` + `:` + base64 of
   `nonce | ciphertext | tag`, e.g. `v2:machineguid,smbiosuuid:PW3g…`. The
   names must be strictly ascending (at most 8 names, 256 characters).

New values always use all sources available at that moment. Decryption
derives the key from exactly the sources named in the value. If one of them
cannot be read any more, the error names that source
(`config.v2_source_missing`) instead of a generic decryption failure.
Values that use fewer sources than are available now (e.g. after adding a key
file) are re-encrypted with all sources on the next load.

#### What is really hardware?

- From firmware or hypervisor: `smbiosuuid` (Windows), `ioplatformuuid`
  (macOS), `hostuuid` (FreeBSD; generated by the OS if the firmware has
  none).
- OS installation IDs: `machineguid` (Windows) and `machineid` (Linux). On
  Linux the key is therefore bound to the installation: a copied disk image
  or cloned VM keeps the ID unless `/etc/machine-id` is regenerated.
- All sources are readable by every local process. The protection is: a
  person who only has the config file (backup, repository, support ticket)
  cannot decrypt it; a person with access to the machine can. Against
  attackers who can read arbitrary files of the machine (file disclosure
  bug) and in containers (machine-id missing or baked into the image) use a
  key file that is kept outside the reachable area, see [Options](#options).

#### Migration of v1 values

On load, each v1 value is decrypted with the legacy key, encrypted as v2 and
the file is rewritten. If no v2 source is available, the v1 value is kept.
Backups made before the migration still contain v1 values, whose key space
is small (see below) – rotate passwords if such copies may have leaked.

#### Options

- Go: `sconfig.SetKeyFile(path)`, `sconfig.SetRejectLegacy(true)`,
  `sconfig.SetWarningHandler(func(msg string) { … })` (default: stderr).
- PHP: `EnvLoader::setKeyFile($path)`, `EnvLoader::setRejectLegacy(true)`,
  `EnvLoader::setWarningHandler(fn (string $msg) => …)` (default:
  `error_log()`), `EnvLoader::setExportPasswords(false)`.
- **Key file** (`SetKeyFile` / `setKeyFile`): adds the source `keyfile`. A
  missing file is created with 32 random bytes and permissions 0600 (Go on
  Windows: access for owner, SYSTEM and Administrators only). Keep it outside
  the application and web directories and back it up: values encrypted with
  it cannot be decrypted without it. Go and PHP can share the same file.
- **Strict mode** (`SetRejectLegacy` / `setRejectLegacy`): legacy v1 values
  are refused instead of being decrypted and migrated.
- **Warnings** are issued after migrating v1 values (rotate these passwords),
  when a migration was not possible, when an existing config file is
  readable by other users (Unix) and (PHP) when the `.env` file lies inside
  `$_SERVER['DOCUMENT_ROOT']`.
- **Exceptions and errors** contain file paths and field names. Log them,
  but do not show them unfiltered to users of a web application.

### Legacy format v1 (read only)

Values without prefix were encrypted by versions before 3.0.0. Go and PHP
used different, incompatible derivations. Both are frozen (Known-Answer-Tests
in both test suites) and only used to decrypt and migrate. Since 3.0.0 an
external command that hangs longer than 15 s makes the v1 computation fail
instead of silently producing another ID.

#### Go (v1)

##### Step 1 – VM detection (Go)

- **Windows:** `wmic computersystem get Manufacturer,Model /value`. VM if the
  manufacturer or model (lower case) contains `vmware`, `virtualbox`,
  `microsoft corporation`, `xen`, `parallels`, `qemu`, `kvm`, `innotek` or
  `bochs`, or if the model contains `virtual`. Physical Microsoft devices
  (e.g. Surface, manufacturer "Microsoft Corporation") are therefore treated
  as VMs. Without `wmic` the result is always "no VM".
- **Linux:** VM if `systemd-detect-virt` prints anything other than `none`,
  or if `/sys/class/dmi/id/sys_vendor`, `product_name` or `chassis_vendor`
  contains `qemu`, `kvm`, `vmware`, `virtualbox`, `xen`, `parallels`,
  `microsoft`, `bochs` or `bhyve`.
- **All other systems** (macOS, BSD, …): never a VM.

##### Step 2 – MAC address (Go, at most one)

- **Windows:** `ipconfig /all` is parsed. Each line ending in `:` (except
  lines containing `Windows IP` or `Configuration`) starts a new adapter
  block. The MAC is taken from the line starting with `Physical Address`,
  `Physische Adresse` or `Physikalische Adresse`. An adapter counts as
  "having a gateway" as soon as it has a line starting with `Default Gateway`
  or `Standardgateway` – whether or not an address (IPv4 or IPv6) is set,
  because the dot leaders `. . .` always contain a dot. The **last** adapter
  in the output that has such a line and a MAC wins. Only English and German
  `ipconfig` output is recognised; other display languages always use the
  fallback.
- **Linux:** the interface after `dev` in the output of
  `ip route get 8.8.8.8`; its MAC is taken from Go's `net.Interfaces()`.
- **macOS:** the interface after `interface:` in the output of
  `route -n get 8.8.8.8`; its MAC is taken from `net.Interfaces()`.
- **Fallback** (nothing found above, or any other system): all interfaces
  from `net.Interfaces()` that have a MAC – including disconnected and
  virtual adapters – are sorted ascending as strings; the first one is used.
- **Normalisation:** lower case, `-` replaced by `:`, spaces removed
  (e.g. `a1:b2:c3:d4:e5:f6`).

##### Step 3 – System-specific identifiers (Go)

- **Windows, VM only:**
  - `MachineGuid` from
    `reg query HKLM\SOFTWARE\Microsoft\Cryptography /v MachineGuid` (value
    after `REG_SZ`). It is created when Windows is installed – an OS ID, not
    hardware.
  - SMBIOS UUID from `wmic csproduct get UUID /value`; `_` replaced by `-`,
    trailing `.`, `!` and spaces removed.
- **Windows, always:**
  - `wmic baseboard get SerialNumber` – second output line, trimmed.
  - `wmic baseboard get Product` – second output line, trimmed (mainboard
    model name, not unique per machine).
  - `wmic diskdrive get SerialNumber` – of all non-empty values the
    **largest** in string order is used. A newly attached drive (e.g. USB)
    can therefore replace it.
- **Windows, physical machines only:** `wmic cpu get ProcessorId` – the
  smallest value in string order. The ProcessorId is the CPUID signature and
  feature mask and is identical for all CPUs of the same model.
- **Linux, VM only:** content of `/etc/machine-id` (OS installation ID) and
  `/sys/class/dmi/id/product_uuid`.
- **Linux, physical machines only:** all lines of `/proc/cpuinfo` containing
  `Serial`, value after `:`; duplicates removed, the smallest is used (only
  present on some ARM systems such as the Raspberry Pi).
- **Linux, always:** `/sys/class/dmi/id/board_serial`.
- **macOS and all other systems:** no further identifiers – the ID consists
  of the **MAC address only**.

##### Step 4 – Hash (Go)

- No identifier at all: error `config.hardware_id_failed`; `LoadConfig`
  terminates the process via `log.Fatalf`.
- The identifiers are sorted ascending (byte-wise), joined with `|` and
  hashed with SHA-256. The first 8 bytes of the hash, read as
  **little-endian**, form the `uint64` hardware ID.

##### Step 5 – Key (Go)

The seed is the hardware ID with the top bit cleared
(`& 0x7fffffffffffffff`) for a frozen copy of the Go 1.23 `math/rand` source
(`key_rand_go123.go`). Like the original, this source reduces the seed
internally to `seed % (2^31 - 1)` (a result of 0 becomes `89482311`), so only
about 2^31 different keys exist, regardless of the 64-bit hardware ID.
32 calls of `byte(Int63() >> 16)` yield the 256-bit key for AES-256-GCM
(random 12-byte nonce per encryption).

#### PHP (v1)

##### Step 1 – VM detection (PHP)

Same rules as in Go (`wmic`, `systemd-detect-virt`, DMI files), executed as
external commands. Only Windows and Linux are checked; all other systems never
count as VM.

##### Step 2 – Identifiers (in collection order, not sorted)

- **Windows:**
  1. VM only: `MachineGuid` (regex `MachineGuid\s+REG_SZ\s+(.+)`).
  2. VM only: SMBIOS UUID from `wmic csproduct get UUID /value`, unchanged.
  3. MAC: first column of the first line of `getmac /fo csv /nh`, unchanged
     (upper case with `-`). This is the first adapter listed by Windows,
     **regardless of whether it is connected**.
  4. First non-empty value of `wmic baseboard get SerialNumber /value`.
  5. First non-empty value of `wmic baseboard get Product /value`.
  6. First non-empty value of `wmic diskdrive get SerialNumber /value`
     (first drive in `wmic` order, not sorted).
  7. Physical machines only: first value of `wmic cpu get ProcessorId /value`.
- **All other systems** (Linux, macOS, BSD, …):
  1. Linux VM only: `/etc/machine-id` and `/sys/class/dmi/id/product_uuid`.
  2. MAC: `cat /sys/class/net/*/address | head -1` – the first interface in
     alphabetical order (may be `docker0`, a bridge or `lo` with
     `00:00:00:00:00:00`). This path does not exist on macOS.
  3. `cat /sys/class/dmi/id/board_serial`.
  4. Physical machines only: the complete first line of `/proc/cpuinfo`
     containing `Serial`, including the label (e.g. `Serial : 0000…`).
- Values that PHP's `empty()` treats as empty (including `"0"`) are skipped.
- **Fallback:** if no identifier was found, `gethostname()` and `PHP_VERSION`
  are used – **no hardware at all**. Renaming the host or updating PHP
  changes the key (typical on macOS).

##### Step 3 – Hash (PHP)

The identifiers are joined with `|` in collection order and hashed with
SHA-256. The first 8 bytes, read as **big-endian**, form a signed 64-bit
integer.

##### Step 4 – Key (PHP)

`mt_srand(<hardware ID>)` – PHP's Mersenne Twister uses only the **lower
32 bits** of the seed. 32 times `(mt_rand() ^ mt_rand()) & 0xff` yield the
AES-256-GCM key.

#### Go and PHP in v1

Identifiers, order, MAC format, byte order, key expansion and ciphertext
layout differ (Go v1: `nonce | ciphertext | tag`, PHP v1:
`iv | tag | ciphertext`). v1 values of one implementation cannot be decrypted
by the other. v2 values are fully compatible.

#### Hardware and weaknesses of v1

- **Hardware** (set by the manufacturer or hypervisor): baseboard serial and
  product, disk serial, CPU ProcessorId / serial, SMBIOS / product UUID, MAC
  address (factory value, but changeable in software; virtual adapters have
  generated MACs).
- **Not hardware:** `MachineGuid`, `/etc/machine-id` (OS installation),
  which adapter has a gateway (network state), the PHP fallback (host name,
  PHP version) and the Go override function.
- **Consequences:**
  - Current Windows 11 versions no longer install `wmic` by default. Then a
    physical Windows PC uses **only the MAC address** (Go: active adapter;
    PHP: first adapter from `getmac`); the same applies to macOS in Go. A
    MAC is visible to every device on the local network segment and its
    vendor prefix is public, so the protection against someone who knows
    the MAC is low.
  - On Linux, `board_serial` and `product_uuid` are usually readable by root
    only. Running as root or as a normal user therefore yields different IDs.
  - **Small key space:** Go uses only about 2^31 and PHP 2^32 different keys.
    Someone who has only the encrypted file can try all of them in a few
    hours, without knowing the hardware.

## Debugging hardware ID / key changes

If the hardware-derived encryption key changes (passwords no longer decrypt on
the same machine), the following can help.

### Why the key can change

v2 values (see [Sources](#sources)) only change when a source changes:

- **Windows:** Windows reinstalled or Sysprep (new `MachineGuid`), mainboard
  replaced, firmware or hypervisor changed the SMBIOS UUID. If PowerShell is
  missing or blocked, only `machineguid` is available; values that list
  `smbiosuuid` then report the missing source.
- **Linux:** new `/etc/machine-id` (reinstall, `systemd-machine-id-setup`,
  image prepared for cloning), container without machine-id.
- **macOS / FreeBSD:** logic board replaced / new `kern.hostuuid`.

Network changes, USB drives, `wmic`, VM detection and process rights no
longer affect v2. Legacy v1 values (until they are migrated) still depend on
the v1 identifiers:

- **Network:** Windows (Go): another adapter has a gateway line or is listed
  last (VPN on/off, Wi‑Fi vs. Ethernet, Hyper-V/VirtualBox adapters), or
  `ipconfig` output in a language other than English/German. Linux/macOS
  (Go): the route to `8.8.8.8` uses another interface, or there is no route
  (offline) and the fallback MAC is used. PHP: the order of `getmac` or of
  `/sys/class/net` changes.
- **Tools and rights:** `wmic` installed or removed (Windows update), process
  running with different rights (Linux root vs. user).
- **Hardware:** mainboard, disk (also attaching a USB drive on Windows/Go),
  CPU or network card changed.
- **OS:** reinstall, Sysprep, cloning, new `/etc/machine-id` (VMs only), VM
  detection result changes.
- **PHP fallback:** host renamed or PHP updated.

### How to debug

1. **Print current hardware ID and all inputs (no config file needed):**

   ```go
   id, err := sconfig.DebugHardwareID()
   // Legacy v1 identifiers, the v1 ID and the v2 sources go to stderr.
   ```

2. **Or** call `LoadConfig` with debug on:

   ```go
   err := sconfig.LoadConfig(&cfg, version, path, false, true) // 5th param = debugOutput
   ```

   Same debug lines go to stderr.

3. Run once when the key is "wrong" and save stderr. Compare with a run when
   the key was "right" (or on another machine). The `[sconfig DEBUG]` lines
   show the v2 sources (`name=value`) and, for legacy values, VM detection,
   the MAC and every v1 identifier; the differing line(s) are the cause.

4. **Hardware-ID track** (v1): When debug is on, each v1 computation and
   each decryption failure appends a line to `sconfig.debug.txt` in the
   executable’s directory. Format:
   `YYYY-MM-DD HH:MM:SS<TAB>hardwareID (hex)<TAB>identifiers`.
   Use this to see a timeline of IDs (e.g. after a failed decrypt).

## Security Notes

- **Machine-bound encryption**: Passwords are encrypted with AES-256-GCM
  using a key derived (HKDF-SHA256) from machine identity sources; encrypted
  config files are unusable on other systems. See
  [How the hardware ID is created](#how-the-hardware-id-is-created).
- **Threat model**: The encryption protects config files that leave the
  machine (backups, repositories, tickets). Every process on the machine can
  read the sources and therefore derive the key.
- **Legacy values**: v1 values have a small key space (about 2^31 / 2^32) and
  are migrated to v2 on load. Rotate passwords if old copies may have leaked.
- **Transparent decryption**: Passwords are automatically decrypted in memory
  for easy access
- **Use `cleanConfig` with care**: Setting `cleanConfig = true` writes
  plaintext passwords to the file; new files are created with permissions
  0600 and all files are written atomically.
- **Debug output** (`debugOutput`, `DebugHardwareID`) prints identifiers and
  key material; only use it for diagnosis.
- **Key file** against file disclosure bugs and for containers, see
  [Options](#options).
- **PHP:** `set()` refuses variable names with other characters than letters,
  digits, `_` and `.` and values with control characters (line breaks would
  inject further variables into the file). Decrypted passwords are exported
  to `$_ENV`/`putenv()` unless `EnvLoader::setExportPasswords(false)` is set.
  Keep the `.env` file outside the web document root (`setExecutableRoot()`).

## Donationware

This project is provided as donationware. If it helps you, please consider
donating to CFI Kinderhilfe to support children in need:

- Donate: [Donation page of CFI Kinderhilfe - in German!](https://cfi-kinderhilfe.de/jetzt-spenden/?q=VAYASCFG)

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
