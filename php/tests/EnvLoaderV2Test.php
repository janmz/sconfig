<?php

declare(strict_types=1);

namespace Sconfig\Tests;

use PHPUnit\Framework\TestCase;
use Sconfig\Command;
use Sconfig\CommandTimeoutException;
use Sconfig\Crypto;
use Sconfig\EnvFile;
use Sconfig\EnvLoader;
use Sconfig\I18n;
use Sconfig\LegacyHardwareId;
use Sconfig\MachineIdentity;

/**
 * Tests for the v2 key format, the frozen legacy derivation (Known-Answer-
 * Tests shared with sconfig_v2_test.go), the migration and the file handling.
 */
class EnvLoaderV2Test extends TestCase
{
    /** "legacy-secret" encrypted by version 2.0.8.62 with hardware ID 12345. */
    private const LEGACY_CIPHER_12345 = 's1jcBn4T02Q2ca+wFYas+WChesXv7Jn5zNPEMKGVOnllPo6v7iFBuuk=';

    /** "shared-secret" encrypted by the Go implementation (override 12345). */
    private const GO_VALUE = 'v2:override:qJKuOJyvyz+FRSiYEkaBm/YvCyRRdidnh5Ew0JYdYnKIUYnlOXnPDx4=';

    private string $tempDir;

    protected function setUp(): void
    {
        parent::setUp();
        EnvLoader::clear();
        EnvLoader::setHardwareIdForTest(12345);
        $this->tempDir = sys_get_temp_dir() . '/sconfig_php_v2_' . uniqid('', true);
        mkdir($this->tempDir, 0700, true);
        EnvLoader::setExecutableRootForTest($this->tempDir);
        EnvLoader::setHomeDirForTest($this->tempDir . DIRECTORY_SEPARATOR . 'home');
    }

    protected function tearDown(): void
    {
        if (is_dir($this->tempDir)) {
            @chmod($this->tempDir, 0700);
            foreach (glob($this->tempDir . '/{,.}*', GLOB_BRACE) ?: [] as $file) {
                if (is_file($file)) {
                    @chmod($file, 0600);
                    unlink($file);
                }
            }
            rmdir($this->tempDir);
        }
        EnvLoader::setExecutableRootForTest(null);
        EnvLoader::setHomeDirForTest(null);
        EnvLoader::setHardwareIdForTest(null);
        EnvLoader::clear();
        parent::tearDown();
    }

    private function envPath(string $name = 'test.env'): string
    {
        return $this->tempDir . '/' . $name;
    }

    public function testLegacyKnownAnswers(): void
    {
        $id = LegacyHardwareId::fromIdentifiers(['00-11-22-33-44-55', 'SERIAL123']);
        self::assertSame(5786165445974681484, $id);
        self::assertSame(
            '8f0cd49f89917efd393f53ee1ef4d8d389fa302cf478ccbf3f1deff350d64c02',
            bin2hex(LegacyHardwareId::deriveKey($id))
        );
        self::assertSame(
            '03ce0640a38e83da19c0d1e16341d0660507d11db9d7bf7c7c6211f78b9d73b4',
            bin2hex(LegacyHardwareId::deriveKey(12345))
        );
    }

    public function testV2KnownAnswersSharedWithGo(): void
    {
        self::assertSame(
            'e28160dbd3a0ec53399e2739e768f142d998295e82e71d4ae8b6127d8c1e651c',
            bin2hex(MachineIdentity::deriveKey([
                'smbiosuuid' => '4c4c4544-0042-3510-8051-b2c04f4a3732',
                'machineguid' => '1b4e28ba-2fa1-11d2-883f-0016d3cca427',
            ]))
        );
        self::assertSame(
            '81148b67e0975a57c71c270d555f4aa1d60e22a572b984e119f7eefa81e6000f',
            bin2hex(MachineIdentity::deriveKey(['override' => '12345']))
        );
    }

    public function testDecryptsGoValue(): void
    {
        self::assertSame('shared-secret', Crypto::decrypt(self::GO_VALUE, 'SHARED'));
    }

