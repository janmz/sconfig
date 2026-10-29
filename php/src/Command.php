<?php

namespace Sconfig;

/**
 * Runs external commands with a timeout (used for the machine identity).
 *
 * A timeout throws CommandTimeoutException. Callers must not treat it as a
 * "missing identifier", otherwise a slow system would silently get another key.
 * Commands get a minimal environment (no passwords exported by EnvLoader, a
 * fixed search path) and their output is limited.
 *
 * @package Sconfig
 * @internal
 */
final class Command
{
    /** Default timeout in seconds for identity commands. */
    public const DEFAULT_TIMEOUT = 15.0;

    /** Timeout in seconds for PowerShell (cold starts can be slow). */
    public const POWERSHELL_TIMEOUT = 30.0;

    /** Maximum number of output bytes that are read. */
    private const MAX_OUTPUT = 1048576;

    /** Environment variables passed to commands on Windows. */
    private const WINDOWS_ENV = [
        'SystemRoot', 'windir', 'SystemDrive', 'ComSpec', 'PATHEXT', 'TEMP', 'TMP',
        'USERPROFILE', 'APPDATA', 'LOCALAPPDATA', 'ProgramData', 'ProgramFiles',
        'PROCESSOR_ARCHITECTURE', 'NUMBER_OF_PROCESSORS', 'OS', 'COMPUTERNAME', 'USERNAME',
    ];

    /**
     * Run a command and return its standard output ('' on any failure).
     *
     * @param string|array<int, string> $command Shell command line (string) or
     *        program plus arguments (array, executed without a shell)
     * @param float $timeout Timeout in seconds
     * @return string Standard output (at most 1 MiB), '' if the command could not run
     * @throws CommandTimeoutException If the command did not finish in time
     */
    public static function run($command, float $timeout = self::DEFAULT_TIMEOUT): string
    {
        $tmp = tempnam(sys_get_temp_dir(), 'scf');
        if ($tmp === false) {
            return '';
        }
        $null = PHP_OS_FAMILY === 'Windows' ? 'NUL' : '/dev/null';
        $descriptors = [
            0 => ['file', $null, 'r'],
            1 => ['file', $tmp, 'w'],
            2 => ['file', $null, 'w'],
        ];
        $process = @proc_open($command, $descriptors, $pipes, null, self::environment());
        if (! is_resource($process)) {
            self::remove($tmp);

            return '';
        }
        $deadline = microtime(true) + $timeout;
        while (true) {
            $status = proc_get_status($process);
            if (! $status['running']) {
                break;
            }
            if (microtime(true) > $deadline) {
                self::kill($process, (int) $status['pid']);
                self::remove($tmp);
                $name = is_array($command) ? (string) ($command[0] ?? '') : $command;

                throw new CommandTimeoutException(
                    sprintf('external command timed out: %s (%.0f s)', $name, $timeout)
                );
            }
            usleep(10000);
        }
        proc_close($process);
        $output = @file_get_contents($tmp, false, null, 0, self::MAX_OUTPUT);
        self::remove($tmp);

        return $output === false ? '' : $output;
    }

    /**
     * Absolute path of a program in the Windows System32 directory.
     */
    public static function windowsTool(string $relative): string
    {
        return self::windowsSystemDir() . '\\' . $relative;
    }

    /**
     * System32 directory. SystemRoot is only used if it is a plain absolute
     * path to an existing Windows directory; otherwise C:\Windows.
     */
    public static function windowsSystemDir(): string
    {
        $root = getenv('SystemRoot');
        if (
            is_string($root) && preg_match('/^[A-Za-z]:\\\\[^;%"<>|?*]*$/', $root) === 1
            && is_file($root . '\\System32\\kernel32.dll')
        ) {
            return rtrim($root, '\\') . '\\System32';
        }

        return 'C:\\Windows\\System32';
    }

    /**
     * Minimal environment for commands.
     *
     * @return array<string, string>
     */
    private static function environment(): array
    {
        if (PHP_OS_FAMILY !== 'Windows') {
            $env = ['PATH' => '/usr/sbin:/usr/bin:/sbin:/bin'];
            foreach (['LANG', 'LC_ALL', 'LC_MESSAGES', 'LC_CTYPE'] as $name) {
                $value = getenv($name);
                if (is_string($value) && $value !== '') {
                    $env[$name] = $value;
                }
            }

            return $env;
        }
        $sys = self::windowsSystemDir();
        $env = [
            'PATH' => $sys . ';' . $sys . '\\Wbem;' . $sys . '\\WindowsPowerShell\\v1.0',
            'NoDefaultCurrentDirectoryInExePath' => '1',
        ];
        foreach (self::WINDOWS_ENV as $name) {
            $value = getenv($name);
            if (is_string($value) && $value !== '') {
                $env[$name] = $value;
            }
        }

        return $env;
    }

    /**
     * Delete a temporary file; retries briefly because a killed process on
     * Windows may still hold the handle for a moment.
     */
    private static function remove(string $path): void
    {
        for ($i = 0; $i < 50 && file_exists($path); $i++) {
            if (@unlink($path)) {
                return;
            }
            usleep(20000);
        }
    }

    /**
     * Terminate a process and (on Windows) its child processes.
     *
     * @param resource $process
     */
    private static function kill($process, int $pid): void
    {
        if (PHP_OS_FAMILY === 'Windows' && $pid > 0) {
            @exec('"' . self::windowsTool('taskkill.exe') . '" /F /T /PID ' . $pid . ' >NUL 2>&1');
        }
        @proc_terminate($process);
        @proc_close($process);
    }
}
