<?php

namespace Sconfig;

/**
 * Description: Loads and manages environment variables from .env files with
 * secure password handling.
 *
 * Design goal (as in the Go implementation): the key is intentionally not
 * random per call but always the same on the same machine, so that .env data
 * stays readable. The aim is a secret that only this machine can reproduce,
 * not a fresh random secret on every start.
 *
 * This class loads environment variables from a .env file, automatically
 * encrypts passwords and makes them accessible via the env() helper function.
 *
 * Password handling:
 * - Fields named `<NAME>_PASSWORD` and `<NAME>_SECURE_PASSWORD` are treated as a pair
 * - If `<NAME>_PASSWORD` contains a plaintext password (not the marker), it is encrypted
 * - The encrypted value (v2 format, see Crypto) is stored in `<NAME>_SECURE_PASSWORD`
 * - `<NAME>_PASSWORD` is replaced with a marker string
 * - Legacy values (before v2) are decrypted and re-encrypted with v2 on load()
 * - In memory, passwords are automatically decrypted for transparent access
 *
 * @package Sconfig
 * @author Jan Neuhaus, VAYA Consulting
 * @version 3.0.0
 *
 * Changelog:
 * 3.0.0 27.10.26 v2 key format shared with Go, migration of legacy values, format-preserving
 *                atomic writes, updateEnv() writes only file and set() keys
 * 1.2.0 27.02.26 set(), updateEnv(): persist changes (e.g. theme); require load() first; may write to different path
 * 1.1.0 24.11.25 Initial build of the PHP variant of the sconfig library
 */
class EnvLoader
{
    /**
     * Optional override for the directory under which .env paths must lie
     * (tests and applications that set an explicit base).
     */
    private static ?string $executableRootOverride = null;

    /**
     * Optional override for the user's home directory (tests only).
     */
    private static ?string $homeDirOverride = null;

    /**
     * @var array<string, string> Values loaded from .env files or set via set()
     */
    private static array $cache = [];

    /**
     * @var bool Whether the environment has been loaded
     */
    private static bool $loaded = false;

    /**
     * @var string|null Path to the loaded .env file
     */
    private static ?string $filePath = null;

    /**
     * @var array<string, string> Origin of each cached key: resolved path of
     *      the .env file it was loaded from, or "set" for set()
     */
    private static array $origin = [];

    /**
     * @var bool Whether decrypted passwords are exported to / putenv()
     */
    private static bool $exportPasswords = true;

    private const ORIGIN_SET = 'set';

    /**
     * Set the directory that contains the application entry script (or the
     * executable directory). Paths passed to load() / updateEnv() must resolve
     * under this directory, under the current working directory or under the
     * user's home directory. Call from application bootstrap if the default
     * (directory of SCRIPT_FILENAME) is not appropriate.
     */
    public static function setExecutableRoot(string $directory): void
    {
        self::$executableRootOverride = $directory;
    }

    /**
     * @internal For tests only (PHPUnit: point at the temp directory).
     */
    public static function setExecutableRootForTest(?string $directory): void
    {
        self::$executableRootOverride = $directory;
    }

    /**
     * @internal For tests only: pin the home directory (null = real home).
     */
    public static function setHomeDirForTest(?string $directory): void
    {
        self::$homeDirOverride = $directory;
    }

    /**
     * @internal For tests only: replace the machine identity by a fixed number
     * (legacy hardware ID and v2 source "override"); null = real machine.
     */
    public static function setHardwareIdForTest(?int $hardwareId): void
    {
        Crypto::setHardwareIdOverride($hardwareId);
    }

    /**
     * Use a secret key file as additional source "keyfile" of the machine
     * identity (protects against attackers who can read files of the machine,
     * e.g. through a file disclosure bug). A missing file is created with 32
     * random bytes and permissions 0600. Values encrypted with a key file can
     * only be decrypted with the same file: losing it loses the passwords.
     * Call before load(); null removes the key file. The path must lie under
     * the script directory, the CWD or the home directory.
     *
     * @throws \RuntimeException If the file cannot be read or created
     */
    public static function setKeyFile(?string $path): void
    {
        I18n::initialize();
        Crypto::setKeyFile($path === null ? null : self::resolveEnvFilePath($path));
    }

