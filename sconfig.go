package sconfig

/*
 * Description: This package contains functions for managing config files with
 * secure passwords.
 *
 * Design goal: the encryption key is intentionally neither random nor new per
 * call. The same machine must always derive the same key so that stored
 * config or .env data stays readable. The key is derived deterministically
 * from machine-bound data (see hwid_v2.go) that only this machine can
 * reproduce; it cannot be guessed without access to the machine.
 *
 * Version: see version.go
 *
 * ChangeLog:
 *  27.10.26	3.0.0	Feature: v2 key format (HKDF, stable machine sources, source list per value), automatic migration of v1 values, pointer/map/array support, atomic writes
 *  10.10.26	2.0.8	Feature: Allow config files in %USERPROFILE% or ~
 *  24.06.26	2.0.5	Fix: second try to handle empty composer.lock
 *  24.06.26	2.0.4	Fix: use 1.25.x in workflow. an composer audit --locked
 *  24.06.26	2.0.3	Fix: include a pre commit hook to prevent ci-workflow failures, import I18N and other ci workflow updates
 *  03.04.26	2.0.2	Fix: Fixed some security issues
 *  27.02.26	2.0.1	Fix: introduced the mandatory v2 path for version 2
 *  27.02.26	2.0.0	Feature: Introduce UpdateConfig and updateEnv to update the files after changes
 *  27.02.26	1.2.16	Feature: UpdateConfig() to write config after changes (e.g. theme)
 *  25.02.26	1.2.16	Fix: remove masking of negative seeds to positive ones
 *  22.02.26	1.2.15	Fix: removed decryption debugging
 *  22.02.26	1.2.14	Fix: output config file path in debug mode
 *  22.02.26	1.2.13	Fix: Recreated math/rand from go 1.23.0
 *  22.02.26	1.2.12	Fix: Updated to go 1.25
 *  04.02.26	1.2.11	Feature: Added tracking of hardware IDs
 *  03.02.26	1.2.10	Feature: Corrected error message for failed decryption
 *  03.02.26	1.2.9	Feature: New debug function
 *  03.02.25	1.2.8	feat: DebugHardwareID() and docs for debugging hardware key changes
 *  03.12.25	1.2.7	fix: use biggest disk id
 *  03.12.25	1.2.6	fix: use ipconfig for mac address
 *  03.12.25	1.2.5	fix: use route table for mac address
 *  02.12.25	1.2.3	fix: using volatile information on VM has voided the hardware id
 *  24.11.25	1.2.2	fixed missing password replacements
 *  24.11.25	1.2.1	fixed wrong composer settings and documentation
 *  24.11.25	1.2.0	included PHP variant
 *
 * Author: Jan Neuhaus, VAYA Consulting, https://vaya-consultig.de/development/ https://github.com/janmz
 *
 * Functions:
 * - LoadConfig(): Loads the configuration from a file and processes it; it may
 *   rewrite the file to encrypt passwords or to migrate legacy values.
 * - UpdateConfig(): Writes the config struct to a file; requires LoadConfig to
 *   have been called first. Secure fields are encrypted in the file unless
 *   cleanConfig is true. The path may differ from the one used in LoadConfig.
 * - DebugHardwareID(): Prints the legacy hardware ID and the v2 machine
 *   identity with all inputs to stderr.
 * - SetLocalesDir(): Loads additional translations from a directory.
 *
 * Files:
 * - hwid_v2.go: machine identity and key derivation for new values (v2)
 * - hwid_v1.go, key_rand_go123.go: frozen legacy hardware ID (v1, read only)
 * - keys.go: key cache, encryption and decryption
 * - walk.go: struct traversal, default values, password pairs
 * - fileio.go: atomic file writes
 * - i18n.go and locales/*.json: translated error messages
 */

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

// PASSWORD_IS_SECURE is the marker written to plaintext password fields after
// successful encryption. Any other value in a `<Name>Password` field is treated
// as a new plaintext password and will be encrypted and replaced by this marker.
var PASSWORD_IS_SECURE string

