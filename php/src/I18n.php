<?php

namespace Sconfig;

/**
 * I18n - Internationalization support for sconfig (PHP)
 *
 * The built-in translations are shipped in php/locales (copies of the Go
 * locales). Additional or overriding translations are only loaded from a
 * directory that the application passes explicitly to initialize(); the
 * password marker is always taken from the built-in translations.
 *
 * @package Sconfig
 */
class I18n
{
    /** Maximum size of a translation file. */
    private const MAX_FILE_SIZE = 1048576;

    /**
     * @var array<string, array<string, string>> Built-in translations by language
     */
    private static array $builtin = [];

    /**
     * @var array<string, array<string, string>> Built-in plus external translations
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
     * Initialize the i18n system.
     *
     * @param string|null $localesPath Optional directory with additional or
     *        overriding translation files (<lang>.json)
     * @return void
     */
    public static function initialize(?string $localesPath = null): void
    {
        if (self::$initialized) {
            return;
        }
        self::$currentLanguage = self::detectLanguage();
        self::$builtin = self::readDirectory(self::findBuiltinDirectory());
        self::$translations = self::$builtin;
        if ($localesPath !== null && realpath($localesPath) !== realpath(self::findBuiltinDirectory())) {
            foreach (self::readDirectory($localesPath) as $lang => $messages) {
                self::$translations[$lang] = array_merge(self::$translations[$lang] ?? [], $messages);
            }
        }
        self::$initialized = true;
    }

    /**
     * Directory of the built-in translations: php/locales (Composer package)
     * or, in the repository, the shared locales directory.
     */
    private static function findBuiltinDirectory(): string
    {
        foreach ([__DIR__ . '/../locales', __DIR__ . '/../../locales'] as $path) {
            if (is_file($path . '/en.json')) {
                return $path;
            }
        }

        return __DIR__ . '/../locales';
    }

    /**
     * Read all <lang>.json files of a directory; invalid files are skipped.
     *
     * @return array<string, array<string, string>>
     */
    private static function readDirectory(string $dir): array
    {
        $result = [];
        $files = is_dir($dir) ? glob($dir . '/*.json') : false;
        if ($files === false) {
            return $result;
        }
        foreach ($files as $file) {
            $data = @file_get_contents($file, false, null, 0, self::MAX_FILE_SIZE + 1);
            if ($data !== false && strlen($data) > self::MAX_FILE_SIZE) {
                continue; // too large: ignored
            }
            $messages = $data === false ? null : json_decode($data, true);
            if (! is_array($messages)) {
                continue;
            }
            $result[basename($file, '.json')] = array_filter($messages, 'is_string');
        }

        return $result;
    }

    /**
     * Detect the language from the environment (POSIX precedence
     * LC_ALL > LC_MESSAGES > LANG, same as the Go version).
     *
     * @return string Language code ('en' or 'de')
     */
    private static function detectLanguage(): string
    {
        foreach (['LC_ALL', 'LC_MESSAGES', 'LANG'] as $envVar) {
            $lang = getenv($envVar);
            if (($lang === false || $lang === '') && isset($_ENV[$envVar])) {
                $lang = (string) $_ENV[$envVar];
            }
            if ($lang !== false && $lang !== '') {
                return strpos(strtolower($lang), 'de') === 0 ? 'de' : 'en';
            }
        }

        return 'en';
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
     * Built-in text of a key (external translations are ignored). Used for
     * the password marker, which must not be changed by external files.
     *
     * @param string|null $lang Language, null = current language
     */
    public static function builtinText(string $key, ?string $lang = null): string
    {
        self::initialize();
        $lang = $lang ?? self::$currentLanguage;

        return self::$builtin[$lang][$key] ?? self::$builtin['en'][$key] ?? $key;
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
        self::initialize();
        $translation = self::$translations[self::$currentLanguage][$key]
            ?? self::$translations['en'][$key]
            ?? $key;
        if ($args === []) {
            return $translation;
        }
        // Go-style %v becomes %s
        $format = str_replace('%v', '%s', $translation);

        try {
            $result = @sprintf($format, ...$args);
        } catch (\Throwable $e) {
            $result = false; // PHP 8: ArgumentCountError / ValueError
        }
        if (! is_string($result)) {
            return $translation . ': ' . implode(', ', array_map('strval', $args));
        }

        return $result;
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
