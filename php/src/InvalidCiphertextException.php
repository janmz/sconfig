<?php

namespace Sconfig;

/**
 * Thrown for structurally invalid encrypted values (format, base64, length),
 * as opposed to values that fail authentication (wrong key).
 *
 * @package Sconfig
 */
class InvalidCiphertextException extends \RuntimeException
{
}
