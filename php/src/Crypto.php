<?php

namespace Sconfig;

/**
 * Encryption and decryption of secure password values.
 *
 * New values use the v3 format "v3:<source names>:" + base64(iv | ct | tag)
 * with the key from MachineIdentity (including the fsid of the current .env
 * file, see setFile()) and the field name as additional authenticated data
 * (identical to the Go implementation). v2 values (same layout, prefix "v2:")
 * and legacy values (plain base64 of iv | tag | ct, frozen LegacyHardwareId
 * key) are decrypted and migrated to v3. Keys are derived lazily and cached.
 *
 * @package Sconfig
 * @internal
 */
final class Crypto
{
    public const V2_PREFIX = 'v2:';
    public const V3_PREFIX = 'v3:';
    private const METHOD = 'aes-256-gcm';
    private const IV_LENGTH = 12;
    private const TAG_LENGTH = 16;
    private const MAX_B64_LENGTH = 4096;
    private const AAD_PREFIX = 'sconfig:';
    private const MAX_SOURCE_NAMES = 8;
    private const MAX_NAMES_LENGTH = 256;
    private const KEY_FILE_MIN = 16;
    private const KEY_FILE_MAX = 4096;
    private const KEY_FILE_NEW = 32;

    private static ?int $hardwareIdOverride = null;
    private static ?string $keyFileValue = null;
    private static bool $rejectLegacy = false;
    private static ?string $legacyKey = null;
    private static ?\RuntimeException $legacyError = null;
    /** @var array<string, string>|null Machine-wide sources (without fsid and keyfile) */
    private static ?array $sources = null;
    private static ?\RuntimeException $sourcesError = null;
    /** @var string File system ID of the current .env file ('' = none) */
    private static string $fsid = '';
    /** @var array<int, string> All file system IDs seen (current first) */
    private static array $knownFsids = [];
    /** @var array<string, string> */
    private static array $keys = [];

    /**
     * Replace the machine identity by a fixed number (tests only): it is used
     * as legacy hardware ID and as the only v3 source "override".
     */
    public static function setHardwareIdOverride(?int $hardwareId): void
    {
        self::$hardwareIdOverride = $hardwareId;
        self::reset();
    }

    /** Refuse legacy v1 values instead of decrypting them (strict mode). */
    public static function setRejectLegacy(bool $reject): void
    {
        self::$rejectLegacy = $reject;
    }

    /**
     * Use a secret key file as additional source "keyfile" (null removes it).
     * A missing file is created with 32 random bytes and permissions 0600.
     *
     * @throws \RuntimeException If the file cannot be read or created
     */
    public static function setKeyFile(?string $path): void
    {
        $value = null;
        if ($path !== null) {
            try {
                $value = self::readOrCreateKeyFile($path);
            } catch (\RuntimeException $e) {
                throw new \RuntimeException(I18n::t('config.keyfile_invalid', $path, $e->getMessage()), 0, $e);
            }
        }
        self::$keyFileValue = $value;
        self::$sources = null;
        self::$sourcesError = null;
        self::$keys = [];
    }

    /**
     * Select the .env file whose file system ID is used as source "fsid" by
     * the following encrypt and decrypt calls.
     *
     * @throws \RuntimeException If an external command hangs
     */
    public static function setFile(string $path): void
    {
        self::$fsid = '';
        if (self::$hardwareIdOverride !== null) {
            return;
        }

        try {
            self::$fsid = MachineIdentity::fileSystemId($path);
        } catch (CommandTimeoutException $e) {
            throw new \RuntimeException(I18n::t('config.hardware_id_failed') . ': ' . $e->getMessage(), 0, $e);
        }
        if (self::$fsid !== '') {
            self::$knownFsids = array_values(array_diff(self::$knownFsids, [self::$fsid]));
            array_unshift(self::$knownFsids, self::$fsid);
        }
    }

    /** Forget all derived keys. */
    public static function reset(): void
    {
        self::$legacyKey = null;
        self::$legacyError = null;
        self::$sources = null;
        self::$sourcesError = null;
        self::$fsid = '';
        self::$knownFsids = [];
        self::$keys = [];
    }

    public static function isV2(string $value): bool
    {
        return strncmp($value, self::V2_PREFIX, strlen(self::V2_PREFIX)) === 0;
    }

    public static function isV3(string $value): bool
    {
        return strncmp($value, self::V3_PREFIX, strlen(self::V3_PREFIX)) === 0;
    }

    /** True for legacy v1 values (plain base64 without version prefix). */
    public static function isLegacy(string $value): bool
    {
        return ! self::isV2($value) && ! self::isV3($value);
    }

    /**
     * Additional authenticated data of a v2/v3 value: the password field
     * prefix in lower case with only a-z and 0-9 (Go "DatabaseSecurePassword"
     * and PHP "DATABASE_SECURE_PASSWORD" give the same value).
     */
    public static function fieldAad(string $field): string
    {
        return self::AAD_PREFIX . (string) preg_replace('/[^a-z0-9]/', '', strtolower($field));
    }

