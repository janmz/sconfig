<?php

namespace Sconfig;

/**
 * Legacy hardware ID and key of the PHP implementation ("v1"), FROZEN.
 *
 * Encrypted values without the "v2:" prefix were encrypted with this key. The
 * derivation must never change, otherwise existing .env files become
 * unreadable. It is only used to decrypt such legacy values; they are
 * re-encrypted with v2 on the next load(). It differs from the Go legacy
 * derivation and has only 2^32 distinct keys (mt_srand uses 32 bits).
 *
 * @package Sconfig
 * @internal
 */
final class LegacyHardwareId
{
    /**
     * Expand the legacy hardware ID into the 32-byte AES key.
     *
     * Intentional use of mt_rand: the same hardware ID must always produce the
     * same key. mt_srand() uses only the lower 32 bits of the seed.
     */
    public static function deriveKey(int $hardwareId): string
    {
        mt_srand($hardwareId);
        $key = '';
        for ($i = 0; $i < 32; $i++) {
            $val = mt_rand() ^ mt_rand();
            $key .= chr($val & 0xff);
        }
        mt_srand(); // do not leave the global generator in a predictable state

        return $key;
    }

    /**
     * Hash the identifiers in collection order: SHA-256, first 8 bytes read
     * big-endian as signed 64-bit integer.
     *
     * @param array<int, string> $identifiers
     */
    public static function fromIdentifiers(array $identifiers): int
    {
        $hash = hash('sha256', implode('|', $identifiers), true);
        $result = 0;
        for ($i = 0; $i < 8; $i++) {
            $result = ($result << 8) | ord($hash[$i]);
        }

        return (int) $result;
    }

    /**
     * Compute the legacy hardware ID of this machine.
     *
     * @throws CommandTimeoutException If an external command hangs
     */
    public static function compute(): int
    {
        return self::fromIdentifiers(self::collectIdentifiers());
    }

    /**
     * Absolute path of wmic.exe. Since 3.0.0 all Windows programs are started
     * with absolute paths and without cmd.exe (before: PATH and current
     * directory); the output and therefore the ID stay the same.
     */
    private static function wmic(): string
    {
        return Command::windowsTool('wbem\\WMIC.exe');
    }

