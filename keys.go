package sconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
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

// errSourceMissing marks a v2/v3 value whose identity source is not readable
// on this system any more.
var errSourceMissing = errors.New("machine identity source not available")

// keyState holds the lazily derived keys. All access is serialized by apiMu.
type keyState struct {
	initialized bool
	// hardwareIDFunc returns the legacy v1 hardware ID (default or override).
	hardwareIDFunc func() (uint64, error)
	// overridden is true when LoadConfig received a custom hardware-ID
	// function; v2/v3 then use only the source "override".
	overridden bool

	v1Key []byte
	v1Err error

	// machine-wide identity sources (v2 + hostname, machtype) or "override"
	srcCollected bool
	srcMachine   map[string]string
	srcErr       error

	// fsid is the file system ID of the current config file ("" = none);
	// knownFSIDs are all file system IDs seen in this process (current
	// first), so that values of a file loaded before stay readable when
	// another file is written (e.g. a backup on another drive).
	fsid       string
	knownFSIDs []string

	// derived caches keys by format version, source names and values.
	derived map[string][]byte
}

var keys keyState

// resetKeys clears all derived keys (tests, ResetForTest).
func resetKeys() {
	keys = keyState{}
}

// resetSources forgets the collected sources and derived keys (e.g. after
// the key file changed); the legacy key and the fsid are kept.
func resetSources() {
	keys.srcCollected = false
	keys.srcMachine = nil
	keys.srcErr = nil
	keys.derived = nil
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

// setConfigFile selects the config file whose file system ID is used as
// source "fsid" by the following encrypt and decrypt calls.
func setConfigFile(path string) {
	keys.fsid = ""
	if keys.overridden {
		return
	}
	keys.fsid = configFileSystemID(path)
	if keys.fsid != "" {
		keys.knownFSIDs = slices.DeleteFunc(keys.knownFSIDs, func(v string) bool { return v == keys.fsid })
		keys.knownFSIDs = slices.Insert(keys.knownFSIDs, 0, keys.fsid)
	}
	if debugMode {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] fsid of %s: %q\n", path, keys.fsid)
	}
}

// machineSources returns all identity sources for new values: the
// machine-wide sources (collected on first use), the fsid of the current
// config file and the optional key file. The returned map is a copy.
func machineSources() (map[string]string, error) {
	if !keys.srcCollected {
		keys.srcCollected = true
		if keys.overridden {
			id, err := keys.hardwareIDFunc()
			if err != nil {
				keys.srcErr = fmt.Errorf("%s: %w", t("config.hardware_id_failed"), err)
			} else {
				keys.srcMachine = map[string]string{sourceOverride: strconv.FormatUint(id, 10)}
			}
		} else {
			keys.srcMachine, keys.srcErr = collectMachineSourcesV3()
			if keys.srcErr != nil {
				keys.srcErr = fmt.Errorf("%s: %w", t("config.hardware_id_failed"), keys.srcErr)
			}
		}
		if debugMode {
			printMachineSources("v3, without fsid and keyfile", keys.srcMachine, keys.srcErr)
		}
	}
	if keys.srcErr != nil {
		return nil, keys.srcErr
	}
	sources := maps.Clone(keys.srcMachine)
	if sources == nil {
		sources = map[string]string{}
	}
	if keys.fsid != "" {
		sources[sourceFSID] = keys.fsid
	}
	if settings.keyFileValue != "" {
		sources[sourceKeyFile] = settings.keyFileValue
	}
	return sources, nil
}

// candidateSources returns the source sets to try for a value with the given
// source names: one set, or, if the value uses fsid, one set per known file
// system ID (current config file first).
func candidateSources(names []string) ([]map[string]string, error) {
	sources, err := machineSources()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(names, sourceFSID) || len(keys.knownFSIDs) == 0 {
		return []map[string]string{sources}, nil
	}
	sets := make([]map[string]string, 0, len(keys.knownFSIDs))
	for _, fsid := range keys.knownFSIDs {
		set := maps.Clone(sources)
		set[sourceFSID] = fsid
		sets = append(sets, set)
	}
	return sets, nil
}