// PASSWORD_IS_SECURE_en is the English variant of the marker that is recognized
// when deciding whether a password field already contains an encrypted value.
var PASSWORD_IS_SECURE_en string

// PASSWORD_IS_SECURE_de is the German variant of the marker that is recognized
// when deciding whether a password field already contains an encrypted value.
var PASSWORD_IS_SECURE_de string

// apiMu serializes LoadConfig, UpdateConfig, DebugHardwareID and ResetForTest,
// which share the package state (keys, markers, debug flags).
var apiMu sync.Mutex

// Debug tracking: when debugOutput is true, the legacy hardware ID and its
// identifiers are logged to sconfig.debug.txt.
var debugMode bool
var lastDebugHardwareID uint64
var lastDebugIdentifiers string

const debugLogFilename = "sconfig.debug.txt"

// executableRootForTest, if set, replaces the directory of os.Executable() when
// resolving config paths (tests only).
var executableRootForTest string

// SetExecutableRootForTest pins the directory under which LoadConfig/UpdateConfig
// paths must lie. Pass an empty string to clear. For tests only; production uses
// the real executable directory.
func SetExecutableRootForTest(dir string) {
	executableRootForTest = dir
}

// homeDirForTest, if set, replaces the user's home directory when resolving
// config paths (tests only).
var homeDirForTest string

// SetHomeDirForTest pins the home directory under which LoadConfig/UpdateConfig
// paths may lie. Pass an empty string to clear. For tests only; production uses
// os.UserHomeDir() (%USERPROFILE% on Windows, $HOME on Unix).
func SetHomeDirForTest(dir string) {
	homeDirForTest = dir
}