    /**
     * Strict mode: refuse legacy values (encrypted before version 3.0.0, small
     * key space) instead of decrypting and migrating them.
     */
    public static function setRejectLegacy(bool $reject): void
    {
        Crypto::setRejectLegacy($reject);
    }

    /**
     * Set the handler for warnings (migrated legacy values, readable files,
     * .env inside the web root). null restores the default (error_log()).
     *
     * @param callable(string): void|null $handler
     */
    public static function setWarningHandler(?callable $handler): void
    {
        Warnings::setHandler($handler);
    }

    /**
     * Control whether decrypted passwords (<NAME>_PASSWORD) are exported to
     * $_ENV and the process environment (putenv). Default true for
     * compatibility. With false they are only available via env()/get() and
     * are not inherited by child processes or shown by phpinfo().
     */
    public static function setExportPasswords(bool $export): void
    {
        self::$exportPasswords = $export;
    }

    private static function isPasswordKey(string $key): bool
    {
        return preg_match('/_PASSWORD$/i', $key) === 1 && preg_match('/_SECURE_PASSWORD$/i', $key) !== 1;
    }

    /**
     * Write a value to $_ENV and the process environment, unless it is a
     * password and exporting passwords is disabled.
     */
    private static function export(string $key, string $value): void
    {
        if (! self::$exportPasswords && self::isPasswordKey($key)) {
            return;
        }
        $_ENV[$key] = $value;
        putenv("{$key}={$value}");
    }

    /**
     * Warn if the .env file lies inside the web document root.
     */
    private static function warnIfInDocumentRoot(string $filePath): void
    {
        $docRoot = $_SERVER['DOCUMENT_ROOT'] ?? '';
        if (! is_string($docRoot) || $docRoot === '') {
            return;
        }
        $docReal = realpath($docRoot);
        if ($docReal === false) {
            return;
        }
        $prefix = rtrim(str_replace('\\', '/', $docReal), '/') . '/';
        if (strncmp(str_replace('\\', '/', $filePath), $prefix, strlen($prefix)) === 0) {
            Warnings::emit(I18n::t('config.warn_docroot', $filePath));
        }
    }

    /**
     * Password marker of the current language (always the built-in text).
     */
    private static function getPasswordMarker(): string
    {
        return I18n::builtinText('config.password_message');
    }

    private static function isPasswordMarker(string $value): bool
    {
        if ($value === '') {
            return false;
        }

        return $value === self::getPasswordMarker()
            || $value === I18n::builtinText('config.password_message', 'en')
            || $value === I18n::builtinText('config.password_message', 'de');
    }

    /**
     * Determine the user's home directory (%USERPROFILE% on Windows, $HOME on
     * Unix), mirroring Go's os.UserHomeDir().
     *
     * @return string|null Home directory, or null if it cannot be determined
     */
    private static function getHomeDirectory(): ?string
    {
        if (self::$homeDirOverride !== null) {
            return self::$homeDirOverride;
        }
        $home = getenv(PHP_OS_FAMILY === 'Windows' ? 'USERPROFILE' : 'HOME');
        if (is_string($home) && $home !== '') {
            return $home;
        }
        $hasPosix = function_exists('posix_getpwuid') && function_exists('posix_geteuid');
        if (PHP_OS_FAMILY !== 'Windows' && $hasPosix) {
            $info = posix_getpwuid(posix_geteuid());
            if (is_array($info) && is_string($info['dir'] ?? null) && $info['dir'] !== '') {
                return $info['dir'];
            }
        }

        return null;
    }

