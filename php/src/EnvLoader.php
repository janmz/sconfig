<?php

namespace Sconfig;

/**
 * EnvLoader - Loads and manages environment variables from .env files with secure password handling
 *
 * This class provides functionality to load environment variables from a .env file,
 * automatically encrypt passwords, and make them accessible via the env() helper function.
 *
 * Password handling:
 * - Fields named `<NAME>_PASSWORD` and `<NAME>_SECURE_PASSWORD` are treated as a pair
 * - If `<NAME>_PASSWORD` contains a plaintext password (not the marker), it will be encrypted
 * - The encrypted value is stored in `<NAME>_SECURE_PASSWORD`
 * - `<NAME>_PASSWORD` is replaced with a marker string
 * - In memory, passwords are automatically decrypted for transparent access
 *
 * @package Sconfig
 * @author Jan Neuhaus, VAYA Consulting
 */
class EnvLoader
{
    /**
     * Marker string that indicates a password is encrypted
     * English: "Enter new password here"
     * German: "Hier neues Passwort eintragen"
     */
    private const PASSWORD_IS_SECURE_EN = "Enter new password here";
    private const PASSWORD_IS_SECURE_DE = "Hier neues Passwort eintragen";

    /**
     * @var array<string, string> Cache of loaded environment variables
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
     * @var string|null Encryption key derived from hardware ID
     */
    private static ?string $encryptionKey = null;

    /**
     * @var bool Whether encryption has been initialized
     */
    private static bool $encryptionInitialized = false;

