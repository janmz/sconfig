# Installation Instructions

## Option 1: Via Packagist (Recommended)

Once the package is published on [Packagist](https://packagist.org/):

```bash
composer require janmz/sconfig
```

## Option 2: Via Git Repository

### Direct Installation

```bash
composer require janmz/sconfig:dev-main --prefer-source
```

### Manual Installation via composer.json

Add to your `composer.json`:

```json
{
    "repositories": [
        {
            "type": "path",
            "url": "../sconfig/php"
        }
    ],
    "require": {
        "janmz/sconfig": "*"
    }
}
```

Or if using the Git repository directly:

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

## Option 3: Local Development

For local development, you can use a path repository:

```json
{
    "repositories": [
        {
            "type": "path",
            "url": "/path/to/sconfig/php"
        }
    ],
    "require": {
        "janmz/sconfig": "*"
    }
}
```

## Publishing to Packagist

To make the package available via `composer require janmz/sconfig`:

### Option A: Using Root composer.json (Recommended)

This repository includes a `composer.json` in the root directory that points to the PHP source files in the `php/` subdirectory. This is the standard approach for Packagist.

1. Create a Git tag (e.g., `v1.0.0-php` or `php-v1.0.0`)
2. Submit the package to [Packagist](https://packagist.org/packages/submit)
   - Repository URL: `https://github.com/janmz/sconfig`
   - Packagist will automatically detect the `composer.json` in the root

### Option B: Separate Repository

If you prefer to keep the PHP package completely separate:

1. Create a new repository (e.g., `janmz/sconfig-php` - note: this is no longer recommended, use the unified `janmz/sconfig` package instead)
2. Copy the contents of the `php/` directory to the new repository root
3. Submit to Packagist with the new repository URL

### Option C: Monorepo with Subdirectory

Some tools support monorepos with subdirectories, but Packagist itself doesn't natively support this. You would need to:
- Use a tool like [Monorepo Builder](https://github.com/symplify/monorepo-builder) to split packages, or
- Create a separate repository for the PHP package

**Current Setup:** This repository includes a `composer.json` in the root that references the PHP files in `php/src/`, making it ready for Packagist submission.

