# sconfig (PHP)

PHP library for loading and managing `.env` configuration files with secure
password handling. This is the PHP variant of the
[sconfig](https://github.com/janmz/sconfig) Go library.

## Installation

### Via Packagist (when published)

```bash
composer require janmz/sconfig
```

### Via Git Repository (development/current)

If the package is not yet published on Packagist, you can install it directly
from the Git repository:

```bash
composer require janmz/sconfig:dev-main --prefer-source
```

Or add to your `composer.json`:

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

**Note:** When installing from Git, you need to specify the path to the PHP
package. The package is located in the `php/` subdirectory of the repository.

## Upgrading from v2 to v3

Require `"janmz/sconfig": "^3.0"` in `composer.json`; `^2.0` keeps v2. New
encrypted values use the v2 key format and cannot be read by older versions;
old values are migrated automatically on the first `load()`. `set()` now
throws `InvalidArgumentException` for invalid names and values with line
breaks, and `updateEnv()` no longer writes values of the process
environment.

## Usage

### Basic Usage

```php
<?php

require_once 'vendor/autoload.php';

use Sconfig\EnvLoader;

// Load .env file
EnvLoader::load('.env');

// Access values using the env() helper function
$dbHost = env('DB_HOST');
$dbPort = env('DB_PORT', 3306); // with default value
$apiKey = env('API_KEY');
```

### Secure Password Handling

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
making them unusable on other systems. The Go version on the same machine can
read them (v2 format).

### Loading from Custom Path

```php
use Sconfig\EnvLoader;

// Load from a custom path
EnvLoader::load('/path/to/your/.env');
```

### Override Existing Variables

By default, existing environment variables are not overridden. To override them:

```php
use Sconfig\EnvLoader;

EnvLoader::load('.env', true); // override = true
```

### Writing back env changes (updateEnv)

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

### Clean Config Mode

To decrypt passwords before writing (use with care, primarily for migration or inspection):

```php
use Sconfig\EnvLoader;

EnvLoader::load('.env', false, true); // cleanConfig = true
```

This will write plaintext passwords back to the `.env` file.

### Direct Class Usage

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

## .env File Format

The `.env` file should follow this format. An example file `example_env` is
included in this package:

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

**Note:** Copy `example_env` to `.env` in your project directory and customize it.

- Lines starting with `#` are treated as comments
- Empty lines are ignored
- Values can be quoted with single or double quotes (quotes are automatically removed)
- Spaces around `=` are automatically trimmed
- **Password pairs**: Fields named `<NAME>_PASSWORD` and
  `<NAME>_SECURE_PASSWORD` are automatically encrypted

## Features

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

## Testing

Run the test suite with PHPUnit (after `composer install`):

```bash
cd php
./vendor/bin/phpunit
```

Tests cover loading/parsing, override/cleanConfig, password encryption (marker
on disk, plaintext in memory), and the `env()` helper.

## API Reference

### EnvLoader::load()

```php
EnvLoader::load(string $filePath, bool $override = false, bool $cleanConfig = false)
```

Loads environment variables from a `.env` file and processes password encryption.

**Parameters:**

- `$filePath` - Path to the `.env` file
- `$override` - Whether to override existing environment variables (default: false)
- `$cleanConfig` - If true, decrypts passwords before writing (default: false,
  use with care)

**Throws:** `RuntimeException` if the file cannot be read or encryption fails

### env(string $key, mixed $default = null)

Global helper function to get an environment variable value.

**Parameters:**

- `$key` - The environment variable key
- `$default` - Default value if key is not found

**Returns:** The environment variable value or default

### EnvLoader::get(string $key, mixed $default = null)

Get an environment variable value.

### EnvLoader::has(string $key): bool

Check if an environment variable exists.

### EnvLoader::clear(): void

Clear the cache and reset loaded state.

### EnvLoader::isLoaded(): bool

Check if environment has been loaded.

## Internationalization

The library supports multiple languages and automatically detects the system
language from environment variables (`LC_ALL` before `LC_MESSAGES` before
`LANG`). The built-in translations are shipped in `locales/` of this package
(copies of the Go `locales/` files).

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

## How the hardware ID is created

Every encrypted value starts with its format:

- `v2:<sources>:<base64>` – current format, written by Go and PHP since
  version 3.0.0.
- plain base64 without prefix – legacy format v1 (before 3.0.0). It is still
  readable and is rewritten as v2 automatically by the next `LoadConfig` or
  `EnvLoader::load()`.

### Current format v2 (identical to the Go version)

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

The PHP v1 derivation:

#### Step 1 – VM detection

- **Windows:** `wmic computersystem get Manufacturer,Model /value`. VM if the
  manufacturer or model (lower case) contains `vmware`, `virtualbox`,
  `microsoft corporation`, `xen`, `parallels`, `qemu`, `kvm`, `innotek` or
  `bochs`, or if the model contains `virtual`. Physical Microsoft devices
  (e.g. Surface) are therefore treated as VMs. Without `wmic`: no VM.
- **Linux:** VM if `systemd-detect-virt` prints anything other than `none`,
  or if `/sys/class/dmi/id/sys_vendor`, `product_name` or `chassis_vendor`
  contains `qemu`, `kvm`, `vmware`, `virtualbox`, `xen`, `parallels`,
  `microsoft`, `bochs` or `bhyve`.
- **All other systems** (macOS, BSD, …): never a VM.

#### Step 2 – Identifiers (in collection order, not sorted)

- **Windows:**
  1. VM only: `MachineGuid` from
     `reg query HKLM\SOFTWARE\Microsoft\Cryptography /v MachineGuid`
     (regex `MachineGuid\s+REG_SZ\s+(.+)`) – an OS installation ID.
  2. VM only: SMBIOS UUID from `wmic csproduct get UUID /value`, unchanged.
  3. MAC: first column of the first line of `getmac /fo csv /nh`, unchanged
     (upper case with `-`). This is the first adapter listed by Windows,
     **regardless of whether it is connected**.
  4. First non-empty value of `wmic baseboard get SerialNumber /value`.
  5. First non-empty value of `wmic baseboard get Product /value`
     (mainboard model name, not unique per machine).
  6. First non-empty value of `wmic diskdrive get SerialNumber /value`
     (first drive in `wmic` order).
  7. Physical machines only: first value of `wmic cpu get ProcessorId /value`
     (CPUID signature, identical for all CPUs of the same model).
- **All other systems** (Linux, macOS, BSD, …):
  1. Linux VM only: `/etc/machine-id` (OS installation ID) and
     `/sys/class/dmi/id/product_uuid`.
  2. MAC: `cat /sys/class/net/*/address | head -1` – the first interface in
     alphabetical order (may be `docker0`, a bridge or `lo` with
     `00:00:00:00:00:00`). This path does not exist on macOS.
  3. `cat /sys/class/dmi/id/board_serial` (usually readable by root only).
  4. Physical machines only: the complete first line of `/proc/cpuinfo`
     containing `Serial`, including the label (e.g. `Serial : 0000…`).
- Values that `empty()` treats as empty (including `"0"`) are skipped.
- **Fallback:** if no identifier was found, `gethostname()` and `PHP_VERSION`
  are used – **no hardware at all**. Renaming the host or updating PHP
  changes the key (typical on macOS).

#### Step 3 – Hash

The identifiers are joined with `|` in collection order and hashed with
SHA-256. The first 8 bytes, read as **big-endian**, form a signed 64-bit
integer (the hardware ID).

#### Step 4 – Key

`mt_srand(<hardware ID>)` – PHP's Mersenne Twister uses only the **lower
32 bits** of the seed. 32 times `(mt_rand() ^ mt_rand()) & 0xff` yield the
AES-256-GCM key (random IV per encryption).

#### Weaknesses of v1

- **Not compatible with Go:** the Go v1 derivation uses other identifiers,
  sorting, MAC format, byte order, key expansion and ciphertext layout. v1
  values of PHP cannot be decrypted by Go and vice versa (v2 values can).
- **MAC only:** current Windows 11 versions no longer install `wmic` by
  default; a physical Windows PC then uses only the MAC address of the first
  `getmac` adapter. A MAC is visible on the local network segment, so the
  protection against someone who knows it is low.
- **Small key space:** only 2^32 different keys exist (Go: about 2^31).
  Someone who has only the encrypted file can try all of them in a few
  hours, without knowing the hardware.
- **Key changes** when any identifier changes: adapter order (`getmac`,
  `/sys/class/net`), `wmic` installed/removed, hardware replaced, OS
  reinstalled or cloned (VMs), different process rights (Linux root vs.
  user), or – in the fallback – host renamed or PHP updated.

## Security Notes

- **Machine-bound encryption**: Passwords are encrypted with AES-256-GCM
  using a key derived (HKDF-SHA256) from machine identity sources; see
  [How the hardware ID is created](#how-the-hardware-id-is-created).
- **Threat model**: The encryption protects .env files that leave the
  machine (backups, repositories, tickets). Every process on the machine can
  read the sources and therefore derive the key.
- **Legacy values**: v1 values have only 2^32 keys and are migrated to v2 on
  load. Rotate passwords if old copies may have leaked.
- **Transparent decryption**: Passwords are automatically decrypted in memory
  for easy access via `env()`
- **Use `cleanConfig` with care**: Setting `cleanConfig = true` writes
  plaintext passwords to the file; new files are created with permissions
  0600 and all files are written atomically.
- **`updateEnv()`** writes only values set with `set()` and values loaded
  from the target file (for a backup path: from the last loaded file), never
  values of the process environment.
- **`set()`** refuses variable names with other characters than letters,
  digits, `_` and `.` and values with control characters (line breaks would
  inject further variables into the file); it throws
  `InvalidArgumentException`.
- **Exported passwords:** decrypted passwords are written to
  `$_ENV`/`putenv()` (inherited by child processes, visible in `phpinfo()`)
  unless `EnvLoader::setExportPasswords(false)` is set.
- **Web root:** keep the `.env` file outside the document root and call
  `EnvLoader::setExecutableRoot()`; a warning is issued otherwise.
- **Windows permissions:** PHP cannot set ACLs. A replaced or new `.env`
  file gets the permissions inherited from its directory; keep it in a
  directory that only the application user can read.

## License

[MIT](../LICENSE)

## Author

Jan Neuhaus, VAYA Consulting - <https://vaya-consulting.de/>

## Related

- [sconfig (Go version)](https://github.com/janmz/sconfig)
