<?php

namespace Sconfig;

/**
 * Machine identity "v2" – used for all newly encrypted values.
 *
 * Same specification as the Go implementation (hwid_v2.go), verified by shared
 * test vectors:
 *   Sources: Windows machineguid (registry, 64-bit view) and smbiosuuid
 *   (PowerShell CIM), Linux machineid (/etc/machine-id), macOS ioplatformuuid
 *   (ioreg), FreeBSD hostuuid (sysctl kern.hostuuid).
 *   Key: HKDF-SHA256 over "<name>=<value>\n" of the used sources sorted by
 *   name, salt "sconfig-v2", info "sconfig v2 aes-256-gcm", 32 bytes.
 *
 * @package Sconfig
 * @internal
 */
final class MachineIdentity
{
    public const KDF_SALT = 'sconfig-v2';
    public const KDF_INFO = 'sconfig v2 aes-256-gcm';

    public const SOURCE_MACHINE_GUID = 'machineguid';
    public const SOURCE_SMBIOS_UUID = 'smbiosuuid';
    public const SOURCE_MACHINE_ID = 'machineid';
    public const SOURCE_IOPLATFORM_UUID = 'ioplatformuuid';
    public const SOURCE_HOST_UUID = 'hostuuid';
    public const SOURCE_OVERRIDE = 'override';
    public const SOURCE_KEY_FILE = 'keyfile';

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
     * Derive the AES-256 key from the given sources.
     *
     * @param array<string, string> $sources
     */
    public static function deriveKey(array $sources): string
    {
        if ($sources === []) {
            throw new \RuntimeException('no machine identity source available');
        }
        ksort($sources, SORT_STRING);
        $ikm = '';
        foreach ($sources as $name => $value) {
            $ikm .= $name . '=' . $value . "\n";
        }

        return hash_hkdf('sha256', $ikm, 32, self::KDF_INFO, self::KDF_SALT);
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
