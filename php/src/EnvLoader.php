<?php

namespace Sconfig;

/**
 * Decription: Loads and manages environment variables from .env files with secure password handling
 *
 * Kernfunktion (wie bei der Go-Variante): Der Schlüssel ist absichtlich nicht zufällig pro
 * Aufruf, sondern auf derselben Maschine immer derselbe, damit .env-Daten lesbar bleiben.
 * Ziel ist ein nur-lokal reproduzierbares Geheimnis, nicht ein frisches Zufallsgeheimnis
 * bei jedem Start.
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
 * @version 1.2.0
 *
 * Changelog:
 * 1.2.0 27.02.26 set(), updateEnv(): persist changes (e.g. theme); require load() first; may write to different path
 * 1.1.0 24.11.25 Initial build of the PHP variant of the sconfig library
 *
 *
 */
class EnvLoader
{
    /**
     * Optional override for the directory under which .env paths must lie
     * (tests and applications that set an explicit base).
     */
    private static ?string $executableRootOverride = null;
    /**
     * Get the password marker string for the current language
     *
     * @return string Marker string indicating password is encrypted
     */
    private static function getPasswordMarker(): string
    {
        I18n::initialize();
        return I18n::t('config.password_message');
    }

    /**
     * Get password marker strings for both languages (for comparison)
     *
     * @return array<string> Array with 'en' and 'de' markers
     */
    private static function getPasswordMarkers(): array
    {
        I18n::initialize();
        $currentLang = I18n::getCurrentLanguage();
        
        // Get English marker
        I18n::setLanguage('en');
        $enMarker = I18n::t('config.password_message');
        
        // Get German marker
        I18n::setLanguage('de');
        $deMarker = I18n::t('config.password_message');
        
        // Restore original language
        I18n::setLanguage($currentLang);
        
        return ['en' => $enMarker, 'de' => $deMarker];
    }

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
     * Set the directory that contains the application entry script (or the
     * executable directory). Paths passed to load() / updateEnv() must resolve
     * under this directory or under the current working directory. Call from
     * application bootstrap if the default (directory of SCRIPT_FILENAME) is
     * not appropriate.
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
     * @return string Absolute base directory for .env path resolution
     */
    private static function getExecutableDirectoryForEnvPaths(): string
    {
        if (self::$executableRootOverride !== null) {
            return self::$executableRootOverride;
        }
        if (isset($_SERVER['SCRIPT_FILENAME']) && is_string($_SERVER['SCRIPT_FILENAME']) && $_SERVER['SCRIPT_FILENAME'] !== '') {
            $dir = dirname($_SERVER['SCRIPT_FILENAME']);
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
     * Clean path and ensure it lies under the script/executable base or the
     * current working directory (getcwd()).
     *
     * @throws \RuntimeException If the path is invalid or escapes both bases
     */
    private static function resolveEnvFilePath(string $filePath): string
    {
        I18n::initialize();
        $sep = DIRECTORY_SEPARATOR;
        $clean = str_replace(['\\', '/'], $sep, $filePath);
        // Resolve like Go filepath.Abs: relative paths are relative to CWD.
        if (!self::isAbsolutePath($filePath)) {
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
            $dir = dirname($abs);
            $dirReal = realpath($dir);
            if ($dirReal === false) {
                throw new \RuntimeException(I18n::t('config.path_invalid', $filePath));
            }
            $final = $dirReal . $sep . basename($abs);
        }

        $bases = [];
        $scriptBase = self::getExecutableDirectoryForEnvPaths();
        $scriptReal = realpath($scriptBase);
        if ($scriptReal !== false) {
            $bases[] = $scriptReal;
        }
        $cwdReal = realpath(getcwd() ?: '.');
        if ($cwdReal !== false) {
            $bases[] = $cwdReal;
        }
        $bases = array_values(array_unique($bases));

        $finalNorm = str_replace('\\', '/', $final);
        foreach ($bases as $baseReal) {
            $basePrefix = rtrim(str_replace('\\', '/', $baseReal), '/');
            if ($finalNorm === $basePrefix || str_starts_with($finalNorm, $basePrefix . '/')) {
                return $final;
            }
        }

        throw new \RuntimeException(I18n::t('config.path_outside_executable', $final));
    }

    /**
     * Load environment variables from a .env file with password encryption support
     *
     * @param string $filePath Path to the .env file (must lie under the script/executable directory or CWD)
     * @param bool $override Whether to override existing environment variables
     * @param bool $cleanConfig If true, decrypts passwords before writing (use with care)
     * @return void
     * @throws \RuntimeException If the file cannot be read or processed
     */
    public static function load(string $filePath, bool $override = false, bool $cleanConfig = false): void
    {
        // Initialize i18n first
        I18n::initialize();
        
        self::initializeEncryption();

        $filePath = self::resolveEnvFilePath($filePath);
        self::$filePath = $filePath;
        $changed = false;

        if (!file_exists($filePath)) {
            // Create empty file if it doesn't exist
            file_put_contents($filePath, '');
        }

        if (!is_readable($filePath)) {
            throw new \RuntimeException(I18n::t('config.read_failed', $filePath));
        }

        $lines = file($filePath, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
        if ($lines === false) {
            throw new \RuntimeException(I18n::t('config.read_failed', $filePath));
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

        // Write back to file if changed (before decrypting, so file keeps marker + encrypted)
        if ($changed) {
            self::writeEnvFile($filePath, $parsed, $lines);
        }

        // Decrypt passwords in memory for transparent access
        if (!$cleanConfig) {
            self::decryptPasswords($parsed);
            // Update cache: always for decrypted password keys (replace marker), else respect override
            foreach ($parsed as $key => $value) {
                $isPasswordKey = preg_match('/_PASSWORD$/i', $key) && !preg_match('/_SECURE_PASSWORD$/i', $key);
                if ($override || $isPasswordKey || !isset(self::$cache[$key])) {
                    self::$cache[$key] = $value;
                    $_ENV[$key] = $value;
                    putenv("{$key}={$value}");
                }
            }
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
        $markers = self::getPasswordMarkers();

        foreach ($parsed as $key => $value) {
            // Check if this is a Password field (not SecurePassword)
            if (preg_match('/^(.+)_PASSWORD$/i', $key, $matches) && 
                !preg_match('/^(.+)_SECURE_PASSWORD$/i', $key)) {
                $prefix = $matches[1];
                $securePasswordKey = $prefix . '_SECURE_PASSWORD';

                // Check if password is plaintext (not the marker)
                if ($value !== $markers['en'] && 
                    $value !== $markers['de'] &&
                    $value !== '') {
                    // Encrypt the password
                    $encrypted = self::encrypt($value);
                    
                    // Ensure SECURE_PASSWORD field exists (create if it doesn't)
                    if (!isset($parsed[$securePasswordKey])) {
                        $parsed[$securePasswordKey] = '';
                    }
                    
                    // Set encrypted value in SECURE_PASSWORD
                    $parsed[$securePasswordKey] = $encrypted;
                    // Replace plaintext with marker
                    $parsed[$key] = self::getPasswordMarker();
                    $changed = true;
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
                        I18n::initialize();
                        $prefix = preg_replace('/_SECURE_PASSWORD$/i', '', $key);
                        throw new \RuntimeException(I18n::t('config.decrypt_failed', $prefix) . ': ' . $e->getMessage());
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
     * Intentional use of mt_rand (not random_bytes): The same hardware ID must
     * always produce the same encryption key so that config files remain
     * decryptable on the same machine. Security is provided by the
     * hardware-derived input being unknowable to anyone without full access to
     * the machine; the PRNG is used only for deterministic expansion of that
     * secret seed into 32 key bytes. See securityreport.md and SECURITY.md.
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
            // Deterministic expansion from hardware ID (same seed => same key, by design)
            // Aligned with Go: math/rand.NewSource(hardwareID) for same-machine decrypt
            mt_srand($hardwareId);
            $key = '';
            for ($i = 0; $i < 32; $i++) {
                // Simulate Int63() >> 16 & 0xff using xor for deterministic byte sequence
                $val = mt_rand() ^ mt_rand();
                $key .= chr($val & 0xff);
            }
            self::$encryptionKey = $key;
        } catch (\Exception $e) {
            throw new \RuntimeException("Failed to initialize encryption: " . $e->getMessage());
        }

        self::$encryptionInitialized = true;
    }

    /**
     * Check if the system is running on a virtual machine
     * Uses multiple detection methods for reliability
     *
     * @return bool True if running on a VM, false otherwise
     */
    private static function isVirtualMachine(): bool
    {
        if (PHP_OS_FAMILY === 'Windows') {
            // Windows VM detection using WMI
            $output = @shell_exec('wmic computersystem get Manufacturer,Model /value 2>nul');
            if ($output) {
                $manufacturer = '';
                $model = '';
                $lines = explode("\n", $output);
                foreach ($lines as $line) {
                    if (strpos($line, 'Manufacturer=') === 0) {
                        $manufacturer = strtolower(trim(substr($line, 13)));
                    } elseif (strpos($line, 'Model=') === 0) {
                        $model = strtolower(trim(substr($line, 6)));
                    }
                }
                
                $vmIndicators = [
                    'vmware', 'virtualbox', 'microsoft corporation', 'xen',
                    'parallels', 'qemu', 'kvm', 'innotek', 'bochs'
                ];
                
                foreach ($vmIndicators as $indicator) {
                    if (strpos($manufacturer, $indicator) !== false || 
                        strpos($model, $indicator) !== false) {
                        return true;
                    }
                }
                
                // Check for Hyper-V (Virtual Machine in Model)
                if (strpos($model, 'virtual') !== false) {
                    return true;
                }
            }
            return false;
        }

        if (PHP_OS_FAMILY !== 'Linux') {
            return false;
        }

        // Method 1: systemd-detect-virt (most reliable)
        $output = @shell_exec('systemd-detect-virt 2>/dev/null');
        if ($output) {
            $virt = trim($output);
            // Returns "none" on bare metal, or VM type (kvm, vmware, qemu, etc.)
            if ($virt !== 'none' && $virt !== '') {
                return true;
            }
        }

        // Method 2: Check DMI vendor/product
        $checks = [
            '/sys/class/dmi/id/sys_vendor',
            '/sys/class/dmi/id/product_name',
            '/sys/class/dmi/id/chassis_vendor',
        ];

        $vmIndicators = [
            'qemu', 'kvm', 'vmware', 'virtualbox', 'xen',
            'parallels', 'microsoft', 'bochs', 'bhyve'
        ];

        foreach ($checks as $file) {
            $content = @file_get_contents($file);
            if ($content) {
                $content = strtolower(trim($content));
                foreach ($vmIndicators as $indicator) {
                    if (strpos($content, $indicator) !== false) {
                        return true;
                    }
                }
            }
        }

        return false;
    }

    /**
     * Get hardware ID for encryption key generation
     * On virtual machines, prioritizes stable identifiers like machine-id and product_uuid
     *
     * @return int 64-bit hardware identifier
     * @throws \RuntimeException If hardware ID cannot be determined
     */
    private static function getHardwareID(): int
    {
        $identifiers = [];
        $isVM = self::isVirtualMachine();

        // Try to get MAC address
        if (PHP_OS_FAMILY === 'Windows') {
            if ($isVM) {
                // For Windows VMs: prioritize stable identifiers
                // 1. MachineGuid from Registry (very stable on Windows)
                $output = @shell_exec('reg query "HKLM\\SOFTWARE\\Microsoft\\Cryptography" /v MachineGuid 2>nul');
                if ($output) {
                    // Parse: HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography
                    //     MachineGuid    REG_SZ    {GUID}
                    $lines = explode("\n", $output);
                    foreach ($lines as $line) {
                        if (preg_match('/MachineGuid\s+REG_SZ\s+(.+)/i', $line, $matches)) {
                            $machineGuid = trim($matches[1]);
                            if (!empty($machineGuid)) {
                                $identifiers[] = $machineGuid;
                                break;
                            }
                        }
                    }
                }
                
                // 2. SMBIOS UUID (usually stable on VMs)
                $output = @shell_exec('wmic csproduct get UUID /value 2>nul');
                if ($output) {
                    $lines = explode("\n", trim($output));
                    foreach ($lines as $line) {
                        if (strpos($line, 'UUID=') === 0) {
                            $uuid = trim(substr($line, 5));
                            if (!empty($uuid)) {
                                $identifiers[] = $uuid;
                                break;
                            }
                        }
                    }
                }
            }

            // Common identifiers (for both VM and physical)
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
            // On VMs, skip CPU ProcessorId as it's often unreliable
            $commands = [
                'wmic baseboard get SerialNumber /value',
                'wmic baseboard get Product /value',
                'wmic diskdrive get SerialNumber /value',
            ];
            
            if (!$isVM) {
                // Only use CPU ProcessorId on physical machines
                $commands[] = 'wmic cpu get ProcessorId /value';
            }

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
            if ($isVM) {
                // For VMs: prioritize stable identifiers
                // 1. machine-id (very stable on VMs)
                $machineId = @file_get_contents('/etc/machine-id');
                if ($machineId && trim($machineId) !== '') {
                    $identifiers[] = trim($machineId);
                }
                
                // 2. product_uuid (usually stable on VMs)
                $productUuid = @file_get_contents('/sys/class/dmi/id/product_uuid');
                if ($productUuid && trim($productUuid) !== '') {
                    $identifiers[] = trim($productUuid);
                }
            }

            // Common identifiers (for both VM and physical)
            $commands = [
                "cat /sys/class/net/*/address 2>/dev/null | head -1",
                "cat /sys/class/dmi/id/board_serial 2>/dev/null",
            ];

            // Only add CPU serial if not on VM (often unreliable on VMs)
            if (!$isVM) {
                $commands[] = "cat /proc/cpuinfo | grep 'Serial' | head -1";
            }

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
        I18n::initialize();
        
        if (self::$encryptionKey === null) {
            throw new \RuntimeException(I18n::t('config.hardware_id_failed'));
        }

        $method = 'aes-256-gcm';
        $ivLength = openssl_cipher_iv_length($method);
        $iv = openssl_random_pseudo_bytes($ivLength);
        
        $tag = '';
        $encrypted = openssl_encrypt($text, $method, self::$encryptionKey, OPENSSL_RAW_DATA, $iv, $tag);
        
        if ($encrypted === false) {
            throw new \RuntimeException(I18n::t('config.failed_checking', 'encryption'));
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
        I18n::initialize();
        
        if (self::$encryptionKey === null) {
            throw new \RuntimeException(I18n::t('config.hardware_id_failed'));
        }

        $data = base64_decode($encryptedText, true);
        if ($data === false) {
            throw new \RuntimeException(I18n::t('config.failed_parsing', 'base64 data'));
        }

        $method = 'aes-256-gcm';
        $ivLength = openssl_cipher_iv_length($method);
        $tagLength = 16; // GCM tag is always 16 bytes

        if (strlen($data) < $ivLength + $tagLength) {
            throw new \RuntimeException(I18n::t('config.failed_parsing', 'encrypted data'));
        }

        $iv = substr($data, 0, $ivLength);
        $tag = substr($data, $ivLength, $tagLength);
        $ciphertext = substr($data, $ivLength + $tagLength);

        $decrypted = openssl_decrypt($ciphertext, $method, self::$encryptionKey, OPENSSL_RAW_DATA, $iv, $tag);
        
        if ($decrypted === false) {
            throw new \RuntimeException(I18n::t('config.decrypt_failed', '', 'decryption'));
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
     * Set an environment variable (cache and $_ENV/putenv).
     * Use together with updateEnv() to persist changes to the .env file.
     * Example: after user changes theme to "light", call set('THEME', 'light') then updateEnv('.env').
     *
     * @param string $key The environment variable key
     * @param string $value The value to set
     * @return void
     */
    public static function set(string $key, string $value): void
    {
        self::$cache[$key] = $value;
        $_ENV[$key] = $value;
        putenv("{$key}={$value}");
    }

    /**
     * Write current cache to .env file. Requires load() to have been called first.
     * Secure password fields are encrypted in the file unless $cleanConfig is true.
     * The path may differ from the one used in load() (e.g. backup.env).
     *
     * @param string $filePath Path to the .env file to write
     * @param bool $cleanConfig If true, passwords are written in plaintext (use with care)
     * @return void
     * @throws \RuntimeException If load() was not called before, or file cannot be written
     */
    public static function updateEnv(string $filePath, bool $cleanConfig = false): void
    {
        I18n::initialize();
        if (!self::$loaded || !self::$encryptionInitialized) {
            throw new \RuntimeException(I18n::t('config.load_first'));
        }
        $filePath = self::resolveEnvFilePath($filePath);
        $lines = [];
        $parsed = [];
        if (file_exists($filePath) && is_readable($filePath)) {
            $lines = file($filePath, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
            if ($lines === false) {
                $lines = [];
            } else {
                foreach ($lines as $line) {
                    $trimmed = trim($line);
                    if (empty($trimmed) || strpos($trimmed, '#') === 0) {
                        continue;
                    }
                    if (strpos($line, '=') !== false) {
                        [$key, $value] = explode('=', $line, 2);
                        $key = trim($key);
                        $value = trim($value);
                        if ((substr($value, 0, 1) === '"' && substr($value, -1) === '"') ||
                            (substr($value, 0, 1) === "'" && substr($value, -1) === "'")) {
                            $value = substr($value, 1, -1);
                        }
                        $parsed[$key] = $value;
                    }
                }
            }
        }
        foreach (self::$cache as $key => $value) {
            $parsed[$key] = $value;
        }
        if ($cleanConfig) {
            self::decryptPasswords($parsed);
        } else {
            self::processPasswords($parsed);
        }
        self::writeEnvFile($filePath, $parsed, $lines);
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
