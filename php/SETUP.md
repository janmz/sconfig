# Setup Instructions for PHP Development Tools

## Installing PHP Code Quality Tools

To enable code formatting and linting in Cursor/VS Code, you need to install the development dependencies:

```bash
cd php
composer install
```

This will install:
- **PHP_CodeSniffer** (phpcs/phpcbf) - For code style checking and fixing
- **PHP-CS-Fixer** - For automatic code formatting

## Cursor/VS Code Configuration

The workspace is already configured with `.vscode/settings.json` to use these tools. Make sure you have the following extensions installed in Cursor/VS Code:

1. **PHP CS Fixer** (junstyle.php-cs-fixer)
2. **PHP CodeSniffer** (shevaua.phpcs)

You can install them via:
- Open Command Palette (Ctrl+Shift+P / Cmd+Shift+P)
- Type "Extensions: Install Extensions"
- Search for "PHP CS Fixer" and "PHP CodeSniffer"

## Manual Usage

### Format code with PHP-CS-Fixer:
```bash
cd php
vendor/bin/php-cs-fixer fix src/
```

### Check code style with PHP_CodeSniffer:
```bash
cd php
vendor/bin/phpcs src/
```

### Auto-fix code style issues:
```bash
cd php
vendor/bin/phpcbf src/
```

## Troubleshooting

If Cursor still shows "PHPCBF was not found":

1. Make sure you've run `composer install` in the `php/` directory
2. Check that the tools are installed: `ls php/vendor/bin/phpcbf`
3. Restart Cursor/VS Code
4. Check the `.vscode/settings.json` file exists and points to the correct paths

