<?php

namespace Sconfig;

/**
 * Reading, rendering and atomic writing of .env files.
 *
 * Unknown lines, empty lines, comments, "export" prefixes and the quote style
 * of existing values are preserved when a file is written back.
 *
 * @package Sconfig
 * @internal
 */
final class EnvFile
{
    /** Permissions of newly created files (they may contain secrets). */
    private const NEW_FILE_MODE = 0600;

    /** Maximum size of a .env file that is read. */
    public const MAX_FILE_SIZE = 1048576;

    /** Maximum length of a value written with set(). */
    public const MAX_VALUE_LENGTH = 65536;

    /** Maximum length of a variable name. */
    private const MAX_KEY_LENGTH = 256;

    /**
     * Check a variable name for set()/new keys (letters, digits, "_" and ".",
     * not starting with a digit).
     *
     * @throws \InvalidArgumentException
     */
    public static function validateKey(string $key): void
    {
        if (strlen($key) > self::MAX_KEY_LENGTH || preg_match('/^[A-Za-z_][A-Za-z0-9_.]*$/', $key) !== 1) {
            throw new \InvalidArgumentException(I18n::t('config.invalid_key', $key));
        }
    }

    /**
     * Check a value: no control characters except tab, limited length. A line
     * break would otherwise inject further variables into the file.
     *
     * @throws \InvalidArgumentException
     */
    public static function validateValue(string $key, string $value): void
    {
        if (strlen($value) > self::MAX_VALUE_LENGTH || preg_match('/[\x00-\x08\x0A-\x1F\x7F]/', $value) === 1) {
            throw new \InvalidArgumentException(I18n::t('config.invalid_value', $key, self::MAX_VALUE_LENGTH));
        }
    }

    /**
     * Read all lines of a file (without line endings, empty lines kept).
     *
     * @return array<int, string>
     * @throws \RuntimeException If the file cannot be read
     */
    public static function readLines(string $path): array
    {
        $content = @file_get_contents($path, false, null, 0, self::MAX_FILE_SIZE + 1);
        if ($content === false) {
            throw new \RuntimeException(I18n::t('config.read_failed', $path));
        }
        if (strlen($content) > self::MAX_FILE_SIZE) {
            throw new \RuntimeException(I18n::t('config.file_too_large', $path, self::MAX_FILE_SIZE));
        }
        if ($content === '') {
            return [];
        }
        $lines = preg_split('/\r\n|\n|\r/', $content);
        if ($lines === false) {
            return [];
        }
        if (end($lines) === '') {
            array_pop($lines); // trailing newline
        }

        return $lines;
    }

    /**
     * Parse lines into key/value pairs and line entries.
     *
     * @param array<int, string> $lines
     * @return array{0: array<string, string>, 1: array<int, array<string, mixed>>}
     */
    public static function parse(array $lines): array
    {
        $parsed = [];
        $entries = [];
        foreach ($lines as $line) {
            $trimmed = trim($line);
            if ($trimmed === '' || $trimmed[0] === '#' || strpos($line, '=') === false) {
                $entries[] = ['type' => 'raw', 'content' => $line];

                continue;
            }
            [$key, $value] = explode('=', $line, 2);
            $key = trim($key);
            $export = false;
            if (preg_match('/^export\s+(.+)$/', $key, $m) === 1) {
                $export = true;
                $key = trim($m[1]);
            }
            $value = trim($value);
            $quote = '';
            $first = substr($value, 0, 1);
            if (strlen($value) >= 2 && ($first === '"' || $first === "'") && substr($value, -1) === $first) {
                $quote = $first;
                $value = substr($value, 1, -1);
            }
            $parsed[$key] = $value;
            $entries[] = ['type' => 'var', 'key' => $key, 'export' => $export, 'quote' => $quote, 'content' => $line];
        }

        return [$parsed, $entries];
    }

