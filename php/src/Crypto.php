<?php

namespace Sconfig;

/**
 * Encryption and decryption of secure password values.
 *
 * New values use the v2 format "v2:<source names>:" + base64(iv | ct | tag)
 * with the key from MachineIdentity and the field name as additional
 * authenticated data (identical to the Go implementation). Legacy values
 * (plain base64 of iv | tag | ct) are decrypted with the frozen
 * LegacyHardwareId key. Keys are derived lazily and cached.
 *
 * @package Sconfig
 * @internal
 */
final class Crypto
{
    public const V2_PREFIX = 'v2:';
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
    /** @var array<string, string>|null */
    private static ?array $sources = null;
    private static ?\RuntimeException $sourcesError = null;
    /** @var array<string, string> */
    private static array $v2Keys = [];

    /**
     * Replace the machine identity by a fixed number (tests only): it is used
     * as legacy hardware ID and as v2 source "override".
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
        self::$v2Keys = [];
    }

    /** Forget all derived keys. */
    public static function reset(): void
    {
        self::$legacyKey = null;
        self::$legacyError = null;
        self::$sources = null;
        self::$sourcesError = null;
        self::$v2Keys = [];
    }

    public static function isV2(string $value): bool
    {
        return strncmp($value, self::V2_PREFIX, strlen(self::V2_PREFIX)) === 0;
    }

    /**
     * Additional authenticated data of a v2 value: the password field prefix
     * in lower case with only a-z and 0-9 (Go "DatabaseSecurePassword" and
     * PHP "DATABASE_SECURE_PASSWORD" give the same value).
     */
    public static function fieldAad(string $field): string
    {
        return self::AAD_PREFIX . (string) preg_replace('/[^a-z0-9]/', '', strtolower($field));
    }

    /**
     * Encrypt for the password field $field with the v2 key of all currently
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
        $key = self::keyFor($names);
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

        return self::V2_PREFIX . implode(',', $names) . ':' . base64_encode($iv . $ciphertext . $tag);
    }

    /**
     * Decrypt a v2 value of the password field $field or a legacy value. The
     * value is validated and decoded once, before any key is derived.
     *
     * @throws InvalidCiphertextException For structurally invalid values
     * @throws \RuntimeException If the key is unavailable or authentication fails
     */
    public static function decrypt(string $value, string $field): string
    {
        if (self::isV2($value)) {
            [$names, $b64] = self::parseV2($value);
            $data = self::decodeB64($b64);
            $key = self::keyFor($names);
            $iv = substr($data, 0, self::IV_LENGTH);
            $tag = substr($data, -self::TAG_LENGTH);
            $ciphertext = substr($data, self::IV_LENGTH, -self::TAG_LENGTH);
            $aad = self::fieldAad($field);
        } else {
            $data = self::decodeB64($value);
            if (self::$rejectLegacy) {
                throw new \RuntimeException(I18n::t('config.legacy_rejected'));
            }
            $key = self::legacyKey();
            // legacy PHP layout: iv | tag | ciphertext, no AAD
            $iv = substr($data, 0, self::IV_LENGTH);
            $tag = substr($data, self::IV_LENGTH, self::TAG_LENGTH);
            $ciphertext = substr($data, self::IV_LENGTH + self::TAG_LENGTH);
            $aad = '';
        }
        $plaintext = openssl_decrypt((string) $ciphertext, self::METHOD, $key, OPENSSL_RAW_DATA, $iv, $tag, $aad);
        if ($plaintext === false) {
            throw new \RuntimeException('decrypt: message authentication failed');
        }

        return $plaintext;
    }

    /**
     * True if a v2 value uses fewer sources than are available now (e.g. a
     * key file was added) and should be re-encrypted.
     */
    public static function needsUpgrade(string $value): bool
    {
        try {
            [$names] = self::parseV2($value);
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
     * Split "v2:<names>:<base64>"; names must be strictly ascending.
     *
     * @return array{0: array<int, string>, 1: string}
     */
    private static function parseV2(string $value): array
    {
        $parts = explode(':', substr($value, strlen(self::V2_PREFIX)), 2);
        if (count($parts) !== 2 || $parts[0] === '') {
            throw new InvalidCiphertextException('malformed v2 value');
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
     * @return array<string, string>
     */
    private static function sources(): array
    {
        if (self::$sourcesError !== null) {
            throw self::$sourcesError;
        }
        if (self::$sources === null) {
            if (self::$hardwareIdOverride !== null) {
                $sources = [MachineIdentity::SOURCE_OVERRIDE => (string) self::$hardwareIdOverride];
            } else {
                try {
                    $sources = MachineIdentity::collect();
                } catch (\RuntimeException $e) {
                    self::$sourcesError = new \RuntimeException(
                        I18n::t('config.hardware_id_failed') . ': ' . $e->getMessage(),
                        0,
                        $e
                    );

                    throw self::$sourcesError;
                }
            }
            if (self::$keyFileValue !== null) {
                $sources[MachineIdentity::SOURCE_KEY_FILE] = self::$keyFileValue;
            }
            self::$sources = $sources;
        }

        return self::$sources;
    }

    /**
     * @param array<int, string> $names
     */
    private static function keyFor(array $names): string
    {
        $cacheKey = implode(',', $names);
        if (isset(self::$v2Keys[$cacheKey])) {
            return self::$v2Keys[$cacheKey];
        }
        $sources = self::sources();
        $used = [];
        foreach ($names as $name) {
            if (! isset($sources[$name])) {
                throw new \RuntimeException(I18n::t('config.v2_source_missing', $name));
            }
            $used[$name] = $sources[$name];
        }
        self::$v2Keys[$cacheKey] = MachineIdentity::deriveKey($used);

        return self::$v2Keys[$cacheKey];
    }
}
