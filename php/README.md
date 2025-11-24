# sconfig (PHP)

PHP library for loading and managing `.env` configuration files with secure password handling. This is the PHP variant of the [sconfig](https://github.com/janmz/sconfig) Go library.

## Installation

### Via Packagist (when published)

```bash
composer require janmz/sconfig
```

### Via Git Repository (development/current)

If the package is not yet published on Packagist, you can install it directly from the Git repository:

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

**Note:** When installing from Git, you need to specify the path to the PHP package. The package is located in the `php/` subdirectory of the repository.

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

The library automatically handles password encryption for fields named `<NAME>_PASSWORD` and `<NAME>_SECURE_PASSWORD`:

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

The encrypted passwords are machine-bound (derived from hardware identifiers), making them unusable on other systems.

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

The `.env` file should follow this format. An example file `example_env` is included in this package:

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
- **Password pairs**: Fields named `<NAME>_PASSWORD` and `<NAME>_SECURE_PASSWORD` are automatically encrypted

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
- Translations loaded from `locales/` directory (same as Go version)

## API Reference

### EnvLoader::load(string $filePath, bool $override = false, bool $cleanConfig = false)

Loads environment variables from a `.env` file and processes password encryption.

**Parameters:**
- `$filePath` - Path to the `.env` file
- `$override` - Whether to override existing environment variables (default: false)
- `$cleanConfig` - If true, decrypts passwords before writing (default: false, use with care)

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

The library supports multiple languages and automatically detects the system language from environment variables (`LANG`, `LC_ALL`, `LC_MESSAGES`). Translations are loaded from the `locales/` directory (same JSON files as the Go version).

Supported languages:
- English (en) - default/fallback
- German (de)

You can manually set the language:

```php
use Sconfig\I18n;

I18n::setLanguage('de'); // Switch to German
$message = I18n::t('config.password_message'); // Get translated message
```

The library automatically finds the `locales/` directory by searching:
1. Parent directory of the PHP package (`../locales`)
2. Project root (`./locales`)
3. Current working directory (`./locales`)

## Security Notes

- **Machine-bound encryption**: Passwords are encrypted using keys derived from hardware identifiers, making encrypted config files unusable on other systems
- **Transparent decryption**: Passwords are automatically decrypted in memory for easy access via `env()`
- **Use `cleanConfig` with care**: Setting `cleanConfig = true` writes plaintext passwords to the file
- **Hardware ID**: The encryption key is generated from system hardware identifiers (MAC address, CPU ID, etc.)

## License

[MIT](../LICENSE)

## Author

Jan Neuhaus, VAYA Consulting - https://vaya-consulting.de/

## Related

- [sconfig (Go version)](https://github.com/janmz/sconfig)