    /**
     * Load environment variables from a .env file with password encryption support
     *
     * @param string $filePath Path to the .env file
     * @param bool $override Whether to override existing environment variables
     * @param bool $cleanConfig If true, decrypts passwords before writing (use with care)
     * @return void
     * @throws \RuntimeException If the file cannot be read or processed
     */
    public static function load(string $filePath, bool $override = false, bool $cleanConfig = false): void
    {
        self::initializeEncryption();

        self::$filePath = $filePath;
        $changed = false;

        if (!file_exists($filePath)) {
            // Create empty file if it doesn't exist
            file_put_contents($filePath, '');
        }

        if (!is_readable($filePath)) {
            throw new \RuntimeException("Environment file is not readable: {$filePath}");
        }

        $lines = file($filePath, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
        if ($lines === false) {
            throw new \RuntimeException("Failed to read environment file: {$filePath}");
        }

        // Parse all lines into key-value pairs, preserving comments and structure
        $parsed = [];
        $lineData = [];

        foreach ($lines as $lineNum => $line) {
            $trimmed = trim($line);
            
            // Preserve comments and empty lines
            if (empty($trimmed) || strpos($trimmed, '#') === 0) {
                $lineData[] = ['type' => 'comment', 'content' => $line];
                continue;
            }

            // Parse KEY=VALUE format
            if (strpos($line, '=') !== false) {
                [$key, $value] = explode('=', $line, 2);
                $key = trim($key);
                $value = trim($value);

                // Remove quotes if present
                if ((substr($value, 0, 1) === '"' && substr($value, -1) === '"') ||
                    (substr($value, 0, 1) === "'" && substr($value, -1) === "'")) {
                    $value = substr($value, 1, -1);
                }

                $parsed[$key] = $value;
                $lineData[] = ['type' => 'variable', 'key' => $key, 'value' => $value, 'original' => $line];
            }
        }

        // Process password encryption/decryption
        if (!$cleanConfig) {
            $changed = self::processPasswords($parsed);
        } else {
            // Decrypt passwords for clean config
            self::decryptPasswords($parsed);
            $changed = true;
        }

        // Update cache and environment
        foreach ($parsed as $key => $value) {
            if ($override || !isset(self::$cache[$key])) {
                self::$cache[$key] = $value;
                
                if ($override || !isset($_ENV[$key])) {
                    $_ENV[$key] = $value;
                    putenv("{$key}={$value}");
                }
            }
        }

        // Decrypt passwords in memory for transparent access
        if (!$cleanConfig) {
            self::decryptPasswords($parsed);
            // Update cache with decrypted values
            foreach ($parsed as $key => $value) {
                self::$cache[$key] = $value;
                $_ENV[$key] = $value;
                putenv("{$key}={$value}");
            }
        }

        // Write back to file if changed
        if ($changed) {
            self::writeEnvFile($filePath, $parsed, $lines);
        }

        self::$loaded = true;
    }

    /**
     * Process password pairs: encrypt plaintext passwords
     *
     * @param array<string, string> $parsed Parsed key-value pairs (modified in place)
     * @return bool True if any changes were made
     */
    private static function processPasswords(array &$parsed): bool
    {
        $changed = false;

        foreach ($parsed as $key => $value) {
            // Check if this is a SecurePassword field
            if (preg_match('/^(.+)_SECURE_PASSWORD$/i', $key, $matches)) {
                $prefix = $matches[1];
                $passwordKey = $prefix . '_PASSWORD';

                // Check if corresponding Password field exists
                if (isset($parsed[$passwordKey])) {
                    $passwordValue = $parsed[$passwordKey];

                    // Check if password is plaintext (not the marker)
                    if ($passwordValue !== self::PASSWORD_IS_SECURE_EN && 
                        $passwordValue !== self::PASSWORD_IS_SECURE_DE &&
                        $passwordValue !== '') {
                        // Encrypt the password
                        $encrypted = self::encrypt($passwordValue);
                        $parsed[$key] = $encrypted;
                        $parsed[$passwordKey] = self::PASSWORD_IS_SECURE_EN;
                        $changed = true;
                    }
                }
            }
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
            // Check if this is a SecurePassword field
            if (preg_match('/^(.+)_SECURE_PASSWORD$/i', $key, $matches)) {
                $prefix = $matches[1];
                $passwordKey = $prefix . '_PASSWORD';

                // Decrypt and set in Password field
                if (!empty($value)) {
                    try {
                        $decrypted = self::decrypt($value);
                        $parsed[$passwordKey] = $decrypted;
                    } catch (\Exception $e) {
                        // If decryption fails, keep the secure password value
                        // This might happen if the file was moved to a different machine
                    }
                }
            }
        }
    }

    /**
     * Write environment variables back to .env file
     *
     * @param string $filePath Path to the .env file
     * @param array<string, string> $parsed Parsed key-value pairs
     * @param array<string> $originalLines Original lines for preserving structure
     * @return void
     */
    private static function writeEnvFile(string $filePath, array $parsed, array $originalLines): void
    {
        $output = [];
        $written = [];

        // First, write all original lines, updating values as needed
        foreach ($originalLines as $line) {
            $trimmed = trim($line);
            
            if (empty($trimmed) || strpos($trimmed, '#') === 0) {
                $output[] = $line;
                continue;
            }

            if (strpos($line, '=') !== false) {
                [$key] = explode('=', $line, 2);
                $key = trim($key);
                
                if (isset($parsed[$key])) {
                    $value = $parsed[$key];
                    // Preserve quotes if original had them
                    $quoted = false;
                    if (preg_match('/^["\']/', $line) && preg_match('/["\']$/', $line)) {
                        $quoted = true;
                        $quote = substr($line, strpos($line, '=') + 1, 1);
                    }
                    
                    if ($quoted) {
                        $output[] = $key . '=' . $quote . $value . $quote;
                    } else {
                        $output[] = $key . '=' . $value;
                    }
                    $written[$key] = true;
                } else {
                    $output[] = $line;
                }
            }
        }

        // Add any new keys that weren't in the original file
        foreach ($parsed as $key => $value) {
            if (!isset($written[$key])) {
                $output[] = $key . '=' . $value;
            }
        }

        file_put_contents($filePath, implode("\n", $output) . "\n");
    }

    /**
     * Initialize encryption system with hardware-based key
     *
     * @return void
     */
    private static function initializeEncryption(): void
    {
        if (self::$encryptionInitialized) {
            return;
        }

        try {
            $hardwareId = self::getHardwareID();
            // Generate encryption key from hardware ID (similar to Go implementation)
            // Go uses: byte(randGenSeeded.Int63() >> 16 & 0xff)
            // We simulate this by using the hardware ID as seed and generating bytes
            mt_srand($hardwareId);
            $key = '';
            for ($i = 0; $i < 32; $i++) {
                // Simulate Int63() >> 16 & 0xff
                // mt_rand() returns 32-bit, so we combine two calls for more entropy
                $val = (mt_rand() << 16) | mt_rand();
                $key .= chr(($val >> 16) & 0xff);
            }
            self::$encryptionKey = $key;
        } catch (\Exception $e) {
            throw new \RuntimeException("Failed to initialize encryption: " . $e->getMessage());
        }

        self::$encryptionInitialized = true;
    }

    /**
     * Get hardware ID for encryption key generation
     *
     * @return int 64-bit hardware identifier
     * @throws \RuntimeException If hardware ID cannot be determined
     */
    private static function getHardwareID(): int
    {
        $identifiers = [];

        // Try to get MAC address
        if (PHP_OS_FAMILY === 'Windows') {
            $output = @shell_exec('getmac /fo csv /nh 2>nul');
            if ($output) {
                $lines = explode("\n", trim($output));
                if (!empty($lines)) {
                    $parts = str_getcsv($lines[0]);
                    if (!empty($parts[0])) {
                        $identifiers[] = $parts[0];
                    }
                }
            }

            // Try Windows-specific hardware IDs
            $commands = [
                'wmic cpu get ProcessorId /value',
                'wmic baseboard get SerialNumber /value',
                'wmic baseboard get Product /value',
                'wmic diskdrive get SerialNumber /value',
            ];

            foreach ($commands as $cmd) {
                $output = @shell_exec($cmd . ' 2>nul');
                if ($output) {
                    $lines = explode("\n", trim($output));
                    foreach ($lines as $line) {
                        if (strpos($line, '=') !== false) {
                            [, $value] = explode('=', $line, 2);
                            $value = trim($value);
                            if (!empty($value)) {
                                $identifiers[] = $value;
                                break;
                            }
                        }
                    }
                }
            }
        } else {
            // Linux/Unix
            $commands = [
                "cat /sys/class/net/*/address 2>/dev/null | head -1",
                "cat /proc/cpuinfo | grep 'Serial' | head -1",
                "cat /sys/class/dmi/id/product_uuid 2>/dev/null",
                "cat /sys/class/dmi/id/board_serial 2>/dev/null",
            ];

            foreach ($commands as $cmd) {
                $output = @shell_exec($cmd);
                if ($output) {
                    $value = trim($output);
                    if (!empty($value)) {
                        $identifiers[] = $value;
                    }
                }
            }
        }

        if (empty($identifiers)) {
            // Fallback: use hostname and PHP version
            $identifiers[] = gethostname();
            $identifiers[] = PHP_VERSION;
        }

        // Combine identifiers and create hash
        $combined = implode('|', $identifiers);
        $hash = hash('sha256', $combined, true);
        
        // Return first 64 bits as integer (similar to Go implementation)
        $result = 0;
        for ($i = 0; $i < 8; $i++) {
            $result = ($result << 8) | ord($hash[$i]);
        }

        // Convert to signed 64-bit if needed (PHP doesn't have uint64)
        if ($result > PHP_INT_MAX) {
            $result = $result - (1 << 63) * 2;
        }

        return (int)$result;
    }

    /**
     * Encrypt text using AES-256-GCM
     *
     * @param string $text Plaintext to encrypt
     * @return string Base64-encoded encrypted text
     */
    private static function encrypt(string $text): string
    {
        if (self::$encryptionKey === null) {
            throw new \RuntimeException("Encryption not initialized");
        }

        $method = 'aes-256-gcm';
        $ivLength = openssl_cipher_iv_length($method);
        $iv = openssl_random_pseudo_bytes($ivLength);
        
        $tag = '';
        $encrypted = openssl_encrypt($text, $method, self::$encryptionKey, OPENSSL_RAW_DATA, $iv, $tag);
        
        if ($encrypted === false) {
            throw new \RuntimeException("Encryption failed");
        }

        // Combine IV, tag, and ciphertext
        $combined = $iv . $tag . $encrypted;
        return base64_encode($combined);
    }

    /**
     * Decrypt text using AES-256-GCM
     *
     * @param string $encryptedText Base64-encoded encrypted text
     * @return string Decrypted plaintext
     * @throws \RuntimeException If decryption fails
     */
    private static function decrypt(string $encryptedText): string
    {
        if (self::$encryptionKey === null) {
            throw new \RuntimeException("Encryption not initialized");
        }

        $data = base64_decode($encryptedText, true);
        if ($data === false) {
            throw new \RuntimeException("Invalid base64 data");
        }

        $method = 'aes-256-gcm';
        $ivLength = openssl_cipher_iv_length($method);
        $tagLength = 16; // GCM tag is always 16 bytes

        if (strlen($data) < $ivLength + $tagLength) {
            throw new \RuntimeException("Invalid encrypted data");
        }

        $iv = substr($data, 0, $ivLength);
        $tag = substr($data, $ivLength, $tagLength);
        $ciphertext = substr($data, $ivLength + $tagLength);

        $decrypted = openssl_decrypt($ciphertext, $method, self::$encryptionKey, OPENSSL_RAW_DATA, $iv, $tag);
        
        if ($decrypted === false) {
            throw new \RuntimeException("Decryption failed");
        }

        return $decrypted;
    }

    /**
     * Get an environment variable value
     *
     * @param string $key The environment variable key
     * @param mixed $default Default value if key is not found
     * @return mixed The environment variable value or default
     */
    public static function get(string $key, $default = null)
    {
        // Check cache first
        if (isset(self::$cache[$key])) {
            return self::$cache[$key];
        }

        // Check $_ENV
        if (isset($_ENV[$key])) {
            self::$cache[$key] = $_ENV[$key];
            return $_ENV[$key];
        }

        // Check getenv()
        $value = getenv($key);
        if ($value !== false) {
            self::$cache[$key] = $value;
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
     * Clear the cache and reset loaded state
     *
     * @return void
     */
    public static function clear(): void
    {
        self::$cache = [];
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
