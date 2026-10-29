<?php

namespace Sconfig;

/**
 * Warnings of the library (e.g. migrated legacy values, readable files).
 *
 * The default handler writes to the PHP error log (error_log()); it never
 * raises a PHP warning, so strict error handlers of applications are not
 * triggered.
 *
 * @package Sconfig
 */
final class Warnings
{
    /** @var callable|null */
    private static $handler = null;

    /**
     * Set the handler that receives warning texts; null restores error_log().
     *
     * @param callable(string): void|null $handler
     */
    public static function setHandler(?callable $handler): void
    {
        self::$handler = $handler;
    }

    /**
     * Pass a warning to the handler.
     */
    public static function emit(string $message): void
    {
        if (self::$handler !== null) {
            (self::$handler)($message);

            return;
        }
        error_log('sconfig: ' . $message);
    }
}