    /**
     * Encrypt for the password field $field with the v3 key of all currently
     * available sources.
     *
     * @throws \RuntimeException If no machine identity is available
     */
    public static function encrypt(string $plaintext, string $field): string
    {
        $sources = self::sources();
        if ($sources === []) {
            throw new \RuntimeException(I18n::t('config.hardware_id_failed'));
        }
        $names = array_keys($sources);
        sort($names, SORT_STRING);
        $key = self::keyFor('v3', $names, $sources);
        $iv = random_bytes(self::IV_LENGTH);
        $tag = '';
        $ciphertext = openssl_encrypt(
            $plaintext,
            self::METHOD,
            $key,
            OPENSSL_RAW_DATA,
            $iv,
            $tag,
            self::fieldAad($field),
            self::TAG_LENGTH
        );
        if ($ciphertext === false) {
            throw new \RuntimeException(I18n::t('config.failed_checking', 'encryption'));
        }

        return self::V3_PREFIX . implode(',', $names) . ':' . base64_encode($iv . $ciphertext . $tag);
    }

    /**
     * Decrypt a v3 or v2 value of the password field $field or a legacy
     * value. The value is validated and decoded once, before any key is
     * derived.
     *
     * @throws InvalidCiphertextException For structurally invalid values
     * @throws \RuntimeException If the key is unavailable or authentication fails
     */
    public static function decrypt(string $value, string $field): string
    {
        $version = self::isV3($value) ? 'v3' : (self::isV2($value) ? 'v2' : '');
        if ($version === '') {
            $data = self::decodeB64($value);
            if (self::$rejectLegacy) {
                throw new \RuntimeException(I18n::t('config.legacy_rejected'));
            }
            // legacy PHP layout: iv | tag | ciphertext, no AAD
            $plaintext = openssl_decrypt(
                (string) substr($data, self::IV_LENGTH + self::TAG_LENGTH),
                self::METHOD,
                self::legacyKey(),
                OPENSSL_RAW_DATA,
                substr($data, 0, self::IV_LENGTH),
                substr($data, self::IV_LENGTH, self::TAG_LENGTH)
            );
            if ($plaintext === false) {
                throw new \RuntimeException('decrypt: message authentication failed');
            }

            return $plaintext;
        }
        [$names, $b64] = self::parseSourceValue($value, $version . ':');
        $data = self::decodeB64($b64);
        $iv = substr($data, 0, self::IV_LENGTH);
        $tag = substr($data, -self::TAG_LENGTH);
        $ciphertext = (string) substr($data, self::IV_LENGTH, -self::TAG_LENGTH);
        $aad = self::fieldAad($field);
        foreach (self::candidateSources($names) as $sources) {
            $key = self::keyFor($version, $names, $sources);
            $plaintext = openssl_decrypt($ciphertext, self::METHOD, $key, OPENSSL_RAW_DATA, $iv, $tag, $aad);
            if ($plaintext !== false) {
                return $plaintext;
            }
        }
        $message = 'decrypt: message authentication failed';
        if ($version === 'v3') {
            $message .= ' (' . I18n::t('config.v3_key_mismatch') . ')';
        }

        throw new \RuntimeException($message);
    }

    /**
     * True if a value should be re-encrypted with all current sources: every
     * v2 value (migration to v3) and v3 values that use fewer sources than
     * are available now (e.g. a key file was added).
     */
    public static function needsUpgrade(string $value): bool
    {
        if (self::isV2($value)) {
            return true;
        }

        try {
            [$names] = self::parseSourceValue($value, self::V3_PREFIX);
            $sources = self::sources();
        } catch (\RuntimeException $e) {
            return false;
        }
        if (count($sources) <= count($names)) {
            return false;
        }
        foreach ($names as $name) {
            if (! isset($sources[$name])) {
                return false;
            }
        }

        return true;
    }

    /**
     * Split "<prefix><names>:<base64>"; names must be strictly ascending.
     *
     * @return array{0: array<int, string>, 1: string}
     */
    private static function parseSourceValue(string $value, string $prefix): array
    {
        $parts = explode(':', substr($value, strlen($prefix)), 2);
        if (count($parts) !== 2 || $parts[0] === '') {
            throw new InvalidCiphertextException('malformed ' . rtrim($prefix, ':') . ' value');
        }
        if (strlen($parts[0]) > self::MAX_NAMES_LENGTH) {
            throw new InvalidCiphertextException('source list too long');
        }
        $names = explode(',', $parts[0]);
        if (count($names) > self::MAX_SOURCE_NAMES) {
            throw new InvalidCiphertextException('too many sources');
        }
        foreach ($names as $i => $name) {
            if (preg_match('/^[a-z0-9]{1,32}$/', $name) !== 1) {
                throw new InvalidCiphertextException('invalid source name');
            }
            if ($i > 0 && strcmp($names[$i - 1], $name) >= 0) {
                throw new InvalidCiphertextException('source names not sorted or duplicated');
            }
        }

        return [$names, $parts[1]];
    }