// getHomeDirForConfigPaths returns the user's home directory, or "" if it
// cannot be determined (then the home directory is simply not an allowed base).
func getHomeDirForConfigPaths() string {
	if homeDirForTest != "" {
		return homeDirForTest
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func getExecutableDirForConfigPaths() (string, error) {
	if executableRootForTest != "" {
		return filepath.Abs(executableRootForTest)
	}
	return getExecutableDir()
}

// maxLinkDepth limits nested link resolution (protects against link loops).
const maxLinkDepth = 40

// resolveLinks resolves all symbolic links of an existing path. On Windows,
// filepath.EvalSymlinks does not follow junctions (mount points) since Go
// 1.23, so they are resolved component by component with os.Readlink.
func resolveLinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || runtime.GOOS != "windows" {
		return resolved, err
	}
	return resolveJunctions(resolved, 0)
}

// resolveJunctions follows Windows junctions and remaining reparse points in
// every component of an existing absolute path.
func resolveJunctions(path string, depth int) (string, error) {
	if depth > maxLinkDepth {
		return "", fmt.Errorf("too many levels of links: %s", path)
	}
	volume := filepath.VolumeName(path)
	parts := strings.Split(strings.Trim(path[len(volume):], `\/`), string(filepath.Separator))
	current := volume + string(filepath.Separator)
	for i, part := range parts {
		if part == "" {
			continue
		}
		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if err != nil {
			return "", err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			if target, err := os.Readlink(next); err == nil {
				if !filepath.IsAbs(target) {
					target = filepath.Join(current, target)
				}
				return resolveJunctions(filepath.Join(append([]string{target}, parts[i+1:]...)...), depth+1)
			}
		}
		current = next
	}
	return current, nil
}

// resolveSymlinks resolves symbolic links (and junctions) of the longest
// existing prefix of an absolute path and appends the not yet existing rest.
func resolveSymlinks(abs string) string {
	current := filepath.Clean(abs)
	rest := ""
	for {
		if resolved, err := resolveLinks(current); err == nil {
			if rest == "" {
				return resolved
			}
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(abs)
		}
		if rest == "" {
			rest = filepath.Base(current)
		} else {
			rest = filepath.Join(filepath.Base(current), rest)
		}
		current = parent
	}
}

// pathUnderBase reports whether target is the same path as base or a path
// inside base (no path traversal above base). Symbolic links are resolved on
// both sides, so a link inside base that points outside is rejected.
func pathUnderBase(base, target string) (bool, error) {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return false, err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(resolveSymlinks(baseAbs), resolveSymlinks(targetAbs))
	if err != nil {
		// e.g. different volume letters on Windows — not under base
		return false, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}

// resolveConfigPath cleans the path, resolves it to an absolute path, and
// ensures it lies under the executable directory (or executableRootForTest in
// tests), under the process current working directory, or under the user's
// home directory (%USERPROFILE% on Windows, ~ on Unix, e.g. for
// AppData\Roaming or ~/.config). Relative paths are resolved with filepath.Abs
// (i.e. relative to the CWD), matching usual CLI expectations.
func resolveConfigPath(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("%s", t("config.path_invalid", err))
	}
	exeBase, err := getExecutableDirForConfigPaths()
	if err != nil {
		return "", fmt.Errorf("%s", t("config.path_invalid", err))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%s", t("config.path_invalid", err))
	}
	bases := []string{exeBase, cwd}
	if home := getHomeDirForConfigPaths(); home != "" {
		bases = append(bases, home)
	}
	for _, base := range bases {
		under, err := pathUnderBase(base, abs)
		if err != nil {
			return "", fmt.Errorf("%s", t("config.path_invalid", err))
		}
		if under {
			return abs, nil
		}
	}
	return "", fmt.Errorf("%s", t("config.path_outside_executable", abs))
}

// getExecutableDir returns the directory of the running executable. It is used
// for path resolution and to place sconfig.debug.txt next to the binary.
func getExecutableDir() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

// writeDebugLog appends one line to sconfig.debug.txt in the executable
// directory. Line format: YYYY-MM-DD HH:MM:SS<TAB>hardwareID (hex)<TAB>identifiers.
// Only used when debug mode is on.
func writeDebugLog(hardwareID uint64, identifiers string, onlyOnInit bool) {
	dir, err := getExecutableDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] cannot get executable dir for debug log: %v\n", err)
		return
	}
	path := filepath.Join(dir, debugLogFilename)
	if onlyOnInit {
		if _, err := os.Stat(path); err == nil {
			return // the file already exists
		}
	}
	line := time.Now().Format("2006-01-02 15:04:05") + "\t" + fmt.Sprintf("0x%016x", hardwareID) + "\t" + identifiers + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] cannot open debug log %s: %v\n", path, err)
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}

// DebugHardwareID computes the legacy hardware ID (v1) and the machine
// identity (v2) and prints all intermediate results to stderr (VM detection,
// MAC source, identifiers, hash, v2 sources). Run it when passwords no longer
// decrypt and compare the output with a run on a working system. It returns
// the legacy v1 hardware ID.
func DebugHardwareID() (uint64, error) {
	apiMu.Lock()
	defer apiMu.Unlock()
	id, err := secure_config_getHardwareID_debug(true)
	sources, srcErr := collectMachineSourcesV2()
	printMachineSourcesV2(sources, srcErr)
	return id, err
}

