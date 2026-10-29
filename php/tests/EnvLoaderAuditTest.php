<?php

declare(strict_types=1);

namespace Sconfig\Tests;

use PHPUnit\Framework\TestCase;
use Sconfig\Crypto;
use Sconfig\EnvFile;
use Sconfig\EnvLoader;
use Sconfig\I18n;

/**
 * Tests for the fixes of the security audit 2026-10-27 (SA-1 ... SA-16).
 */
class EnvLoaderAuditTest extends TestCase
{
    /** "legacy-secret" encrypted by version 2.0.8.62 with hardware ID 12345. */
    private const LEGACY_CIPHER_12345 = 's1jcBn4T02Q2ca+wFYas+WChesXv7Jn5zNPEMKGVOnllPo6v7iFBuuk=';

    private string $tempDir;

    /** @var array<int, string> */
    private array $warnings = [];

    protected function setUp(): void
    {
        parent::setUp();
        EnvLoader::clear();
        EnvLoader::setHardwareIdForTest(12345);
        $this->warnings = [];
        EnvLoader::setWarningHandler(function (string $message): void {
            $this->warnings[] = $message;
        });
        $this->tempDir = sys_get_temp_dir() . '/sconfig_php_audit_' . uniqid('', true);
        mkdir($this->tempDir, 0700, true);
        EnvLoader::setExecutableRootForTest($this->tempDir);
        EnvLoader::setHomeDirForTest($this->tempDir . DIRECTORY_SEPARATOR . 'home');
    }

    protected function tearDown(): void
    {
        EnvLoader::setKeyFile(null);
        EnvLoader::setRejectLegacy(false);
        EnvLoader::setExportPasswords(true);
        EnvLoader::setWarningHandler(null);
        unset($_SERVER['DOCUMENT_ROOT']);
        foreach (glob($this->tempDir . '/{,.}*', GLOB_BRACE) ?: [] as $file) {
            if (is_file($file)) {
                unlink($file);
            }
        }
        rmdir($this->tempDir);
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

    private function marker(): string
    {
        return I18n::builtinText('config.password_message', 'en');
    }

    public function testSetRejectsLineInjection(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "DB_HOST=real.example\n");
        EnvLoader::load($path);
        foreach (["light\nDB_HOST=evil.example", "a\rb", "a\0b", "a\x1bb"] as $value) {
            try {
                EnvLoader::set('THEME', $value);
                self::fail('value with control character accepted');
            } catch (\InvalidArgumentException $e) {
                self::assertStringContainsString('THEME', $e->getMessage());
            }
        }
        foreach (['', '1ABC', 'A B', "A\nB", 'A=B'] as $key) {
            try {
                EnvLoader::set($key, 'x');
                self::fail('invalid key accepted: ' . $key);
            } catch (\InvalidArgumentException $e) {
                self::assertNotSame('', $e->getMessage());
            }
        }
        EnvLoader::set('THEME', "tab\tis ok");
        EnvLoader::updateEnv($path);
        EnvLoader::clear();
        EnvLoader::load($path);
        self::assertSame('real.example', EnvLoader::get('DB_HOST'));
        self::assertSame("tab\tis ok", EnvLoader::get('THEME'));
    }

    public function testRenderRefusesLineBreaks(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        EnvFile::render([], ['KEY' => "a\nINJECTED=1"]);
    }

