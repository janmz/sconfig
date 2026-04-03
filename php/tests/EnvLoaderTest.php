<?php

declare(strict_types=1);

namespace Sconfig\Tests;

use PHPUnit\Framework\TestCase;
use Sconfig\EnvLoader;

/**
 * Tests for EnvLoader: load, get, has, clear, password encryption, cleanConfig.
 */
class EnvLoaderTest extends TestCase
{
    private string $tempDir;

    protected function setUp(): void
    {
        parent::setUp();
        EnvLoader::clear();
        $this->tempDir = sys_get_temp_dir() . '/sconfig_php_test_' . uniqid('', true);
        mkdir($this->tempDir, 0700, true);
        EnvLoader::setExecutableRootForTest($this->tempDir);
    }

    protected function tearDown(): void
    {
        if (isset($this->tempDir) && is_dir($this->tempDir)) {
            $files = new \RecursiveIteratorIterator(
                new \RecursiveDirectoryIterator($this->tempDir, \RecursiveDirectoryIterator::SKIP_DOTS),
                \RecursiveIteratorIterator::CHILD_FIRST
            );
            foreach ($files as $file) {
                if ($file->isDir()) {
                    rmdir($file->getRealPath());
                } else {
                    unlink($file->getRealPath());
                }
            }
            rmdir($this->tempDir);
        }
        EnvLoader::setExecutableRootForTest(null);
        EnvLoader::clear();
        parent::tearDown();
    }

    private function envPath(string $name = 'test.env'): string
    {
        return $this->tempDir . '/' . $name;
    }

    public function testClearResetsState(): void
    {
        $path = $this->envPath();
        $uniqueKey = 'SCONFIG_TEST_CLEAR_' . uniqid();
        file_put_contents($path, $uniqueKey . "=bar\n");
        EnvLoader::load($path);
        self::assertTrue(EnvLoader::isLoaded());
        self::assertSame('bar', EnvLoader::get($uniqueKey));

        EnvLoader::clear();
        self::assertFalse(EnvLoader::isLoaded());
        // After clear(), cache is empty; unset env so get() no longer sees the value
        putenv($uniqueKey);
        unset($_ENV[$uniqueKey]);
        self::assertNull(EnvLoader::get($uniqueKey));
    }

    public function testGetReturnsDefaultWhenKeyMissing(): void
    {
        EnvLoader::load($this->envPath()); // empty file
        self::assertNull(EnvLoader::get('MISSING'));
        self::assertSame('default', EnvLoader::get('MISSING', 'default'));
    }

    public function testHasReturnsFalseWhenKeyMissing(): void
    {
        EnvLoader::load($this->envPath());
        self::assertFalse(EnvLoader::has('MISSING'));
    }

