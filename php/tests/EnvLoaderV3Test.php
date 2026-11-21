<?php

declare(strict_types=1);

namespace Sconfig\Tests;

use PHPUnit\Framework\TestCase;
use Sconfig\Crypto;
use Sconfig\EnvLoader;
use Sconfig\I18n;
use Sconfig\MachineIdentity;

/**
 * Tests for the v3 key format (Known-Answer-Tests shared with
 * sconfig_v3_test.go), the migration of v2 values and the binding to host
 * name and file system.
 */
class EnvLoaderV3Test extends TestCase
{
    /** "shared-secret" for field "shared", written by Go (override 12345). */
    private const GO_V3_VALUE = 'v3:override:Fw37IF2vJefENHXxr+lHOvMi9ORfmR0Ju8QpGM4D0xuFEzVkZghTGQw=';

    /** "v2-secret" for field "Database", v2 format of version 3.0.0 (override 12345). */
    private const V2_VALUE_DATABASE = 'v2:override:cYuA/Ekli4qbh33oldf91p83HqsNJTolW+rBI2FT2b1JsCfoyw==';

    private string $tempDir;

    /** @var array<int, string> */
    private array $warnings = [];

    protected function setUp(): void
    {
        parent::setUp();
        EnvLoader::clear();
        EnvLoader::setHardwareIdForTest(12345);
        $this->tempDir = sys_get_temp_dir() . '/sconfig_php_v3_' . uniqid('', true);
        mkdir($this->tempDir, 0700, true);
        // real spelling of the path (case on Windows), as EnvLoader resolves it
        $this->tempDir = (string) realpath($this->tempDir);
        EnvLoader::setExecutableRootForTest($this->tempDir);
        EnvLoader::setHomeDirForTest($this->tempDir . DIRECTORY_SEPARATOR . 'home');
        $this->warnings = [];
        EnvLoader::setWarningHandler(function (string $message): void {
            $this->warnings[] = $message;
        });
    }

    protected function tearDown(): void
    {
        MachineIdentity::setTestHooks(null, null);
        $this->removeDir($this->tempDir);
        EnvLoader::setWarningHandler(null);
        EnvLoader::setExecutableRootForTest(null);
        EnvLoader::setHomeDirForTest(null);
        EnvLoader::setHardwareIdForTest(null);
        EnvLoader::clear();
        parent::tearDown();
    }

    private function removeDir(string $dir): void
    {
        if (! is_dir($dir)) {
            return;
        }
        foreach (glob($dir . '/{,.}*', GLOB_BRACE) ?: [] as $file) {
            $base = basename($file);
            if ($base === '.' || $base === '..') {
                continue;
            }
            is_dir($file) ? $this->removeDir($file) : unlink($file);
        }
        rmdir($dir);
    }

    public function testV3KnownAnswersSharedWithGo(): void
    {
        self::assertSame(
            '242105607bde99e45b89b953c3306952ad59ceba37219e87a44454bfacc97673',
            bin2hex(MachineIdentity::deriveKeyV3([
                'smbiosuuid' => '4c4c4544-0042-3510-8051-b2c04f4a3732',
                'machtype' => 'amd64',
                'machineguid' => '1b4e28ba-2fa1-11d2-883f-0016d3cca427',
                'hostname' => 'build-01.example.com',
                'fsid' => '68ed38fecd67cf6',
            ]))
        );
        self::assertSame(
            '4b4610f288c50c5ff7ad1c006b276ecab575313dc7577d32d4f87fff1bcebf89',
            bin2hex(MachineIdentity::deriveKeyV3(['override' => '12345']))
        );
    }

    public function testDecryptsGoV3Value(): void
    {
        self::assertSame('shared-secret', Crypto::decrypt(self::GO_V3_VALUE, 'shared'));
    }

    public function testV2ValueIsMigratedSilently(): void
    {
        $path = $this->tempDir . '/v2.env';
        $marker = I18n::builtinText('config.password_message', 'en');
        file_put_contents(
            $path,
            "DATABASE_PASSWORD=\"{$marker}\"\nDATABASE_SECURE_PASSWORD=" . self::V2_VALUE_DATABASE . "\n"
        );
        EnvLoader::load($path);
        self::assertSame('v2-secret', EnvLoader::get('DATABASE_PASSWORD'));
        self::assertStringContainsString(
            'DATABASE_SECURE_PASSWORD=v3:override:',
            (string) file_get_contents($path)
        );
        self::assertSame([], $this->warnings);
    }

