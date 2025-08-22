package sconfig

// Package sconfig provides utilities to load and maintain JSON configuration files
// that contain sensitive passwords. It transparently encrypts plaintext passwords
// on first load, replaces them in the file with a marker string, and decrypts them
// back into memory for application use.
//
// The package supports:
//   - Default value population via struct field tags (e.g., `default:"value"`).
//   - Automatic version synchronization of a `Version` field in config structs.
//   - Transparent password handling using paired fields following the naming
//     pattern `<Name>Password` and `<Name>SecurePassword`.
//
// Internationalization (i18n) is built-in for error messages using embedded
// locale files in `locales/`. English is the fallback language.