    /**
     * @return string Absolute base directory for .env path resolution
     */
    private static function getExecutableDirectoryForEnvPaths(): string
    {
        if (self::$executableRootOverride !== null) {
            return self::$executableRootOverride;
        }
        $script = $_SERVER['SCRIPT_FILENAME'] ?? null;
        if (is_string($script) && $script !== '') {
            $dir = dirname($script);
            $real = realpath($dir);

            return $real !== false ? $real : $dir;
        }
        $cwd = getcwd();

        return $cwd !== false ? $cwd : '.';
    }

    private static function isAbsolutePath(string $path): bool
    {
        if ($path === '') {
            return false;
        }
        if (PHP_OS_FAMILY === 'Windows') {
            return preg_match('/^[A-Za-z]:[\\\\\\/]|^\\\\\\\\/', $path) === 1;
        }

        return $path[0] === '/' || $path[0] === '\\';
    }

    /**
     * Clean path and ensure it lies under the script/executable base, the
     * current working directory (getcwd()) or the user's home directory
     * (%USERPROFILE% on Windows, ~ on Unix, e.g. AppData\Roaming or ~/.config).
     * Symbolic links are resolved (realpath).
     *
     * @throws \RuntimeException If the path is invalid or escapes all bases
     */
    private static function resolveEnvFilePath(string $filePath): string
    {
        $sep = DIRECTORY_SEPARATOR;
        $clean = str_replace(['\\', '/'], $sep, $filePath);
        // Resolve like Go filepath.Abs: relative paths are relative to CWD.
        if (! self::isAbsolutePath($filePath)) {
            $cwd = getcwd();
            if ($cwd === false) {
                throw new \RuntimeException(I18n::t('config.path_invalid', $filePath));
            }
            $abs = $cwd . $sep . ltrim($clean, $sep);
        } else {
            $abs = $clean;
        }
        $rp = realpath($abs);
        if ($rp !== false) {
            $final = $rp;
        } else {
            $dirReal = realpath(dirname($abs));
            if ($dirReal === false) {
                throw new \RuntimeException(I18n::t('config.path_invalid', $filePath));
            }
            $final = $dirReal . $sep . basename($abs);
        }

        $bases = [self::getExecutableDirectoryForEnvPaths(), getcwd() ?: '.'];
        $home = self::getHomeDirectory();
        if ($home !== null) {
            $bases[] = $home;
        }
        $finalNorm = str_replace('\\', '/', $final);
        foreach ($bases as $base) {
            $baseReal = realpath($base);
            if ($baseReal === false) {
                continue;
            }
            $basePrefix = rtrim(str_replace('\\', '/', $baseReal), '/');
            if ($finalNorm === $basePrefix || strncmp($finalNorm, $basePrefix . '/', strlen($basePrefix) + 1) === 0) {
                return $final;
            }
        }

        throw new \RuntimeException(I18n::t('config.path_outside_executable', $final));
    }

    /**
     * Load environment variables from a .env file with password encryption support
     *
     * @param string $filePath Path to the .env file (under script/executable dir, CWD or home dir)
     * @param bool $override Whether to override existing environment variables
     * @param bool $cleanConfig If true, decrypts passwords before writing (use with care)
     * @return void
     * @throws \RuntimeException If the file cannot be read, decrypted or written
     */
    public static function load(string $filePath, bool $override = false, bool $cleanConfig = false): void
    {
        I18n::initialize();

        $filePath = self::resolveEnvFilePath($filePath);
        self::$filePath = $filePath;
        self::warnIfInDocumentRoot($filePath);

        if (! file_exists($filePath)) {
            EnvFile::writeAtomic($filePath, '');
        }
        if (! is_readable($filePath)) {
            throw new \RuntimeException(I18n::t('config.read_failed', $filePath));
        }

        [$parsed, $entries] = EnvFile::parse(EnvFile::readLines($filePath));

        if (! $cleanConfig) {
            $changed = self::processPasswords($parsed);
        } else {
            self::decryptPasswords($parsed);
            $changed = true;
        }

        // Update cache and environment
        foreach ($parsed as $key => $value) {
            if ($override || ! isset(self::$cache[$key])) {
                self::$cache[$key] = $value;
                self::$origin[$key] = $filePath;
                if ($override || ! isset($_ENV[$key])) {
                    self::export($key, $value);
                }
            }
        }

        // Write back before decrypting, so the file keeps marker + encrypted value
        if ($changed) {
            EnvFile::writeAtomic($filePath, EnvFile::render($entries, $parsed));
        }

        // Decrypt passwords in memory for transparent access
        if (! $cleanConfig) {
            self::decryptPasswords($parsed);
            // Always replace the marker of password keys, else respect $override
            foreach ($parsed as $key => $value) {
                if ($override || self::isPasswordKey($key) || ! isset(self::$cache[$key])) {
                    self::$cache[$key] = $value;
                    self::$origin[$key] = self::$origin[$key] ?? $filePath;
                    self::export($key, $value);
                }
            }
        }

        self::$loaded = true;
    }