// LoadConfig reads a JSON configuration file into the provided struct, applies
// default values from struct tags, synchronizes an optional top-level
// `Version` field, and manages password encryption/decryption.
//
// Config path: `path` is cleaned and must lie under the directory of the
// executable, under the current working directory or under the user's home
// directory (%USERPROFILE% on Windows, ~ on Unix). Relative paths are resolved
// against the CWD (filepath.Abs); symbolic links are resolved for the check.
//
// Behavior:
//   - If the file does not exist, an empty configuration is assumed.
//   - `default:"…"` tags are applied to all reachable structs, including
//     slice/array/map elements and pointer targets loaded from the file.
//   - Fields named `<Name>Password` and `<Name>SecurePassword` are treated as a
//     pair (in nested structs, pointers, slices, arrays and maps as well). If
//     the plaintext password differs from the recognized marker, it is
//     encrypted into `<Name>SecurePassword` (v2 format) and the plaintext
//     field is replaced by the marker string. Legacy v1 values are migrated
//     to v2 automatically.
//   - When `cleanConfig` is true, the function writes back a config file where
//     passwords are present as plaintext (use with care), primarily for
//     migration or inspection purposes.
//   - On successful completion and when `cleanConfig` is false, passwords are
//     decrypted in memory so callers can use the plaintext values directly.
//   - The file is written atomically; new files get permissions 0600.
//   - `debugOutput` enables detailed diagnostics (hardware ID, keys, paths) on
//     stderr. Only useful when an error occurred and the used data is needed
//     for the analysis; keep it off in normal operation (sensitive data).
//
// The optional `getHardwareID_func` replaces the machine identity (intended
// for tests): its value is used as legacy hardware ID and as the only v2
// source "override". The key state is set by the first call and kept until
// ResetForTest.
func LoadConfig(config interface{}, version int, path string, cleanConfig bool, debugOutput bool, getHardwareID_func ...func() (uint64, error)) error {
	apiMu.Lock()
	defer apiMu.Unlock()

	path, err := resolveConfigPath(path)
	if err != nil {
		return err
	}
	configValue, err := configStruct(config)
	if err != nil {
		return err
	}

	var hardwareIDFunc func() (uint64, error)
	if len(getHardwareID_func) > 0 && getHardwareID_func[0] != nil {
		hardwareIDFunc = getHardwareID_func[0]
	}
	config_init(hardwareIDFunc, debugOutput)

	writeMode := configFileMode(path)
	file := []byte("{}")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		file, err = readFileLimited(path, maxConfigFileSize)
		if err != nil {
			return fmt.Errorf(t("config.read_failed"), err)
		}
		if debugOutput {
			fmt.Fprintf(os.Stderr, "%s %s\n", t("config.debug_file_abs_path"), path)
		}
	}

	if err := applyDefaults(configValue); err != nil {
		return fmt.Errorf(t("config.failed_defaulting"), err)
	}
	if err := json.Unmarshal(file, config); err != nil {
		return fmt.Errorf(t("config.failed_parsing"), err)
	}
	if err := applyDecodedDefaults(configValue, file); err != nil {
		return fmt.Errorf(t("config.failed_defaulting"), err)
	}

	changed := false
	if err := syncVersionField(configValue, version, &changed); err != nil {
		return fmt.Errorf(t("config.failed_checking"), err)
	}
	if err := encryptPasswords(configValue, &changed); err != nil {
		return fmt.Errorf(t("config.failed_checking"), err)
	}
	if cleanConfig {
		// Decrypt passwords before writing
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
		changed = true
	}
	if changed {
		if err := writeConfigFile(config, path, writeMode); err != nil {
			return err
		}
	}
	if !cleanConfig {
		// Decrypt passwords after writing
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
	}
	return nil
}

// UpdateConfig writes the config struct to the given path. Secure password
// fields are encrypted in the file unless cleanConfig is true (then plaintext).
// LoadConfig must have been called at least once before; otherwise UpdateConfig
// returns an error. The path may differ from the one used in LoadConfig (e.g.
// write to a backup file) but must likewise lie under the executable directory,
// the current working directory or the user's home directory. The `Version`
// field is written as it is.
// Example: after the user changes the theme from "dark" to "light" in the UI,
// set cfg.Theme = "light" and call UpdateConfig(cfg, "config.json").
func UpdateConfig(config interface{}, path string, cleanConfig ...bool) error {
	apiMu.Lock()
	defer apiMu.Unlock()

	path, err := resolveConfigPath(path)
	if err != nil {
		return err
	}
	cleanConfigVal := len(cleanConfig) > 0 && cleanConfig[0]
	if !keys.initialized {
		return fmt.Errorf("%s", t("config.load_first"))
	}
	configValue, err := configStruct(config)
	if err != nil {
		return err
	}
	writeMode := configFileMode(path)
	if cleanConfigVal {
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
	} else {
		changed := false
		if err := encryptPasswords(configValue, &changed); err != nil {
			return err
		}
	}
	if err := writeConfigFile(config, path, writeMode); err != nil {
		return err
	}
	if !cleanConfigVal {
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
	}
	return nil
}

