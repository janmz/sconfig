package sconfig

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

// Key file limits (bytes). New key files get keyFileNewSize random bytes.
const (
	keyFileMinSize = 16
	keyFileMaxSize = 4096
	keyFileNewSize = 32
)

// settings are process-wide options. They are not cleared by ResetForTest.
// All access is serialized by apiMu.
var settings struct {
	// keyFileValue is the hex content of the key file ("" = no key file).
	keyFileValue string
	// rejectLegacy refuses legacy v1 values instead of migrating them.
	rejectLegacy bool
	// warnHandler receives warnings; nil writes them to stderr.
	warnHandler func(string)
	// allowWeakIdentity permits new values without a strong source.
	allowWeakIdentity bool
	// excludeHostname / excludeFSID leave these sources out of new values.
	excludeHostname bool
	excludeFSID     bool
}

// SetAllowWeakIdentity controls what happens when no strong identity source
// (a machine ID such as machineguid or machine-id, or a key file) is
// available, e.g. in a container without /etc/machine-id. By default
// encrypting then fails, because a key from host name, architecture and
// file system ID alone can be guessed. With true such a weak key is used
// and a warning is issued once per process. Prefer SetKeyFile.
func SetAllowWeakIdentity(allow bool) {
	apiMu.Lock()
	defer apiMu.Unlock()
	settings.allowWeakIdentity = allow
}

// SetUseHostname controls whether new values are bound to the host name
// (default true). With false, values are portable between containers whose
// host name changes; existing values that use the host name are re-encrypted
// without it on the next LoadConfig (the old host name must still match).
func SetUseHostname(use bool) {
	apiMu.Lock()
	defer apiMu.Unlock()
	settings.excludeHostname = !use
}

// SetUseFileSystemID controls whether new values are bound to the file
// system of the config file (default true). With false, the config file can
// be moved to another drive; existing values that use the file system ID are
// re-encrypted without it on the next LoadConfig (while still readable).
func SetUseFileSystemID(use bool) {
	apiMu.Lock()
	defer apiMu.Unlock()
	settings.excludeFSID = !use
}

// SetWarningHandler sets the function that receives warnings of the library
// (e.g. "legacy values were migrated, rotate the passwords" or "config file is
// readable by other users"). nil restores the default, which writes the
// warnings to stderr; pass func(string) {} to ignore them.
func SetWarningHandler(handler func(message string)) {
	apiMu.Lock()
	defer apiMu.Unlock()
	settings.warnHandler = handler
}

// SetRejectLegacy controls the strict mode. With true, legacy v1 values
// (encrypted before version 3.0.0, small key space) are refused with an error
// instead of being decrypted and migrated to v3.
func SetRejectLegacy(reject bool) {
	apiMu.Lock()
	defer apiMu.Unlock()
	settings.rejectLegacy = reject
}

// SetKeyFile adds a secret key file as additional source "keyfile" of the
// machine identity. If the file does not exist, it is created with 32 random
// bytes and permissions 0600 (on Windows: access for owner, SYSTEM and
// Administrators only). Values encrypted with a key file can only be
// decrypted with the same file: losing it loses the passwords. The path must
// lie under the executable directory, the CWD or the user's home directory.
// Call it before LoadConfig; an empty path removes the key file again.
func SetKeyFile(path string) error {
	apiMu.Lock()
	defer apiMu.Unlock()
	value := ""
	if path != "" {
		resolved, err := resolveConfigPath(path)
		if err != nil {
			return err
		}
		if value, err = readOrCreateKeyFile(resolved); err != nil {
			return fmt.Errorf("%s", t("config.keyfile_invalid", resolved, err))
		}
	}
	settings.keyFileValue = value
	// sources and keys must be collected again
	resetSources()
	return nil
}

// readOrCreateKeyFile returns the hex content of the key file, creating the
// file with random content if it does not exist.
func readOrCreateKeyFile(path string) (string, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return createKeyFile(path)
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, keyFileMaxSize+1))
	if err != nil {
		return "", err
	}
	if len(data) < keyFileMinSize || len(data) > keyFileMaxSize {
		return "", fmt.Errorf("size must be %d to %d bytes, got %d", keyFileMinSize, keyFileMaxSize, len(data))
	}
	return hex.EncodeToString(data), nil
}

// createKeyFile creates a new key file exclusively with random content.
func createKeyFile(path string) (string, error) {
	data := make([]byte, keyFileNewSize)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, newConfigFileMode)
	if err != nil {
		return "", err
	}
	if err := restrictNewFile(path); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// warn passes a warning to the handler (default: stderr).
func warn(message string) {
	if settings.warnHandler != nil {
		settings.warnHandler(message)
		return
	}
	fmt.Fprintln(os.Stderr, "sconfig: "+message)
}