    /**
     * Check whether the system runs on a virtual machine (legacy rules).
     */
    private static function isVirtualMachine(): bool
    {
        if (PHP_OS_FAMILY === 'Windows') {
            $output = Command::run([self::wmic(), 'computersystem', 'get', 'Manufacturer,Model', '/value']);
            if ($output) {
                $manufacturer = '';
                $model = '';
                foreach (explode("\n", $output) as $line) {
                    if (strpos($line, 'Manufacturer=') === 0) {
                        $manufacturer = strtolower(trim(substr($line, 13)));
                    } elseif (strpos($line, 'Model=') === 0) {
                        $model = strtolower(trim(substr($line, 6)));
                    }
                }
                $vmIndicators = [
                    'vmware', 'virtualbox', 'microsoft corporation', 'xen',
                    'parallels', 'qemu', 'kvm', 'innotek', 'bochs',
                ];
                foreach ($vmIndicators as $indicator) {
                    if (
                        strpos($manufacturer, $indicator) !== false ||
                        strpos($model, $indicator) !== false
                    ) {
                        return true;
                    }
                }
                // Hyper-V ("Virtual Machine" in the model)
                if (strpos($model, 'virtual') !== false) {
                    return true;
                }
            }

            return false;
        }

        if (PHP_OS_FAMILY !== 'Linux') {
            return false;
        }

        // Method 1: systemd-detect-virt
        $output = Command::run('systemd-detect-virt 2>/dev/null');
        if ($output) {
            $virt = trim($output);
            if ($virt !== 'none' && $virt !== '') {
                return true;
            }
        }

        // Method 2: DMI vendor/product
        $checks = [
            '/sys/class/dmi/id/sys_vendor',
            '/sys/class/dmi/id/product_name',
            '/sys/class/dmi/id/chassis_vendor',
        ];
        $vmIndicators = [
            'qemu', 'kvm', 'vmware', 'virtualbox', 'xen',
            'parallels', 'microsoft', 'bochs', 'bhyve',
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
     * Collect the legacy identifiers in collection order (not sorted).
     *
     * @return array<int, string>
     */
    private static function collectIdentifiers(): array
    {
        $identifiers = [];
        $isVM = self::isVirtualMachine();

        if (PHP_OS_FAMILY === 'Windows') {
            if ($isVM) {
                // 1. MachineGuid from the registry
                $output = Command::run([
                    Command::windowsTool('reg.exe'), 'query', 'HKLM\\SOFTWARE\\Microsoft\\Cryptography',
                    '/v', 'MachineGuid',
                ]);
                if ($output) {
                    foreach (explode("\n", $output) as $line) {
                        if (preg_match('/MachineGuid\s+REG_SZ\s+(.+)/i', $line, $matches)) {
                            $machineGuid = trim($matches[1]);
                            if (! empty($machineGuid)) {
                                $identifiers[] = $machineGuid;

                                break;
                            }
                        }
                    }
                }

                // 2. SMBIOS UUID
                $output = Command::run([self::wmic(), 'csproduct', 'get', 'UUID', '/value']);
                if ($output) {
                    foreach (explode("\n", trim($output)) as $line) {
                        if (strpos($line, 'UUID=') === 0) {
                            $uuid = trim(substr($line, 5));
                            if (! empty($uuid)) {
                                $identifiers[] = $uuid;

                                break;
                            }
                        }
                    }
                }
            }

            // MAC of the first adapter listed by getmac
            $output = Command::run([Command::windowsTool('getmac.exe'), '/fo', 'csv', '/nh']);
            if ($output) {
                $lines = explode("\n", trim($output));
                if (! empty($lines)) {
                    $parts = str_getcsv($lines[0]);
                    if (! empty($parts[0])) {
                        $identifiers[] = $parts[0];
                    }
                }
            }

            // Baseboard, disk and (physical machines only) CPU
            $commands = [
                ['baseboard', 'get', 'SerialNumber', '/value'],
                ['baseboard', 'get', 'Product', '/value'],
                ['diskdrive', 'get', 'SerialNumber', '/value'],
            ];
            if (! $isVM) {
                $commands[] = ['cpu', 'get', 'ProcessorId', '/value'];
            }
            foreach ($commands as $args) {
                $output = Command::run(array_merge([self::wmic()], $args));
                if ($output) {
                    foreach (explode("\n", trim($output)) as $line) {
                        if (strpos($line, '=') !== false) {
                            [, $value] = explode('=', $line, 2);
                            $value = trim($value);
                            if (! empty($value)) {
                                $identifiers[] = $value;

                                break;
                            }
                        }
                    }
                }
            }
        } else {
            if ($isVM) {
                $machineId = @file_get_contents('/etc/machine-id');
                if ($machineId && trim($machineId) !== '') {
                    $identifiers[] = trim($machineId);
                }
                $productUuid = @file_get_contents('/sys/class/dmi/id/product_uuid');
                if ($productUuid && trim($productUuid) !== '') {
                    $identifiers[] = trim($productUuid);
                }
            }

            $commands = [
                "cat /sys/class/net/*/address 2>/dev/null | head -1",
                "cat /sys/class/dmi/id/board_serial 2>/dev/null",
            ];
            if (! $isVM) {
                $commands[] = "cat /proc/cpuinfo | grep 'Serial' | head -1";
            }
            foreach ($commands as $cmd) {
                $output = Command::run($cmd);
                if ($output) {
                    $value = trim($output);
                    if (! empty($value)) {
                        $identifiers[] = $value;
                    }
                }
            }
        }

        if (empty($identifiers)) {
            // Fallback (no hardware): host name and PHP version
            $identifiers[] = gethostname();
            $identifiers[] = PHP_VERSION;
        }

        return $identifiers;
    }
}