    public function testPasswordsNotExportedWhenDisabled(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "NOEXPORT_PASSWORD=hidden\nNOEXPORT_HOST=visible\n");
        EnvLoader::setExportPasswords(false);
        EnvLoader::load($path);
        self::assertSame('hidden', EnvLoader::get('NOEXPORT_PASSWORD'));
        self::assertFalse(getenv('NOEXPORT_PASSWORD'));
        self::assertArrayNotHasKey('NOEXPORT_PASSWORD', $_ENV);
        self::assertSame('visible', getenv('NOEXPORT_HOST'));
        putenv('NOEXPORT_HOST');
        unset($_ENV['NOEXPORT_HOST'], $_ENV['NOEXPORT_SECURE_PASSWORD']);
        putenv('NOEXPORT_SECURE_PASSWORD');
    }

    public function testUpdateEnvWritesOnlyKeysOfItsOwnFile(): void
    {
        $a = $this->envPath('a.env');
        $b = $this->envPath('b.env');
        file_put_contents($a, "ONLY_A=1\n");
        file_put_contents($b, "ONLY_B=2\n");
        EnvLoader::load($a);
        EnvLoader::load($b);
        EnvLoader::set('SHARED_SET', 'x');
        EnvLoader::updateEnv($a);
        $content = (string) file_get_contents($a);
        self::assertStringContainsString('ONLY_A=1', $content);
        self::assertStringContainsString('SHARED_SET=x', $content);
        self::assertStringNotContainsString('ONLY_B', $content);
    }

    public function testWarningForDocumentRoot(): void
    {
        $_SERVER['DOCUMENT_ROOT'] = $this->tempDir;
        EnvLoader::load($this->envPath());
        self::assertCount(1, $this->warnings);
        self::assertStringContainsString('test.env', $this->warnings[0]);
    }

    public function testMigrationWarningAndStrictMode(): void
    {
        $path = $this->envPath();
        $content = "DB_PASSWORD=\"{$this->marker()}\"\nDB_SECURE_PASSWORD=" . self::LEGACY_CIPHER_12345 . "\n";
        file_put_contents($path, $content);
        EnvLoader::setRejectLegacy(true);

        try {
            EnvLoader::load($path);
            self::fail('strict mode must refuse legacy values');
        } catch (\RuntimeException $e) {
            self::assertStringContainsString('DB', $e->getMessage());
        }
        EnvLoader::setRejectLegacy(false);
        EnvLoader::clear();
        EnvLoader::load($path);
        self::assertSame('legacy-secret', EnvLoader::get('DB_PASSWORD'));
        self::assertCount(1, $this->warnings);
        self::assertStringContainsString('1', $this->warnings[0]);
    }

    public function testKeyFileAndUpgrade(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "DB_PASSWORD=with-keyfile\n");
        EnvLoader::load($path);
        $keyFile = $this->envPath('sconfig.key');
        EnvLoader::setKeyFile($keyFile);
        self::assertSame(32, filesize($keyFile));
        if (PHP_OS_FAMILY !== 'Windows') {
            self::assertSame(0600, fileperms($keyFile) & 0777);
        }
        EnvLoader::clear();
        EnvLoader::load($path);
        self::assertStringContainsString('DB_SECURE_PASSWORD=v2:keyfile,override:', (string) file_get_contents($path));
        self::assertSame('with-keyfile', EnvLoader::get('DB_PASSWORD'));

        EnvLoader::setKeyFile(null);
        EnvLoader::clear();

        try {
            EnvLoader::load($path);
            self::fail('value with key file must not decrypt without it');
        } catch (\RuntimeException $e) {
            self::assertStringContainsString('keyfile', $e->getMessage());
        }
    }

    public function testFieldBinding(): void
    {
        $value = Crypto::encrypt('bound', 'DATABASE');
        self::assertSame('bound', Crypto::decrypt($value, 'Database'));
        $this->expectException(\RuntimeException::class);
        Crypto::decrypt($value, 'API');
    }

    public function testSourceListMustBeCanonical(): void
    {
        foreach (
            [
                'v2:smbiosuuid,machineguid:' . self::LEGACY_CIPHER_12345,
                'v2:a,a:' . self::LEGACY_CIPHER_12345,
                'v2:a,b,c,d,e,f,g,h,i:' . self::LEGACY_CIPHER_12345,
            ] as $value
        ) {
            try {
                Crypto::decrypt($value, 'DB');
                self::fail('non-canonical source list accepted: ' . $value);
            } catch (\Sconfig\InvalidCiphertextException $e) {
                self::assertNotSame('', $e->getMessage());
            }
        }
    }

    public function testFileTooLarge(): void
    {
        $path = $this->envPath('big.env');
        file_put_contents($path, str_repeat('A', EnvFile::MAX_FILE_SIZE + 1));
        $this->expectException(\RuntimeException::class);
        EnvFile::readLines($path);
    }

    public function testWarningForReadableFile(): void
    {
        if (PHP_OS_FAMILY === 'Windows') {
            self::markTestSkipped('Unix permissions');
        }
        $path = $this->envPath();
        file_put_contents($path, "DB_PASSWORD=x\n");
        chmod($path, 0644);
        EnvLoader::load($path);
        self::assertNotEmpty($this->warnings);
        self::assertSame(0644, fileperms($path) & 0777);
    }
}
