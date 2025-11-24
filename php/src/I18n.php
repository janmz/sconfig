<?php

namespace Sconfig;

/**
 * I18n - Internationalization support for sconfig (PHP)
 *
 * Loads translations from the locales/ directory and provides
 * translation functionality similar to the Go version.
 *
 * @package Sconfig
 */
class I18n
{
    /**
     * @var array<string, array<string, string>> Loaded translations by language
     */
    private static array $translations = [];

    /**
     * @var string Current language code (e.g., 'en', 'de')
     */
    private static string $currentLanguage = 'en';

    /**
     * @var bool Whether translations have been loaded
     */
    private static bool $initialized = false;

    /**
     * @var string|null Path to locales directory
     */
    private static ?string $localesPath = null;

    /**
     * Initialize the i18n system
     *
     * @param string|null $localesPath Optional path to locales directory
     * @return void
     */
    public static function initialize(?string $localesPath = null): void
    {
        if (self::$initialized) {
            return;
        }

        // Try to find locales directory
        if ($localesPath === null) {
            $localesPath = self::findLocalesDirectory();
        }

        self::$localesPath = $localesPath;

        // Detect system language
        self::$currentLanguage = self::detectLanguage();

        // Load translations
        self::loadTranslations();

        self::$initialized = true;
    }

    /**
     * Find the locales directory
     *
     * Tries to find the locales directory relative to the package or project root.
     *
     * @return string Path to locales directory
     */
    private static function findLocalesDirectory(): string
    {
        // Try several possible locations
        $possiblePaths = [
            // From PHP package directory: ../locales (parent directory)
            __DIR__ . '/../../locales',
            // From project root: ./locales
            __DIR__ . '/../../../locales',
            // From vendor directory: ../../../../locales
            __DIR__ . '/../../../../locales',
            // Current directory
            getcwd() . '/locales',
            // Relative to composer vendor directory
            dirname(dirname(dirname(__DIR__))) . '/locales',
        ];

        foreach ($possiblePaths as $path) {
            if (is_dir($path) && file_exists($path . '/en.json')) {
                return $path;
            }
        }

        // Fallback: return a default path (will fail gracefully if not found)
        return __DIR__ . '/../../locales';
    }

    /**
     * Detect system language from environment variables
     *
     * @return string Language code ('en' or 'de')
     */
    private static function detectLanguage(): string
    {
        // Try environment variables (same as Go version)
        $envVars = ['LANG', 'LC_ALL', 'LC_MESSAGES'];
        
        foreach ($envVars as $envVar) {
            $lang = getenv($envVar);
            if ($lang !== false && $lang !== '') {
                $lang = strtolower($lang);
                if (strpos($lang, 'de') === 0) {
                    return 'de';
                }
                if (strpos($lang, 'en') === 0) {
                    return 'en';
                }
            }
        }

        // Try $_ENV as fallback
        foreach ($envVars as $envVar) {
            if (isset($_ENV[$envVar])) {
                $lang = strtolower($_ENV[$envVar]);
                if (strpos($lang, 'de') === 0) {
                    return 'de';
                }
                if (strpos($lang, 'en') === 0) {
                    return 'en';
                }
            }
        }

        // Default to English
        return 'en';
    }

    /**
     * Load translations from JSON files
     *
     * @return void
     */
    private static function loadTranslations(): void
    {
        if (self::$localesPath === null || !is_dir(self::$localesPath)) {
            // If locales directory not found, use empty translations
            self::$translations['en'] = [];
            self::$translations['de'] = [];
            return;
        }

        // Load English translations (fallback)
        $enFile = self::$localesPath . '/en.json';
        if (file_exists($enFile)) {
            $data = file_get_contents($enFile);
            if ($data !== false) {
                $translations = json_decode($data, true);
                if (is_array($translations)) {
                    self::$translations['en'] = $translations;
                }
            }
        }

        // Load German translations
        $deFile = self::$localesPath . '/de.json';
        if (file_exists($deFile)) {
            $data = file_get_contents($deFile);
            if ($data !== false) {
                $translations = json_decode($data, true);
                if (is_array($translations)) {
                    self::$translations['de'] = $translations;
                }
            }
        }

        // Try to load external translation files (override embedded ones)
        self::loadExternalTranslations();
    }

    /**
     * Load external translation files from locales directory
     *
     * @return void
     */
    private static function loadExternalTranslations(): void
    {
        if (self::$localesPath === null || !is_dir(self::$localesPath)) {
            return;
        }

        $files = glob(self::$localesPath . '/*.json');
        if ($files === false) {
            return;
        }

        foreach ($files as $file) {
            $lang = basename($file, '.json');
            if (!isset(self::$translations[$lang])) {
                self::$translations[$lang] = [];
            }

            $data = file_get_contents($file);
            if ($data !== false) {
                $translations = json_decode($data, true);
                if (is_array($translations)) {
                    // Merge with existing translations (external overrides embedded)
                    self::$translations[$lang] = array_merge(
                        self::$translations[$lang] ?? [],
                        $translations
                    );
                }
            }
        }
    }

    /**
     * Set the current language
     *
     * @param string $lang Language code ('en' or 'de')
     * @return void
     */
    public static function setLanguage(string $lang): void
    {
        self::$currentLanguage = $lang;
    }

    /**
     * Get the current language
     *
     * @return string Current language code
     */
    public static function getCurrentLanguage(): string
    {
        return self::$currentLanguage;
    }

    /**
     * Translate a key to the current language
     *
     * @param string $key Translation key
     * @param mixed ...$args Arguments for sprintf formatting
     * @return string Translated string
     */
    public static function translate(string $key, ...$args): string
    {
        if (!self::$initialized) {
            self::initialize();
        }

        // Get translation for current language
        $translation = self::$translations[self::$currentLanguage][$key] ?? null;

        // Fallback to English if not found
        if ($translation === null) {
            $translation = self::$translations['en'][$key] ?? $key;
        }

        // Format with arguments if provided
        if (!empty($args)) {
            // Handle Go-style %v formatting (convert to %s for PHP)
            $translation = str_replace('%v', '%s', $translation);
            return sprintf($translation, ...$args);
        }

        return $translation;
    }

    /**
     * Helper function for easy access (similar to Go's t() function)
     *
     * @param string $key Translation key
     * @param mixed ...$args Arguments for sprintf formatting
     * @return string Translated string
     */
    public static function t(string $key, ...$args): string
    {
        return self::translate($key, ...$args);
    }
}

