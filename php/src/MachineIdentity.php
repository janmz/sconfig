<?php

namespace Sconfig;

/**
 * Machine identity "v3" (new values since 3.1.0) and "v2" (read and migrate).
 *
 * Same specification as the Go implementation (hwid_v2.go, hwid_v3.go),
 * verified by shared test vectors:
 *   v2 sources: Windows machineguid (registry, 64-bit view) and smbiosuuid
 *   (PowerShell CIM), Linux machineid (/etc/machine-id), macOS ioplatformuuid
 *   (ioreg), FreeBSD hostuuid (sysctl kern.hostuuid).
 *   v3 sources: all v2 sources plus hostname (gethostname()), machtype
 *   (native architecture, normalized) and fsid (file system of the config
 *   file: Windows volume serial number, Linux "stat -f -c %i", otherwise
 *   st_dev).
 *   Key: HKDF-SHA256 over "<name>=<value>\n" of the used sources sorted by
 *   name, 32 bytes; v2: salt "sconfig-v2", info "sconfig v2 aes-256-gcm";
 *   v3: salt "sconfig-v3", info "sconfig v3 aes-256-gcm".
 *
 * @package Sconfig
 * @internal
 */
final class MachineIdentity
{
    public const KDF_SALT = 'sconfig-v2';
    public const KDF_INFO = 'sconfig v2 aes-256-gcm';
    public const KDF_SALT_V3 = 'sconfig-v3';
    public const KDF_INFO_V3 = 'sconfig v3 aes-256-gcm';

    public const SOURCE_MACHINE_GUID = 'machineguid';
    public const SOURCE_SMBIOS_UUID = 'smbiosuuid';
    public const SOURCE_MACHINE_ID = 'machineid';
    public const SOURCE_IOPLATFORM_UUID = 'ioplatformuuid';
    public const SOURCE_HOST_UUID = 'hostuuid';
    public const SOURCE_OVERRIDE = 'override';
    public const SOURCE_KEY_FILE = 'keyfile';
    public const SOURCE_HOSTNAME = 'hostname';
    public const SOURCE_MACHTYPE = 'machtype';
    public const SOURCE_FSID = 'fsid';

    /** Host names that do not identify a machine. */
    private const PLACEHOLDER_HOSTNAMES = ['localhost', 'localhost.localdomain'];

    /** @var (callable(): (string|false))|null Test hook replacing gethostname() */
    private static $hostnameFunc = null;

    /** @var (callable(string): string)|null Test hook replacing fileSystemId() */
    private static $fileSystemIdFunc = null;

    /** @var (callable(): array<string, string>)|null Test hook replacing collect() */
    private static $v2SourcesFunc = null;

    /**
     * Replace host name and file system ID (tests only); null restores the
     * real functions.
     *
     * @param (callable(): (string|false))|null $hostname
     * @param (callable(string): string)|null $fileSystemId Returns the normalized fsid
     * @param (callable(): array<string, string>)|null $v2Sources Returns the v2 sources
     */
    public static function setTestHooks(?callable $hostname, ?callable $fileSystemId, ?callable $v2Sources = null): void
    {
        $any = $hostname !== null || $fileSystemId !== null || $v2Sources !== null;
        if ($any && ! defined('SCONFIG_TESTING')) {
            throw new \LogicException('MachineIdentity::setTestHooks() is only available in tests (SCONFIG_TESTING)');
        }
        self::$hostnameFunc = $hostname;
        self::$fileSystemIdFunc = $fileSystemId;
        self::$v2SourcesFunc = $v2Sources;
    }

    /** Firmware placeholder UUIDs that do not identify a machine. */
    private const PLACEHOLDER_UUIDS = [
        '00000000-0000-0000-0000-000000000000',
        'ffffffff-ffff-ffff-ffff-ffffffffffff',
        '03000200-0400-0500-0006-000700080009',
        '00020003-0004-0005-0006-000700080009',
    ];

