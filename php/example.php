<?php

/**
 * Example usage of sconfig-php with password encryption
 */

use Sconfig\EnvLoader;
use Sconfig\I18n;

require_once __DIR__ . '/vendor/autoload.php';


// Load .env file from current directory
// Note: Rename 'example_env' to '.env' before running
// Passwords will be automatically encrypted if DB_PASSWORD contains plaintext
try {
    EnvLoader::load('example_env'); // Change to '.env' after renaming the file
    echo "Environment loaded successfully!\n";
} catch (\RuntimeException $e) {
    echo "Error: " . $e->getMessage() . "\n";
    exit(1);
}

// Access values using the env() helper function (global function, no namespace needed)
$dbHost = env('DB_HOST', 'localhost');
$dbPort = env('DB_PORT', 3306);
$apiKey = env('API_KEY');

echo "DB Host: {$dbHost}\n";
echo "DB Port: {$dbPort}\n";
echo "API Key: " . ($apiKey ?: 'not set') . "\n";

// Access password - automatically decrypted in memory
// The .env file contains the encrypted version in DB_SECURE_PASSWORD
// but env('DB_PASSWORD') returns the decrypted value
$dbPassword = env('DB_PASSWORD');
if ($dbPassword) {
    echo "DB Password: " . str_repeat('*', strlen($dbPassword)) . " (decrypted)\n";
}

// Example: Using EnvLoader directly
if (EnvLoader::has('APP_ENV')) {
    $appEnv = EnvLoader::get('APP_ENV');
    echo "App Environment: {$appEnv}\n";
}

echo "\nNote: If DB_PASSWORD contained a plaintext password, it has been encrypted\n";
echo "and stored in DB_SECURE_PASSWORD. The .env file has been updated.\n";

// Example: Using i18n
echo "\n--- Internationalization Example ---\n";
echo "Current language: " . I18n::getCurrentLanguage() . "\n";
echo "Password marker: " . I18n::t('config.password_message') . "\n";

// Switch to German
I18n::setLanguage('de');
echo "German password marker: " . I18n::t('config.password_message') . "\n";

// Switch back to English
I18n::setLanguage('en');
echo "English password marker: " . I18n::t('config.password_message') . "\n";