    public function testLegacyValueIsMigratedToV2(): void
    {
        $path = $this->envPath();
        $marker = I18n::builtinText('config.password_message', 'en');
        file_put_contents($path, "DB_PASSWORD=\"{$marker}\"\nDB_SECURE_PASSWORD=" . self::LEGACY_CIPHER_12345 . "\n");
        EnvLoader::load($path);
        self::assertSame('legacy-secret', EnvLoader::get('DB_PASSWORD'));
        $content = (string) file_get_contents($path);
        self::assertStringContainsString('DB_SECURE_PASSWORD=v2:override:', $content);

        EnvLoader::clear();
        EnvLoader::load($path);
        self::assertSame('legacy-secret', EnvLoader::get('DB_PASSWORD'));
    }

    public function testNewValuesUseV2(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "DB_PASSWORD=secret\n");
        EnvLoader::load($path);
        $content = (string) file_get_contents($path);
        self::assertStringContainsString('DB_SECURE_PASSWORD=v2:override:', $content);
        self::assertStringNotContainsString('secret' . "\n", $content);
        self::assertSame('secret', EnvLoader::get('DB_PASSWORD'));
    }

    public function testMissingSourceIsReported(): void
    {
        $path = $this->envPath();
        $marker = I18n::builtinText('config.password_message', 'en');
        $secure = 'v2:machineid:' . self::LEGACY_CIPHER_12345;
        file_put_contents($path, "DB_PASSWORD=\"{$marker}\"\nDB_SECURE_PASSWORD={$secure}\n");
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessageMatches('/machineid/');
        EnvLoader::load($path);
    }

    public function testWrongKeyGivesRuntimeExceptionNotError(): void
    {
        $path = $this->envPath();
        $marker = I18n::builtinText('config.password_message', 'en');
        file_put_contents($path, "DB_PASSWORD=\"{$marker}\"\nDB_SECURE_PASSWORD=" . self::GO_VALUE . "\n");
        EnvLoader::setHardwareIdForTest(99999);

        try {
            EnvLoader::load($path);
            self::fail('expected RuntimeException');
        } catch (\RuntimeException $e) {
            self::assertStringContainsString('DB', $e->getMessage());
            self::assertStringContainsString('authentication failed', $e->getMessage());
        }
    }

    public function testInvalidValueIsReported(): void
    {
        $path = $this->envPath();
        $marker = I18n::builtinText('config.password_message', 'en');
        file_put_contents($path, "DB_PASSWORD=\"{$marker}\"\nDB_SECURE_PASSWORD=not-base64!!!\n");
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessageMatches('/invalid base64/');
        EnvLoader::load($path);
    }

    public function testUpdateEnvDoesNotWriteProcessEnvironment(): void
    {
        putenv('SCONFIG_LEAK_TEST=from-env');

        try {
            $path = $this->envPath();
            file_put_contents($path, "THEME=dark\n");
            EnvLoader::load($path);
            self::assertSame('from-env', EnvLoader::get('SCONFIG_LEAK_TEST'));
            EnvLoader::set('THEME', 'light');
            EnvLoader::updateEnv($path);
            $content = (string) file_get_contents($path);
            self::assertStringContainsString('THEME=light', $content);
            self::assertStringNotContainsString('SCONFIG_LEAK_TEST', $content);
        } finally {
            putenv('SCONFIG_LEAK_TEST');
        }
    }

    public function testFormatIsPreserved(): void
    {
        $path = $this->envPath();
        $content = "# header\n\nAPP_NAME=\"My App\"\nexport PATH_X=/opt/x\nRAW LINE WITHOUT EQUALS\n"
            . "SINGLE='a b'\nDB_PASSWORD=plain\n";
        file_put_contents($path, $content);
        EnvLoader::load($path);
        $written = (string) file_get_contents($path);
        self::assertStringContainsString("# header\n\nAPP_NAME=\"My App\"\n", $written);
        self::assertStringContainsString("export PATH_X=/opt/x\n", $written);
        self::assertStringContainsString("RAW LINE WITHOUT EQUALS\n", $written);
        self::assertStringContainsString("SINGLE='a b'\n", $written);
        $marker = I18n::builtinText('config.password_message');
        self::assertStringContainsString('DB_PASSWORD="' . $marker . '"', $written);
        self::assertSame('My App', EnvLoader::get('APP_NAME'));
        self::assertSame('/opt/x', EnvLoader::get('PATH_X'));

        // A second load reads its own output unchanged
        EnvLoader::clear();
        EnvLoader::load($path);
        self::assertSame('plain', EnvLoader::get('DB_PASSWORD'));
        self::assertSame($written, file_get_contents($path));
    }

    public function testWriteFailureThrows(): void
    {
        if (function_exists('posix_geteuid') && posix_geteuid() === 0) {
            self::markTestSkipped('root ignores file permissions');
        }
        $path = $this->envPath();
        file_put_contents($path, "DB_PASSWORD=plain\n");
        if (PHP_OS_FAMILY === 'Windows') {
            chmod($path, 0444); // read-only attribute: rename over it fails
        } else {
            chmod($this->tempDir, 0555); // no temporary file in the directory
        }

        try {
            $this->expectException(\RuntimeException::class);
            EnvLoader::load($path);
        } finally {
            chmod($this->tempDir, 0700);
            chmod($path, 0600);
        }
    }

    public function testNewFileIsPrivateAndNoTempFilesRemain(): void
    {
        $path = $this->envPath('new.env');
        EnvLoader::load($path);
        self::assertFileExists($path);
        if (PHP_OS_FAMILY !== 'Windows') {
            self::assertSame(0600, fileperms($path) & 0777);
        }
        foreach (scandir($this->tempDir) ?: [] as $name) {
            self::assertStringNotContainsString('.sconfig', $name);
        }
    }

    public function testEnvFileFormatValue(): void
    {
        self::assertSame('abc', EnvFile::formatValue('abc', ''));
        self::assertSame('"a b"', EnvFile::formatValue('a b', ''));
        self::assertSame("'say \"hi\"'", EnvFile::formatValue('say "hi"', ''));
        self::assertSame("'x'", EnvFile::formatValue('x', "'"));
    }

    public function testTranslateWithMissingArgumentsDoesNotThrow(): void
    {
        $text = I18n::t('config.decrypt_failed', 'ONLY_ONE');
        self::assertStringContainsString('ONLY_ONE', $text);
    }

    public function testSourceNormalization(): void
    {
        self::assertSame(
            '1b4e28ba-2fa1-11d2-883f-0016d3cca427',
            MachineIdentity::normalizeUuid(" {1B4E28BA-2FA1-11D2-883F-0016D3CCA427}\r\n")
        );
        self::assertSame('', MachineIdentity::normalizeUuid('00000000-0000-0000-0000-000000000000'));
        self::assertSame('', MachineIdentity::normalizeUuid('03000200-0400-0500-0006-000700080009'));
        self::assertSame('', MachineIdentity::normalizeUuid('not-a-uuid'));
        self::assertSame(
            '0123456789abcdef0123456789abcdef',
            MachineIdentity::normalizeMachineId("0123456789ABCDEF0123456789abcdef\n")
        );
        self::assertSame('', MachineIdentity::normalizeMachineId(str_repeat('0', 32)));
        $reg = "\r\nHKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Cryptography\r\n"
            . "    MachineGuid    REG_SZ    1b4e28ba-2fa1-11d2-883f-0016d3cca427\r\n\r\n";
        self::assertSame(
            '1b4e28ba-2fa1-11d2-883f-0016d3cca427',
            MachineIdentity::parseRegQueryValue($reg, 'MachineGuid')
        );
        $ioreg = '  | |   "IOPlatformUUID" = "1B4E28BA-2FA1-11D2-883F-0016D3CCA427"';
        self::assertSame('1B4E28BA-2FA1-11D2-883F-0016D3CCA427', MachineIdentity::parseIoregPlatformUuid($ioreg));
    }

    public function testCollectSourcesHasValidNames(): void
    {
        foreach (MachineIdentity::collect() as $name => $value) {
            self::assertMatchesRegularExpression('/^[a-z0-9]{1,32}$/', $name);
            self::assertNotSame('', $value);
        }
    }

    public function testCommandTimeout(): void
    {
        $command = PHP_OS_FAMILY === 'Windows' ? ['ping', '-n', '6', '127.0.0.1'] : ['sleep', '5'];
        $this->expectException(CommandTimeoutException::class);
        Command::run($command, 0.3);
    }
}