    /**
     * Read all v2 sources of this system. Unreadable sources are left out.
     *
     * @return array<string, string> Source name => normalized value
     * @throws CommandTimeoutException If an external command hangs
     */
    public static function collect(): array
    {
        $sources = [];
        switch (PHP_OS_FAMILY) {
            case 'Windows':
                $sys = Command::windowsSystemDir();
                $out = Command::run([
                    $sys . '\\reg.exe', 'query', 'HKLM\\SOFTWARE\\Microsoft\\Cryptography',
                    '/v', 'MachineGuid', '/reg:64',
                ]);
                $value = self::normalizeUuid(self::parseRegQueryValue($out, 'MachineGuid'));
                if ($value !== '') {
                    $sources[self::SOURCE_MACHINE_GUID] = $value;
                }
                $out = Command::run([
                    $sys . '\\WindowsPowerShell\\v1.0\\powershell.exe', '-NoProfile', '-NonInteractive',
                    '-Command', '(Get-CimInstance -ClassName Win32_ComputerSystemProduct).UUID',
                ], Command::POWERSHELL_TIMEOUT);
                $value = self::normalizeUuid($out);
                if ($value !== '') {
                    $sources[self::SOURCE_SMBIOS_UUID] = $value;
                }

                break;
            case 'Linux':
                foreach (['/etc/machine-id', '/var/lib/dbus/machine-id'] as $path) {
                    $data = @file_get_contents($path);
                    $value = $data === false ? '' : self::normalizeMachineId($data);
                    if ($value !== '') {
                        $sources[self::SOURCE_MACHINE_ID] = $value;

                        break;
                    }
                }

                break;
            case 'Darwin':
                $out = Command::run(['/usr/sbin/ioreg', '-rd1', '-c', 'IOPlatformExpertDevice']);
                $value = self::normalizeUuid(self::parseIoregPlatformUuid($out));
                if ($value !== '') {
                    $sources[self::SOURCE_IOPLATFORM_UUID] = $value;
                }

                break;
            case 'BSD':
                $value = self::normalizeUuid(Command::run(['/sbin/sysctl', '-n', 'kern.hostuuid']));
                if ($value !== '') {
                    $sources[self::SOURCE_HOST_UUID] = $value;
                }

                break;
        }

        return $sources;
    }

    /**
     * Read the machine-wide v3 sources (v2 sources plus hostname and
     * machtype). fsid depends on the config file and is added by Crypto.
     *
     * @return array<string, string> Source name => normalized value
     * @throws CommandTimeoutException If an external command hangs
     */
    public static function collectV3(): array
    {
        $sources = self::$v2SourcesFunc !== null ? (self::$v2SourcesFunc)() : self::collect();
        $hostname = self::$hostnameFunc !== null ? (self::$hostnameFunc)() : gethostname();
        $value = $hostname === false ? '' : self::normalizeHostname($hostname);
        if ($value !== '') {
            $sources[self::SOURCE_HOSTNAME] = $value;
        }
        $value = self::readMachType();
        if ($value !== '') {
            $sources[self::SOURCE_MACHTYPE] = $value;
        }

        return $sources;
    }

    /**
     * Native machine architecture, normalized ('' if unknown).
     *
     * @throws CommandTimeoutException If an external command hangs
     */
    private static function readMachType(): string
    {
        if (PHP_OS_FAMILY === 'Windows') {
            $out = Command::run([
                Command::windowsSystemDir() . '\\reg.exe', 'query',
                'HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment',
                '/v', 'PROCESSOR_ARCHITECTURE', '/reg:64',
            ]);

            return self::normalizeMachType(self::parseRegQueryValue($out, 'PROCESSOR_ARCHITECTURE'));
        }
        if (PHP_OS_FAMILY === 'Darwin') {
            // hw.optional.arm64 is 1 on Apple silicon, also under Rosetta
            if (trim(Command::run(['/usr/sbin/sysctl', '-n', 'hw.optional.arm64'])) === '1') {
                return 'arm64';
            }
        }

        return self::normalizeMachType(php_uname('m'));
    }

    /**
     * Normalized ID of the file system that holds $path (or its nearest
     * existing parent directory), '' if it cannot be read.
     *
     * @throws CommandTimeoutException If an external command hangs
     */
    public static function fileSystemId(string $path): string
    {
        $path = self::existingPathFor($path);
        if (self::$fileSystemIdFunc !== null) {
            return (self::$fileSystemIdFunc)($path);
        }
        if (PHP_OS_FAMILY === 'Linux') {
            // statfs f_fsid; GNU coreutils and busybox print val[0]<<32 | val[1]
            foreach (['/usr/bin/stat', '/bin/stat'] as $tool) {
                if (is_file($tool)) {
                    return self::normalizeFsid(Command::run([$tool, '-f', '-c', '%i', '--', $path]));
                }
            }

            return '';
        }
        $stat = @stat($path);
        if ($stat === false) {
            return '';
        }
        $dev = (int) $stat['dev'];
        if (PHP_OS_FAMILY === 'Windows' || PHP_OS_FAMILY === 'Darwin') {
            // Windows: volume serial number (32 bit); macOS: dev_t is 32 bit
            $dev &= 0xFFFFFFFF;
        }

        return self::normalizeFsid(sprintf('%x', $dev));
    }

    /**
     * $path itself or its nearest existing parent directory.
     */
    private static function existingPathFor(string $path): string
    {
        $current = $path;
        while (! file_exists($current)) {
            $parent = dirname($current);
            if ($parent === $current) {
                break;
            }
            $current = $parent;
        }

        return $current;
    }

