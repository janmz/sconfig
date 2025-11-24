<?php

if (!function_exists('env')) {
    /**
     * Get an environment variable value
     *
     * This is a global helper function that provides easy access to environment variables.
     * It first checks the EnvLoader cache, then $_ENV, then getenv().
     *
     * @param string $key The environment variable key
     * @param mixed $default Default value if key is not found
     * @return mixed The environment variable value or default
     */
    function env(string $key, $default = null)
    {
        return \Sconfig\EnvLoader::get($key, $default);
    }
}