    /**
     * Encrypt plaintext passwords (v2), migrate legacy values to v2 and
     * re-encrypt v2 values that use fewer sources than are available now.
     * Warnings are issued for migrated and for not migratable legacy values.
     *
     * @param array<string, string> $parsed Parsed key-value pairs (modified in place)
     * @return bool True if any changes were made
     */
    private static function processPasswords(array &$parsed): bool
    {
        $changed = false;
        $migrated = 0;
        $failed = 0;
        foreach ($parsed as $key => $value) {
            if (
                preg_match('/^(.+)_PASSWORD$/i', $key, $matches) !== 1 ||
                preg_match('/^(.+)_SECURE_PASSWORD$/i', $key) === 1
            ) {
                continue;
            }
            $prefix = $matches[1];
            $secureKey = $prefix . '_SECURE_PASSWORD';
            $secureValue = $parsed[$secureKey] ?? '';

            if (self::isPasswordMarker($value)) {
                if ($secureValue === '') {
                    throw new \RuntimeException(I18n::t('config.secure_password_missing', $prefix));
                }
                $legacy = ! Crypto::isV2($secureValue);
                if ($legacy || Crypto::needsUpgrade($secureValue)) {
                    $plain = self::decryptValue($prefix, $secureValue);

                    try {
                        $parsed[$secureKey] = Crypto::encrypt($plain, $prefix);
                        $changed = true;
                        $migrated += $legacy ? 1 : 0;
                    } catch (\RuntimeException $e) {
                        // keep the readable old value until a v2 identity exists
                        $failed += $legacy ? 1 : 0;
                    }
                }

                continue;
            }

            if ($value !== '') {
                $parsed[$secureKey] = Crypto::encrypt($value, $prefix);
                $parsed[$key] = self::getPasswordMarker();
                $changed = true;
            }
        }
        if ($migrated > 0) {
            Warnings::emit(I18n::t('config.warn_migrated', $migrated));
        }
        if ($failed > 0) {
            Warnings::emit(I18n::t('config.warn_migration_failed', $failed));
        }

        return $changed;
    }

    /**
     * Decrypt passwords in memory for transparent access
     *
     * @param array<string, string> $parsed Parsed key-value pairs (modified in place)
     * @return void
     */
    private static function decryptPasswords(array &$parsed): void
    {
        foreach ($parsed as $key => $value) {
            if (preg_match('/^(.+)_SECURE_PASSWORD$/i', $key, $matches) !== 1) {
                continue;
            }
            $prefix = $matches[1];
            $passwordKey = $prefix . '_PASSWORD';
            if ($value === '') {
                if (self::isPasswordMarker($parsed[$passwordKey] ?? '')) {
                    throw new \RuntimeException(I18n::t('config.secure_password_missing', $prefix));
                }

                continue;
            }
            $parsed[$passwordKey] = self::decryptValue($prefix, $value);
        }
    }

    /**
     * Decrypt one value and map errors to the user messages.
     *
     * @throws \RuntimeException
     */
    private static function decryptValue(string $prefix, string $value): string
    {
        try {
            return Crypto::decrypt($value, $prefix);
        } catch (InvalidCiphertextException $e) {
            throw new \RuntimeException(
                I18n::t('config.encrypted_password_invalid', $prefix, $e->getMessage()),
                0,
                $e
            );
        } catch (\RuntimeException $e) {
            throw new \RuntimeException(I18n::t('config.decrypt_failed', $prefix, $e->getMessage()), 0, $e);
        }
    }

