package sconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// maxEncryptedPasswordB64Len limits the base64 ciphertext size to avoid
// excessive memory use from malformed or hostile config values.
const maxEncryptedPasswordB64Len = 4096

// gcmNonceSize and gcmTagSize describe the AES-GCM layout nonce|ciphertext|tag.
const (
	gcmNonceSize = 12
	gcmTagSize   = 16
)

// errInvalidCiphertext marks values that are structurally invalid (format,
// base64, length) as opposed to values that fail authentication.
var errInvalidCiphertext = errors.New("invalid encrypted value")

// errSourceMissing marks a v2 value whose identity source is not readable on
// this system any more.
var errSourceMissing = errors.New("machine identity source not available")

// keyState holds the lazily derived keys. All access is serialized by apiMu.
type keyState struct {
	initialized bool
	// hardwareIDFunc returns the legacy v1 hardware ID (default or override).
	hardwareIDFunc func() (uint64, error)
	// overridden is true when LoadConfig received a custom hardware-ID
	// function; v2 then uses only the source "override".
	overridden bool

	v1Key []byte
	v1Err error

	v2Collected bool
	v2Sources   map[string]string
	v2Err       error
	v2Keys      map[string][]byte // keyed by the comma-joined source names
}

var keys keyState

// resetKeys clears all derived keys (tests, ResetForTest).
func resetKeys() {
	keys = keyState{}
}

// legacyKey returns the v1 key, deriving it on first use.
func legacyKey() ([]byte, error) {
	if keys.v1Key == nil && keys.v1Err == nil {
		id, err := keys.hardwareIDFunc()
		if err != nil {
			keys.v1Err = fmt.Errorf("%s: %w", t("config.hardware_id_failed"), err)
		} else {
			keys.v1Key = deriveKeyV1(id)
			if debugMode {
				fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Hardware ID (v1) used for legacy key: %d (0x%016x)\n", id, id)
				fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Legacy encryption key (32 bytes): %x\n", keys.v1Key)
			}
		}
	}
	return keys.v1Key, keys.v1Err
}

// machineSources returns the v2 identity sources of this system (plus the
// optional key file), collecting them on first use.
func machineSources() (map[string]string, error) {
	if !keys.v2Collected {
		keys.v2Collected = true
		if keys.overridden {
			id, err := keys.hardwareIDFunc()
			if err != nil {
				keys.v2Err = fmt.Errorf("%s: %w", t("config.hardware_id_failed"), err)
			} else {
				keys.v2Sources = map[string]string{sourceOverride: strconv.FormatUint(id, 10)}
			}
		} else {
			keys.v2Sources, keys.v2Err = collectMachineSourcesV2()
			if keys.v2Err != nil {
				keys.v2Err = fmt.Errorf("%s: %w", t("config.hardware_id_failed"), keys.v2Err)
			}
		}
		if keys.v2Err == nil && settings.keyFileValue != "" {
			keys.v2Sources[sourceKeyFile] = settings.keyFileValue
		}
		if debugMode {
			printMachineSourcesV2(keys.v2Sources, keys.v2Err)
		}
	}
	return keys.v2Sources, keys.v2Err
}

// machineKey returns the v2 key for the given source names.
func machineKey(names []string) ([]byte, error) {
	cacheKey := strings.Join(names, ",")
	if key, ok := keys.v2Keys[cacheKey]; ok {
		return key, nil
	}
	sources, err := machineSources()
	if err != nil {
		return nil, err
	}
	used := make(map[string]string, len(names))
	for _, name := range names {
		value, ok := sources[name]
		if !ok {
			return nil, fmt.Errorf("%w: %s", errSourceMissing, t("config.v2_source_missing", name))
		}
		used[name] = value
	}
	key, err := deriveKeyV2(used)
	if err != nil {
		return nil, err
	}
	if keys.v2Keys == nil {
		keys.v2Keys = map[string][]byte{}
	}
	keys.v2Keys[cacheKey] = key
	return key, nil
}