    public function testHasReturnsTrueWhenKeyPresent(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "FOO=bar\n");
        EnvLoader::load($path);
        self::assertTrue(EnvLoader::has('FOO'));
    }

    public function testLoadCreatesEmptyFileWhenMissing(): void
    {
        $path = $this->envPath('missing.env');
        self::assertFileDoesNotExist($path);
        EnvLoader::load($path);
        self::assertFileExists($path);
        self::assertSame('', trim(file_get_contents($path)));
    }

    public function testLoadParsesKeyValue(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "FOO=bar\nBAZ=qux\n");
        EnvLoader::load($path);
        self::assertSame('bar', EnvLoader::get('FOO'));
        self::assertSame('qux', EnvLoader::get('BAZ'));
    }

    public function testLoadStripsDoubleQuotes(): void
    {
        $path = $this->envPath();
        file_put_contents($path, 'FOO="bar value"' . "\n");
        EnvLoader::load($path);
        self::assertSame('bar value', EnvLoader::get('FOO'));
    }

    public function testLoadStripsSingleQuotes(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "FOO='bar value'\n");
        EnvLoader::load($path);
        self::assertSame('bar value', EnvLoader::get('FOO'));
    }

    public function testLoadPreservesComments(): void
    {
        $path = $this->envPath();
        $content = "# comment\nFOO=bar\n# another\nBAR=baz\n";
        file_put_contents($path, $content);
        EnvLoader::load($path);
        self::assertSame('bar', EnvLoader::get('FOO'));
        self::assertSame('baz', EnvLoader::get('BAR'));
        $written = file_get_contents($path);
        self::assertStringContainsString('# comment', $written);
        self::assertStringContainsString('# another', $written);
    }

    public function testLoadThrowsWhenFileNotReadable(): void
    {
        if (PHP_OS_FAMILY === 'Windows') {
            $this->markTestSkipped('Cannot make file unreadable on Windows');
        }
        $path = $this->envPath();
        file_put_contents($path, "FOO=bar\n");
        chmod($path, 0000);
        try {
            $this->expectException(\RuntimeException::class);
            $this->expectExceptionMessage('failed to read');
            EnvLoader::load($path);
        } finally {
            chmod($path, 0600);
        }
    }

    public function testOverrideFalseKeepsExistingValues(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "FOO=first\n");
        EnvLoader::load($path);
        self::assertSame('first', EnvLoader::get('FOO'));

        file_put_contents($path, "FOO=second\n");
        EnvLoader::load($path, false);
        self::assertSame('first', EnvLoader::get('FOO'));
    }

    public function testOverrideTrueOverwritesValues(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "FOO=first\n");
        EnvLoader::load($path);
        file_put_contents($path, "FOO=second\n");
        EnvLoader::load($path, true);
        self::assertSame('second', EnvLoader::get('FOO'));
    }

    public function testPasswordEncryptionReplacesPlaintextWithMarkerInFile(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "DATABASE_PASSWORD=plaintext-secret\n");
        EnvLoader::load($path);

        self::assertSame('plaintext-secret', EnvLoader::get('DATABASE_PASSWORD'));
        $content = file_get_contents($path);
        self::assertStringNotContainsString('plaintext-secret', $content);
        self::assertMatchesRegularExpression('/DATABASE_PASSWORD=.+/', $content);
        self::assertStringContainsString('DATABASE_SECURE_PASSWORD=', $content);
    }

    public function testPasswordEncryptionDecryptsInMemory(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "API_PASSWORD=my-api-key\n");
        EnvLoader::load($path);
        self::assertSame('my-api-key', EnvLoader::get('API_PASSWORD'));
    }

    public function testCleanConfigLeavesPlaintextInFile(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "DATABASE_PASSWORD=plain-in-file\n");
        EnvLoader::load($path, true, true);
        self::assertSame('plain-in-file', EnvLoader::get('DATABASE_PASSWORD'));
        $content = file_get_contents($path);
        self::assertStringContainsString('plain-in-file', $content);
    }

    public function testIsLoadedFalseInitially(): void
    {
        EnvLoader::clear();
        self::assertFalse(EnvLoader::isLoaded());
    }

    public function testIsLoadedTrueAfterLoad(): void
    {
        EnvLoader::load($this->envPath());
        self::assertTrue(EnvLoader::isLoaded());
    }

    public function testGetReturnsFromCacheAfterLoad(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "CACHE_TEST=value1\n");
        EnvLoader::load($path);
        putenv('CACHE_TEST=value2'); // should not override cache
        self::assertSame('value1', EnvLoader::get('CACHE_TEST'));
    }

    public function testEnvHelperReturnsValueAfterLoad(): void
    {
        if (!function_exists('env')) {
            $this->markTestSkipped('env() helper not loaded (e.g. autoload files)');
        }
        $path = $this->envPath();
        file_put_contents($path, "HELPER_FOO=helper-value\n");
        EnvLoader::load($path);
        self::assertSame('helper-value', env('HELPER_FOO'));
        self::assertSame('default', env('HELPER_MISSING', 'default'));
    }

    public function testSetUpdatesCacheAndGetReturnsValue(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "INITIAL=ok\n");
        EnvLoader::load($path);
        EnvLoader::set('THEME', 'light');
        self::assertSame('light', EnvLoader::get('THEME'));
    }

    public function testUpdateEnvWritesThemeChange(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "THEME=dark\n");
        EnvLoader::load($path);
        EnvLoader::set('THEME', 'light');
        EnvLoader::updateEnv($path);
        $content = file_get_contents($path);
        self::assertStringContainsString('THEME=light', $content);
    }

    public function testUpdateEnvThrowsWhenNotLoaded(): void
    {
        EnvLoader::clear();
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('Config');
        EnvLoader::updateEnv($this->envPath());
    }

    public function testUpdateEnvDifferentPath(): void
    {
        $loadPath = $this->envPath('app.env');
        $writePath = $this->envPath('backup.env');
        file_put_contents($loadPath, "THEME=dark\n");
        EnvLoader::load($loadPath);
        EnvLoader::set('THEME', 'light');
        EnvLoader::updateEnv($writePath);
        self::assertFileExists($writePath);
        $content = file_get_contents($writePath);
        self::assertStringContainsString('THEME=light', $content);
    }

    public function testUpdateEnvCleanConfigWritesPlaintextPassword(): void
    {
        $path = $this->envPath();
        file_put_contents($path, "DB_PASSWORD=secret\n");
        EnvLoader::load($path);
        EnvLoader::updateEnv($path, true);
        $content = file_get_contents($path);
        self::assertStringContainsString('secret', $content);
    }

    public function testLoadRejectsPathOutsideExecutableRoot(): void
    {
        $outside = dirname($this->tempDir) . DIRECTORY_SEPARATOR . 'outside_' . uniqid('', true) . '.env';
        $this->expectException(\RuntimeException::class);
        EnvLoader::load($outside);
    }
}