    /**
     * Get an environment variable value: loaded/set values first, then $_ENV,
     * then getenv(). Values from the process environment are not cached, so
     * updateEnv() never writes them into a file.
     *
     * @param string $key The environment variable key
     * @param mixed $default Default value if key is not found
     * @return mixed The environment variable value or default
     */
    public static function get(string $key, $default = null)
    {
        if (isset(self::$cache[$key])) {
            return self::$cache[$key];
        }
        if (isset($_ENV[$key])) {
            return $_ENV[$key];
        }
        $value = getenv($key);
        if ($value !== false) {
            return $value;
        }

        return $default;
    }

    /**
     * Check if an environment variable exists
     *
     * @param string $key The environment variable key
     * @return bool True if the variable exists
     */
    public static function has(string $key): bool
    {
        return isset(self::$cache[$key]) ||
               isset($_ENV[$key]) ||
               getenv($key) !== false;
    }

    /**
     * Set an environment variable (cache and $_ENV/putenv).
     * Use together with updateEnv() to persist changes to the .env file.
     * Example: after user changes theme to "light", call set('THEME', 'light') then updateEnv('.env').
     *
     * @param string $key The environment variable key (letters, digits, "_", ".")
     * @param string $value The value to set (no control characters except tab)
     * @return void
     * @throws \InvalidArgumentException For invalid names or values (e.g. line
     *         breaks, which would inject further variables into the file)
     */
    public static function set(string $key, string $value): void
    {
        I18n::initialize();
        EnvFile::validateKey($key);
        EnvFile::validateValue($key, $value);
        self::$cache[$key] = $value;
        self::$origin[$key] = self::ORIGIN_SET;
        self::export($key, $value);
    }

    /**
     * Write the loaded and set() values to a .env file. Requires load() first.
     * Secure password fields are encrypted in the file unless $cleanConfig is
     * true. The path may differ from the one used in load() (e.g. backup.env).
     * Written are the set() values plus the values loaded from this file, or,
     * for a file that was not loaded (backup), from the last loaded file.
     * Values that only come from the process environment are never written.
     *
     * @param string $filePath Path to the .env file to write
     * @param bool $cleanConfig If true, passwords are written in plaintext (use with care)
     * @return void
     * @throws \RuntimeException If load() was not called before, or file cannot be written
     */
    public static function updateEnv(string $filePath, bool $cleanConfig = false): void
    {
        I18n::initialize();
        if (! self::$loaded) {
            throw new \RuntimeException(I18n::t('config.load_first'));
        }
        $filePath = self::resolveEnvFilePath($filePath);
        $parsed = [];
        $entries = [];
        if (file_exists($filePath) && is_readable($filePath)) {
            [$parsed, $entries] = EnvFile::parse(EnvFile::readLines($filePath));
        }
        $source = in_array($filePath, self::$origin, true) ? $filePath : self::$filePath;
        foreach (self::$cache as $key => $value) {
            $origin = self::$origin[$key] ?? null;
            if ($origin === self::ORIGIN_SET || $origin === $source) {
                $parsed[$key] = $value;
            }
        }
        if ($cleanConfig) {
            self::decryptPasswords($parsed);
        } else {
            self::processPasswords($parsed);
        }
        EnvFile::writeAtomic($filePath, EnvFile::render($entries, $parsed));
    }

    /**
     * Clear the cache and reset loaded state
     *
     * @return void
     */
    public static function clear(): void
    {
        self::$cache = [];
        self::$origin = [];
        self::$loaded = false;
        self::$filePath = null;
    }

    /**
     * Check if environment has been loaded
     *
     * @return bool True if environment has been loaded
     */
    public static function isLoaded(): bool
    {
        return self::$loaded;
    }
}