// isV2Value reports whether an encrypted value uses the v2 format.
func isV2Value(value string) bool {
	return strings.HasPrefix(value, v2Prefix)
}

// newGCM creates the AES-256-GCM AEAD for the given key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher init: %w", err)
	}
	return cipher.NewGCM(block)
}

// fieldAAD returns the additional authenticated data of a v2 value: the
// password field prefix in lower case with only a-z and 0-9, so that Go
// ("DatabaseSecurePassword") and PHP ("DATABASE_SECURE_PASSWORD") agree. A
// value can therefore not be moved to a field with another name.
func fieldAAD(field string) []byte {
	var b strings.Builder
	b.WriteString(v2AADPrefix)
	for _, r := range strings.ToLower(field) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return []byte(b.String())
}

// sealB64 encrypts text with a random nonce and returns base64(nonce|ct|tag).
func sealB64(key []byte, text string, aad []byte) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("encrypt: nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(text), aad)), nil
}

// decodeCiphertextB64 validates size and base64 encoding once and returns
// the raw nonce|ciphertext|tag bytes.
func decodeCiphertextB64(text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("%w: empty", errInvalidCiphertext)
	}
	if len(text) > maxEncryptedPasswordB64Len {
		return nil, fmt.Errorf("%w: too long (%d characters, max %d)", errInvalidCiphertext, len(text), maxEncryptedPasswordB64Len)
	}
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64: %v", errInvalidCiphertext, err)
	}
	if len(data) < gcmNonceSize+gcmTagSize {
		return nil, fmt.Errorf("%w: too short (need at least %d bytes, got %d)", errInvalidCiphertext, gcmNonceSize+gcmTagSize, len(data))
	}
	return data, nil
}

// openData decrypts the raw nonce|ciphertext|tag bytes with key.
func openData(key, data, aad []byte) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	plaintext, err := gcm.Open(nil, data[:gcmNonceSize], data[gcmNonceSize:], aad)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// needsUpgrade reports whether a v2 value was encrypted with fewer sources
// than are available now (e.g. a key file was added); such values are
// re-encrypted with all sources.
func needsUpgrade(value string) bool {
	names, _, err := parseV2Value(value)
	if err != nil {
		return false
	}
	sources, err := machineSources()
	if err != nil || len(sources) <= len(names) {
		return false
	}
	for _, name := range names {
		if _, ok := sources[name]; !ok {
			return false // not a subset: keep the value as it is
		}
	}
	return true
}

// encrypt encrypts text for the password field `field` with the v2 key of all
// currently available sources and returns the complete "v2:<names>:<base64>"
// value.
func encrypt(text, field string) (string, error) {
	sources, err := machineSources()
	if err != nil {
		return "", err
	}
	if len(sources) == 0 {
		return "", fmt.Errorf("%s: %w", t("config.hardware_id_failed"), errNoMachineSource)
	}
	names := sortedSourceNames(sources)
	key, err := machineKey(names)
	if err != nil {
		return "", err
	}
	b64, err := sealB64(key, text, fieldAAD(field))
	if err != nil {
		return "", err
	}
	return v2Prefix + strings.Join(names, ",") + ":" + b64, nil
}

// decrypt decrypts a v2 value of the password field `field` or a legacy v1
// value (plain base64, no field binding). The value is validated and decoded
// exactly once, before any key is derived, so that a malformed value is always
// reported as errInvalidCiphertext. With SetRejectLegacy(true) v1 values are
// refused.
func decrypt(value, field string) (string, error) {
	b64 := value
	var names []string
	if isV2Value(value) {
		var err error
		if names, b64, err = parseV2Value(value); err != nil {
			return "", err
		}
	}
	data, err := decodeCiphertextB64(b64)
	if err != nil {
		return "", err
	}
	if names == nil {
		if settings.rejectLegacy {
			return "", errors.New(t("config.legacy_rejected"))
		}
		key, err := legacyKey()
		if err != nil {
			return "", err
		}
		return openData(key, data, nil)
	}
	key, err := machineKey(names)
	if err != nil {
		return "", err
	}
	return openData(key, data, fieldAAD(field))
}
