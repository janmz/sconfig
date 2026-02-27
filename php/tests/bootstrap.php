<?php

/**
 * PHPUnit bootstrap for sconfig PHP tests.
 * Sets up autoload and locales path so I18n finds translations.
 */

$autoload = dirname(__DIR__) . '/vendor/autoload.php';
if (!is_file($autoload)) {
    throw new RuntimeException('Run "composer install" in the php directory first.');
}
require $autoload;

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