// configStruct returns the struct that config points to, or an error if
// config is not a pointer to a struct.
func configStruct(config interface{}) (reflect.Value, error) {
	v := reflect.ValueOf(config)
	if v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%s", t("config.config_no_struct"))
	}
	return v.Elem(), nil
}

// writeConfigFile marshals config and writes it atomically to path.
func writeConfigFile(config interface{}, path string, mode os.FileMode) error {
	configJSON, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return fmt.Errorf(t("config.failed_build_json"), err)
	}
	if err := writeFileAtomic(path, configJSON, mode); err != nil {
		return fmt.Errorf(t("config.failed_writing"), path, err)
	}
	return nil
}

// config_init prepares the key state and the password markers on the first
// call (until ResetForTest). Keys are derived lazily on first use, so errors
// of the machine identity are returned by the operation that needs a key.
//
// Design goal: the same machine must always derive the same key
// (deterministically from the machine identity). The aim is not a random
// one-time secret but a key that only this machine can reproduce, so that
// other systems cannot decrypt the values.
func config_init(getHardwareID_func func() (uint64, error), debugOutput bool) {
	debugMode = debugOutput
	if keys.initialized {
		return
	}
	resetKeys()
	keys.initialized = true
	if getHardwareID_func != nil {
		keys.hardwareIDFunc = getHardwareID_func
		keys.overridden = true
	} else {
		keys.hardwareIDFunc = func() (uint64, error) {
			return secure_config_getHardwareID_debug(debugMode)
		}
	}
	PASSWORD_IS_SECURE_de = markerText("de")
	PASSWORD_IS_SECURE_en = markerText("en")
	PASSWORD_IS_SECURE = markerText(getCurrentLanguage())
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Password secure marker: %s\n", PASSWORD_IS_SECURE)
	}
}

// ResetForTest clears the package state so the next LoadConfig derives the
// keys again from the given hardware-ID function. For tests only.
func ResetForTest() {
	apiMu.Lock()
	defer apiMu.Unlock()
	resetKeys()
}

func isPasswordMarker(value string) bool {
	if value == "" {
		return false
	}
	return value == PASSWORD_IS_SECURE ||
		value == PASSWORD_IS_SECURE_de ||
		value == PASSWORD_IS_SECURE_en
}

func passwordFieldLabel(prefix string) string {
	if prefix == "" {
		return t("config.unknown_password_field")
	}
	return prefix
}

func passwordPairError(prefix, messageKey string, detail ...interface{}) error {
	fieldName := passwordFieldLabel(prefix)
	if len(detail) == 0 {
		return fmt.Errorf("%s", t(messageKey, fieldName))
	}
	return fmt.Errorf("%s", t(messageKey, fieldName, detail[0]))
}

// syncVersionField sets the top-level field `Version` (also when promoted from
// an embedded struct) to version. Nested structs keep their own Version.
func syncVersionField(v reflect.Value, version int, changed *bool) error {
	field, ok := v.Type().FieldByName("Version")
	if !ok || !field.IsExported() {
		return nil
	}
	fieldValue, err := v.FieldByIndexErr(field.Index)
	if err != nil {
		return nil // embedded nil pointer: no Version field to sync
	}
	if !fieldValue.CanSet() {
		return fmt.Errorf("%s", t("config.field_not_settable", "Version"))
	}
	switch fieldValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fieldValue.Int() != int64(version) {
			fieldValue.SetInt(int64(version))
			*changed = true
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if fieldValue.Uint() != uint64(version) {
			fieldValue.SetUint(uint64(version))
			*changed = true
		}
	default:
		return fmt.Errorf("%s", t("config.version_type_unsupported", fieldValue.Kind()))
	}
	return nil
}