    public function testSourceNormalization(): void
    {
        $hostnames = [
            " Build-01.Example.COM\r\n" => 'build-01.example.com',
            'NOAH' => 'noah',
            'localhost' => '',
            'LocalHost.LocalDomain' => '',
            'bad name' => '',
            "m\u{00fc}ller" => '',
            '' => '',
            str_repeat('a', 256) => '',
        ];
        foreach ($hostnames as $in => $want) {
            self::assertSame($want, MachineIdentity::normalizeHostname((string) $in), (string) $in);
        }
        $machTypes = [
            "x86_64\n" => 'amd64', 'AMD64' => 'amd64', 'aarch64' => 'arm64', 'ARM64' => 'arm64',
            'i686' => '386', 'x86' => '386', 'armv7l' => 'arm', 'riscv64' => 'riscv64',
            'ppc64le' => 'ppc64le', 'unknown' => '', 'bad value' => '', '' => '',
        ];
        foreach ($machTypes as $in => $want) {
            self::assertSame($want, MachineIdentity::normalizeMachType((string) $in), (string) $in);
        }
        $fsids = [
            "68ed38fecd67cf6\n" => '68ed38fecd67cf6', '00EA5DC109' => 'ea5dc109',
            '0' => '', '' => '', 'xyz' => '', '12345678901234567' => '',
        ];
        foreach ($fsids as $in => $want) {
            self::assertSame($want, MachineIdentity::normalizeFsid((string) $in), (string) $in);
        }
    }

    public function testCollectV3Sources(): void
    {
        $sources = MachineIdentity::collectV3();
        foreach ($sources as $name => $value) {
            self::assertMatchesRegularExpression('/^[a-z0-9]{1,32}$/', $name);
            self::assertNotSame('', $value);
        }
        self::assertArrayHasKey('machtype', $sources);
        self::assertNotSame('', MachineIdentity::fileSystemId($this->tempDir . '/missing/x.env'));
    }

    /**
     * Uses the real machine sources with a replaced host name and fsid.
     */
    public function testBindsHostnameAndFileSystem(): void
    {
        EnvLoader::setHardwareIdForTest(null);
        $host = 'host-a';
        $fsids = [];
        MachineIdentity::setTestHooks(
            static function () use (&$host): string {
                return $host;
            },
            static function (string $path) use (&$fsids): string {
                $path = str_replace('\\', '/', $path);
                foreach ($fsids as $prefix => $id) {
                    if (strpos($path, str_replace('\\', '/', (string) $prefix)) === 0) {
                        return $id;
                    }
                }

                return 'aaa';
            }
        );
        $path = $this->tempDir . '/bound.env';
        file_put_contents($path, "DB_PASSWORD=bound-secret\n");
        EnvLoader::load($path);
        $content = (string) file_get_contents($path);
        self::assertMatchesRegularExpression(
            '/DB_SECURE_PASSWORD=v3:[a-z0-9,]*fsid,hostname,[a-z0-9,]*machtype/',
            $content
        );

        // backup on another file system, also as clean copy
        $other = $this->tempDir . '/otherfs';
        mkdir($other, 0700);
        $fsids[$other] = 'bbb';
        EnvLoader::updateEnv($other . '/backup.env');
        EnvLoader::updateEnv($other . '/clean.env', true);
        self::assertStringContainsString('DB_PASSWORD=bound-secret', (string) file_get_contents($other . '/clean.env'));
        EnvLoader::clear();
        Crypto::reset();
        EnvLoader::load($other . '/backup.env');
        self::assertSame('bound-secret', EnvLoader::get('DB_PASSWORD'));

        // file moved to another file system
        EnvLoader::clear();
        Crypto::reset();
        $fsids[$this->tempDir . '/bound'] = 'ccc';

        try {
            EnvLoader::load($path);
            self::fail('a changed file system must not decrypt');
        } catch (\RuntimeException $e) {
            self::assertStringContainsString(I18n::t('config.v3_key_mismatch'), $e->getMessage());
        }
        unset($fsids[$this->tempDir . '/bound']);

        // computer renamed
        EnvLoader::clear();
        Crypto::reset();
        $host = 'host-b';

        try {
            EnvLoader::load($path);
            self::fail('a renamed computer must not decrypt');
        } catch (\RuntimeException $e) {
            self::assertNotSame('', $e->getMessage());
        }
        $host = 'host-a';
        EnvLoader::clear();
        Crypto::reset();
        EnvLoader::load($path);
        self::assertSame('bound-secret', EnvLoader::get('DB_PASSWORD'));
    }
}
