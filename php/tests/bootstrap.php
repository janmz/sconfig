<?php

/**
 * PHPUnit bootstrap for sconfig PHP tests.
 * Sets up autoload and locales path so I18n finds translations.
 */

$autoload = dirname(__DIR__) . '/vendor/autoload.php';
if (! is_file($autoload)) {
    throw new RuntimeException('Run "composer install" in the php directory first.');
}
require $autoload;

// Test files are created with file_put_contents(); with the usual Unix umask
// 022 they get mode 0644 and EnvFile emits a "readable by other users"
// warning, which breaks tests asserting the exact warning list. Restrict new
// files to the owner (tests that need 0644 chmod explicitly). No-op on Windows.
umask(0077);

// Ensure locales are found when running from repo root (sconfig) or from php/
$localesCandidates = [
    dirname(__DIR__) . '/locales',
    dirname(dirname(__DIR__)) . '/locales',
];
foreach ($localesCandidates as $path) {
    if (is_dir($path) && is_file($path . '/en.json')) {
        \Sconfig\I18n::initialize($path);

        break;
    }
}