    /**
     * Derive the v2 AES-256 key from the given sources.
     *
     * @param array<string, string> $sources
     */
    public static function deriveKey(array $sources): string
    {
        return self::deriveKeyHkdf($sources, self::KDF_SALT, self::KDF_INFO);
    }

    /**
     * Derive the v3 AES-256 key from the given sources.
     *
     * @param array<string, string> $sources
     */
    public static function deriveKeyV3(array $sources): string
    {
        return self::deriveKeyHkdf($sources, self::KDF_SALT_V3, self::KDF_INFO_V3);
    }

    /**
     * HKDF-SHA256 over "<name>=<value>\n" of the sources sorted by name.
     *
     * @param array<string, string> $sources
     */
    private static function deriveKeyHkdf(array $sources, string $salt, string $info): string
    {
        if ($sources === []) {
            throw new \RuntimeException('no machine identity source available');
        }
        ksort($sources, SORT_STRING);
        $ikm = '';
        foreach ($sources as $name => $value) {
            $ikm .= $name . '=' . $value . "\n";
        }

        return hash_hkdf('sha256', $ikm, 32, $info, $salt);
    }

    /**
     * Lower-case A-Z only, independent of the locale (strtolower depends on
     * it before PHP 8.2); identical to the Go implementation.
     */
    private static function asciiLower(string $value): string
    {
        return strtr($value, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz');
    }

    /**
     * Trim and lower-case a host name; '' for empty, too long, non-printable
     * or placeholder names.
     */
    public static function normalizeHostname(string $value): string
    {
        $v = self::asciiLower(trim($value));
        if (
            $v === '' || strlen($v) > 255 || in_array($v, self::PLACEHOLDER_HOSTNAMES, true) ||
            preg_match('/^[\x21-\x7e]+$/', $v) !== 1
        ) {
            return '';
        }

        return $v;
    }

    /**
     * Map architecture names to one canonical name ('' if invalid/unknown).
     */
    public static function normalizeMachType(string $value): string
    {
        $v = self::asciiLower(trim($value));
        switch ($v) {
            case 'x86_64':
            case 'amd64':
            case 'x64':
            case 'em64t':
                return 'amd64';
            case 'aarch64':
            case 'arm64':
            case 'aarch64_be':
                return 'arm64';
            case 'i386':
            case 'i486':
            case 'i586':
            case 'i686':
            case 'x86':
            case 'i86pc':
                return '386';
            case 'unknown':
                return '';
        }
        if ($v === 'arm' || strncmp($v, 'armv', 4) === 0) {
            return 'arm';
        }

        return preg_match('/^[a-z0-9_]{1,32}$/', $v) === 1 ? $v : '';
    }

    /**
     * Validate a hex file system ID and remove leading zeros ('' for 0).
     */
    public static function normalizeFsid(string $value): string
    {
        $v = self::asciiLower(trim($value));
        if (preg_match('/^[0-9a-f]{1,16}$/', $v) !== 1) {
            return '';
        }

        return ltrim($v, '0');
    }

    /**
     * Trim, lower-case and remove braces; '' for malformed or placeholder UUIDs.
     */
    public static function normalizeUuid(string $value): string
    {
        $v = strtolower(trim($value));
        if (substr($v, 0, 1) === '{') {
            $v = substr($v, 1);
        }
        if (substr($v, -1) === '}') {
            $v = substr($v, 0, -1);
        }
        $pattern = '/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/';
        if (preg_match($pattern, $v) !== 1 || in_array($v, self::PLACEHOLDER_UUIDS, true)) {
            return '';
        }

        return $v;
    }

    /**
     * Validate a systemd/dbus machine ID (32 hex digits, not all zero).
     */
    public static function normalizeMachineId(string $value): string
    {
        $v = strtolower(trim($value));
        if (preg_match('/^[0-9a-f]{32}$/', $v) !== 1 || trim($v, '0') === '') {
            return '';
        }

        return $v;
    }

    /**
     * Extract the REG_SZ data of $valueName from "reg query" output.
     */
    public static function parseRegQueryValue(string $out, string $valueName): string
    {
        foreach (explode("\n", $out) as $line) {
            $fields = preg_split('/\s+/', trim($line));
            if (
                is_array($fields) && count($fields) >= 3 &&
                strcasecmp($fields[0], $valueName) === 0 && $fields[1] === 'REG_SZ'
            ) {
                return implode(' ', array_slice($fields, 2));
            }
        }

        return '';
    }

    /**
     * Extract IOPlatformUUID from "ioreg" output.
     */
    public static function parseIoregPlatformUuid(string $out): string
    {
        if (preg_match('/"IOPlatformUUID"\s*=\s*"([^"]+)"/', $out, $m) === 1) {
            return $m[1];
        }

        return '';
    }
}