    /**
     * Render the entries with the values from $parsed; keys that are not in
     * the entries are appended. Values with line breaks or NUL and invalid new
     * keys are refused, so that no line can be injected.
     *
     * @throws \InvalidArgumentException
     * @param array<int, array<string, mixed>> $entries
     * @param array<string, string> $parsed
     */
    public static function render(array $entries, array $parsed): string
    {
        $output = [];
        $written = [];
        foreach ($entries as $entry) {
            if ($entry['type'] === 'raw') {
                $output[] = (string) $entry['content'];

                continue;
            }
            $key = (string) $entry['key'];
            if (! array_key_exists($key, $parsed)) {
                $output[] = (string) $entry['content']; // unknown key: keep the line

                continue;
            }
            self::checkLineSafe($key, (string) $parsed[$key]);
            $prefix = $entry['export'] ? 'export ' : '';
            $output[] = $prefix . $key . '=' . self::formatValue($parsed[$key], (string) $entry['quote']);
            $written[$key] = true;
        }
        foreach ($parsed as $key => $value) {
            if (! isset($written[$key])) {
                self::validateKey((string) $key);
                self::checkLineSafe((string) $key, (string) $value);
                $output[] = $key . '=' . self::formatValue((string) $value, '');
            }
        }

        return $output === [] ? '' : implode("\n", $output) . "\n";
    }

    /**
     * @throws \InvalidArgumentException If the value would break the line structure
     */
    private static function checkLineSafe(string $key, string $value): void
    {
        if (strpbrk($value, "\r\n\0") !== false) {
            throw new \InvalidArgumentException(I18n::t('config.invalid_value', $key, self::MAX_VALUE_LENGTH));
        }
    }

    /**
     * Quote a value if the original was quoted or if it needs quotes
     * (whitespace, "#", quotes, backslash) to be read back unchanged.
     */
    public static function formatValue(string $value, string $quote): string
    {
        if ($quote === '' && preg_match('/[\s#"\'\\\\]/', $value) === 1) {
            $quote = '"';
        }
        if ($quote === '"' && strpos($value, '"') !== false && strpos($value, "'") === false) {
            $quote = "'";
        }

        return $quote . $value . $quote;
    }

    /**
     * Write content to a temporary file in the same directory and rename it
     * over $path. Existing permissions (and, if allowed, owner and group) are
     * kept, new files get 0600. A warning is issued if an existing file is
     * readable by other users. $path should be the resolved path (realpath),
     * so that a symbolic link is kept and its target is replaced.
     *
     * @throws \RuntimeException If the file cannot be written
     */
    public static function writeAtomic(string $path, string $content): void
    {
        $dir = dirname($path);
        $tmp = @tempnam($dir, '.sconfig');
        if ($tmp === false || realpath(dirname($tmp)) !== realpath($dir)) {
            if (is_string($tmp)) {
                @unlink($tmp);
            }

            throw new \RuntimeException(I18n::t('config.failed_writing', $path, 'cannot create temporary file'));
        }
        $exists = file_exists($path);
        $mode = $exists ? (fileperms($path) & 0777) : self::NEW_FILE_MODE;
        if (@file_put_contents($tmp, $content) !== strlen($content)) {
            @unlink($tmp);

            throw new \RuntimeException(I18n::t('config.failed_writing', $path, 'write failed'));
        }
        @chmod($tmp, $mode);
        if ($exists && PHP_OS_FAMILY !== 'Windows') {
            $owner = @fileowner($path);
            $group = @filegroup($path);
            if (is_int($owner) && is_int($group)) {
                @chown($tmp, $owner); // needs privileges; otherwise the current user
                @chgrp($tmp, $group);
            }
            if (($mode & 0077) !== 0) {
                Warnings::emit(I18n::t('config.warn_permissions', $path, sprintf('%04o', $mode)));
            }
        }
        if (! @rename($tmp, $path)) {
            @unlink($tmp);

            throw new \RuntimeException(I18n::t('config.failed_writing', $path, 'rename failed'));
        }
    }
}