    private static function decodeB64(string $text): string
    {
        if ($text === '') {
            throw new InvalidCiphertextException('empty encrypted password');
        }
        if (strlen($text) > self::MAX_B64_LENGTH) {
            throw new InvalidCiphertextException('encrypted password too long');
        }
        $data = base64_decode($text, true);
        if ($data === false) {
            throw new InvalidCiphertextException('invalid base64');
        }
        if (strlen($data) < self::IV_LENGTH + self::TAG_LENGTH) {
            throw new InvalidCiphertextException('ciphertext too short');
        }

        return $data;
    }

    private static function readOrCreateKeyFile(string $path): string
    {
        if (! file_exists($path)) {
            $data = random_bytes(self::KEY_FILE_NEW);
            $handle = @fopen($path, 'x');
            if ($handle === false) {
                throw new \RuntimeException('cannot create file');
            }
            @chmod($path, 0600);
            $written = fwrite($handle, $data);
            fclose($handle);
            if ($written !== strlen($data)) {
                @unlink($path);

                throw new \RuntimeException('cannot write file');
            }

            return bin2hex($data);
        }
        $data = @file_get_contents($path, false, null, 0, self::KEY_FILE_MAX + 1);
        if ($data === false) {
            throw new \RuntimeException('cannot read file');
        }
        if (strlen($data) < self::KEY_FILE_MIN || strlen($data) > self::KEY_FILE_MAX) {
            throw new \RuntimeException(sprintf(
                'size must be %d to %d bytes, got %d',
                self::KEY_FILE_MIN,
                self::KEY_FILE_MAX,
                strlen($data)
            ));
        }

        return bin2hex($data);
    }

    private static function legacyKey(): string
    {
        if (self::$legacyError !== null) {
            throw self::$legacyError;
        }
        if (self::$legacyKey === null) {
            try {
                $id = self::$hardwareIdOverride ?? LegacyHardwareId::compute();
                self::$legacyKey = LegacyHardwareId::deriveKey($id);
            } catch (\RuntimeException $e) {
                self::$legacyError = new \RuntimeException(
                    I18n::t('config.hardware_id_failed') . ': ' . $e->getMessage(),
                    0,
                    $e
                );

                throw self::$legacyError;
            }
        }

        return self::$legacyKey;
    }

    /**
     * All sources for new values: machine-wide sources (collected once), the
     * fsid of the current .env file and the optional key file.
     *
     * @return array<string, string>
     */
    private static function sources(): array
    {
        if (self::$sourcesError !== null) {
            throw self::$sourcesError;
        }
        if (self::$sources === null) {
            if (self::$hardwareIdOverride !== null) {
                self::$sources = [MachineIdentity::SOURCE_OVERRIDE => (string) self::$hardwareIdOverride];
            } else {
                try {
                    self::$sources = MachineIdentity::collectV3();
                } catch (\RuntimeException $e) {
                    self::$sourcesError = new \RuntimeException(
                        I18n::t('config.hardware_id_failed') . ': ' . $e->getMessage(),
                        0,
                        $e
                    );

                    throw self::$sourcesError;
                }
            }
        }
        $sources = self::$sources;
        if (self::$fsid !== '') {
            $sources[MachineIdentity::SOURCE_FSID] = self::$fsid;
        }
        if (self::$keyFileValue !== null) {
            $sources[MachineIdentity::SOURCE_KEY_FILE] = self::$keyFileValue;
        }

        return $sources;
    }

    /**
     * Source sets to try for a value: one set, or, if the value uses fsid,
     * one per known file system ID (current .env file first), so that values
     * of a file loaded before stay readable when another file is written.
     *
     * @param array<int, string> $names
     * @return array<int, array<string, string>>
     */
    private static function candidateSources(array $names): array
    {
        $sources = self::sources();
        if (! in_array(MachineIdentity::SOURCE_FSID, $names, true) || self::$knownFsids === []) {
            return [$sources];
        }
        $sets = [];
        foreach (self::$knownFsids as $fsid) {
            $sources[MachineIdentity::SOURCE_FSID] = $fsid;
            $sets[] = $sources;
        }

        return $sets;
    }

    /**
     * Key of format $version ('v2' or 'v3') for the source names, with the
     * values taken from $sources.
     *
     * @param array<int, string> $names
     * @param array<string, string> $sources
     */
    private static function keyFor(string $version, array $names, array $sources): string
    {
        $used = [];
        $cacheKey = $version;
        foreach ($names as $name) {
            if (! isset($sources[$name])) {
                throw new \RuntimeException(I18n::t('config.v2_source_missing', $name));
            }
            $used[$name] = $sources[$name];
            $cacheKey .= "\0" . $name . '=' . $sources[$name];
        }
        if (! isset(self::$keys[$cacheKey])) {
            self::$keys[$cacheKey] = $version === 'v2'
                ? MachineIdentity::deriveKey($used)
                : MachineIdentity::deriveKeyV3($used);
        }

        return self::$keys[$cacheKey];
    }
}