// sourceKey returns the key of format version ("v2" or "v3") for the given
// source names, taking the values from sources.
func sourceKey(version string, names []string, sources map[string]string) ([]byte, error) {
	var cacheKey strings.Builder
	cacheKey.WriteString(version)
	used := make(map[string]string, len(names))
	for _, name := range names {
		value, ok := sources[name]
		if !ok {
			return nil, fmt.Errorf("%w: %s", errSourceMissing, t("config.v2_source_missing", name))
		}
		used[name] = value
		cacheKey.WriteString("\x00" + name + "=" + value)
	}
	if key, ok := keys.derived[cacheKey.String()]; ok {
		return key, nil
	}
	derive := deriveKeyV3
	if version == "v2" {
		derive = deriveKeyV2
	}
	key, err := derive(used)
	if err != nil {
		return nil, err
	}
	if keys.derived == nil {
		keys.derived = map[string][]byte{}
	}
	keys.derived[cacheKey.String()] = key
	return key, nil
}

// isV2Value reports whether an encrypted value uses the v2 format.
func isV2Value(value string) bool {
	return strings.HasPrefix(value, v2Prefix)
}

// isV3Value reports whether an encrypted value uses the v3 format.
func isV3Value(value string) bool {
	return strings.HasPrefix(value, v3Prefix)
}

// isLegacyValue reports whether an encrypted value uses the legacy v1 format
// (plain base64 without a version prefix).
func isLegacyValue(value string) bool {
	return !isV2Value(value) && !isV3Value(value)
}

// newGCM creates the AES-256-GCM AEAD for the given key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher init: %w", err)
	}
	return cipher.NewGCM(block)
}

// fieldAAD returns the additional authenticated data of a v2/v3 value: the
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

// needsUpgrade reports whether a value should be re-encrypted with all
// current sources: every v2 value (migration to v3) and v3 values that were
// encrypted with fewer sources than are available now (e.g. a key file was
// added).
func needsUpgrade(value string) bool {
	if isV2Value(value) {
		return true
	}
	names, _, err := parseV3Value(value)
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

// encrypt encrypts text for the password field `field` with the v3 key of all
// currently available sources and returns the complete "v3:<names>:<base64>"
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
	key, err := sourceKey("v3", names, sources)
	if err != nil {
		return "", err
	}
	b64, err := sealB64(key, text, fieldAAD(field))
	if err != nil {
		return "", err
	}
	return v3Prefix + strings.Join(names, ",") + ":" + b64, nil
}

// decrypt decrypts a v3 or v2 value of the password field `field` or a legacy
// v1 value (plain base64, no field binding). The value is validated and
// decoded exactly once, before any key is derived, so that a malformed value
// is always reported as errInvalidCiphertext. With SetRejectLegacy(true) v1
// values are refused.
func decrypt(value, field string) (string, error) {
	version := ""
	b64 := value
	var names []string
	var err error
	switch {
	case isV3Value(value):
		version = "v3"
		names, b64, err = parseV3Value(value)
	case isV2Value(value):
		version = "v2"
		names, b64, err = parseV2Value(value)
	}
	if err != nil {
		return "", err
	}
	data, err := decodeCiphertextB64(b64)
	if err != nil {
		return "", err
	}
	if version == "" {
		if settings.rejectLegacy {
			return "", errors.New(t("config.legacy_rejected"))
		}
		key, err := legacyKey()
		if err != nil {
			return "", err
		}
		return openData(key, data, nil)
	}
	sets, err := candidateSources(names)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, sources := range sets {
		key, err := sourceKey(version, names, sources)
		if err != nil {
			return "", err
		}
		plain, err := openData(key, data, fieldAAD(field))
		if err == nil {
			return plain, nil
		}
		lastErr = err
	}
	if version == "v3" {
		return "", fmt.Errorf("%w (%s)", lastErr, t("config.v3_key_mismatch"))
	}
	return "", lastErr
}
