package sconfig

/*
 * Machine identity "v3" – used for all newly encrypted values since 3.1.0.
 *
 * Specification (identical in the Go and the PHP implementation, see
 * README "How the hardware ID is created" and the shared test vectors):
 *
 *   Sources: all v2 sources (machineguid, smbiosuuid, machineid,
 *   ioplatformuuid, hostuuid, optional keyfile) plus
 *     hostname  host name of the operating system (Go os.Hostname, PHP
 *               gethostname): trimmed, ASCII lower case, 1-255 printable
 *               ASCII characters; "localhost" and "localhost.localdomain"
 *               are ignored
 *     machtype  native machine architecture, normalized (amd64, arm64, 386,
 *               arm or the raw value, a-z0-9_ only):
 *                 Windows: PROCESSOR_ARCHITECTURE of the system environment
 *                          (registry, 64-bit view; not the process value)
 *                 macOS:   "arm64" if sysctl hw.optional.arm64 is 1 (also
 *                          under Rosetta), otherwise uname -m
 *                 others:  uname -m
 *     fsid      ID of the file system that holds the config file (or its
 *               directory if the file does not exist yet), lower-case hex
 *               without leading zeros, 0 is ignored:
 *                 Windows: volume serial number
 *                 Linux:   statfs f_fsid as val[0]<<32 | val[1]
 *                          (= "stat -f -c %i", GNU coreutils and busybox)
 *                 others:  st_dev of stat() (macOS: lower 32 bits)
 *   With a hardware-ID override (tests) only "override" (and keyfile) is used.
 *   Input keying material, key size, AAD and value layout as in v2, but
 *     salt "sconfig-v3", info "sconfig v3 aes-256-gcm" and the prefix "v3:".
 *
 * hostname and machtype are no secrets for someone with access to the
 * machine, but an attacker who only has the config file must know them, and
 * they separate cloned VMs that share machine-id or SMBIOS UUID. fsid binds
 * the values to the file system of the config file.
 */

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const (
	v3Prefix  = "v3:"
	v3KDFSalt = "sconfig-v3"
	v3KDFInfo = "sconfig v3 aes-256-gcm"

	sourceHostname = "hostname"
	sourceMachType = "machtype"
	sourceFSID     = "fsid"

	// maxHostnameLength is the maximum length of a DNS name.
	maxHostnameLength = 255
	// identityTrimSet are the characters trimmed from source values (the same
	// set as PHP trim()).
	identityTrimSet = " \t\n\r\x00\x0b"
)

var (
	machTypePattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
	fsidHexPattern  = regexp.MustCompile(`^[0-9a-f]{1,16}$`)
)

// placeholderHostnames do not identify a machine and are ignored.
var placeholderHostnames = map[string]bool{
	"localhost":             true,
	"localhost.localdomain": true,
}

// asciiLower lower-cases A-Z only (like PHP strtolower since PHP 8.2), so
// that Go and PHP normalize non-ASCII input identically.
func asciiLower(value string) string {
	b := []byte(value)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// normalizeHostname trims and lower-cases a host name. It returns "" for
// empty, too long, non-printable or placeholder names.
func normalizeHostname(value string) string {
	v := asciiLower(strings.Trim(value, identityTrimSet))
	if v == "" || len(v) > maxHostnameLength || placeholderHostnames[v] {
		return ""
	}
	for i := 0; i < len(v); i++ {
		if v[i] < 0x21 || v[i] > 0x7e {
			return ""
		}
	}
	return v
}

// normalizeMachType maps the architecture names of the different systems to
// one canonical name. It returns "" for invalid or unknown values.
func normalizeMachType(value string) string {
	v := asciiLower(strings.Trim(value, identityTrimSet))
	switch v {
	case "x86_64", "amd64", "x64", "em64t":
		return "amd64"
	case "aarch64", "arm64", "aarch64_be":
		return "arm64"
	case "i386", "i486", "i586", "i686", "x86", "i86pc":
		return "386"
	case "unknown":
		return ""
	}
	if v == "arm" || strings.HasPrefix(v, "armv") {
		return "arm"
	}
	if !machTypePattern.MatchString(v) {
		return ""
	}
	return v
}

// normalizeFSID validates a hex file system ID and removes leading zeros.
// It returns "" for invalid values and for 0.
func normalizeFSID(value string) string {
	v := asciiLower(strings.Trim(value, identityTrimSet))
	if !fsidHexPattern.MatchString(v) {
		return ""
	}
	return strings.TrimLeft(v, "0")
}

// formatFSID formats a numeric file system ID like normalizeFSID.
func formatFSID(id uint64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatUint(id, 16)
}

// hostnameFunc and fileSystemIDFunc can be replaced by tests.
var (
	hostnameFunc     = os.Hostname
	fileSystemIDFunc = fileSystemID
)

// readHostname returns the normalized host name or "".
func readHostname() string {
	name, err := hostnameFunc()
	if err != nil {
		return ""
	}
	return normalizeHostname(name)
}

// readMachType returns the normalized native machine architecture or "".
// A timeout of an external command is returned as error.
func readMachType() (string, error) {
	switch runtime.GOOS {
	case "windows":
		out, err := runCommand(winTool("reg.exe"), "query",
			`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`,
			"/v", "PROCESSOR_ARCHITECTURE", "/reg:64")
		if err != nil {
			return "", ignoreUnlessTimeout(err)
		}
		return normalizeMachType(parseRegQueryValue(string(out), "PROCESSOR_ARCHITECTURE")), nil
	case "darwin":
		out, err := runCommand("/usr/sbin/sysctl", "-n", "hw.optional.arm64")
		if errors.Is(err, errCommandTimeout) {
			return "", err
		}
		if err == nil && strings.Trim(string(out), identityTrimSet) == "1" {
			return "arm64", nil
		}
	}
	out, err := runCommand(resolveTool("uname", "/usr/bin/uname", "/bin/uname"), "-m")
	if err != nil {
		return "", ignoreUnlessTimeout(err)
	}
	return normalizeMachType(string(out)), nil
}

// ignoreUnlessTimeout drops errors of unreadable sources but keeps timeouts.
func ignoreUnlessTimeout(err error) error {
	if errors.Is(err, errCommandTimeout) {
		return err
	}
	return nil
}

// collectMachineSourcesV3 reads the machine-wide v3 sources (v2 sources plus
// hostname and machtype). fsid depends on the config file and is added by
// the key state.
func collectMachineSourcesV3() (map[string]string, error) {
	sources, err := collectMachineSourcesV2()
	if err != nil {
		return nil, err
	}
	if v := readHostname(); v != "" {
		sources[sourceHostname] = v
	}
	machType, err := readMachType()
	if err != nil {
		return nil, err
	}
	if machType != "" {
		sources[sourceMachType] = machType
	}
	return sources, nil
}

// existingPathFor returns path itself or its nearest existing parent
// directory (a config file that does not exist yet is created there).
func existingPathFor(path string) string {
	current := filepath.Clean(path)
	for {
		if _, err := os.Stat(current); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current
		}
		current = parent
	}
}

// configFileSystemID returns the normalized fsid of the file system that
// holds path (or its nearest existing parent), or "" if it cannot be read.
func configFileSystemID(path string) string {
	id, err := fileSystemIDFunc(existingPathFor(path))
	if err != nil {
		return ""
	}
	return formatFSID(id)
}

// deriveKeyV3 derives the AES-256 key from the given sources (all of them).
func deriveKeyV3(sources map[string]string) ([]byte, error) {
	return deriveKeyHKDF(sources, v3KDFSalt, v3KDFInfo)
}

// parseV3Value splits "v3:<names>:<base64>" into the source names and the
// base64 part.
func parseV3Value(value string) ([]string, string, error) {
	return parseSourceValue(value, v3Prefix)
}

// printMachineSources writes the identity sources of the given format
// version ("v3") to stderr (debug only). The key file value is not shown.
func printMachineSources(version string, sources map[string]string, err error) {
	fmt.Fprintf(os.Stderr, "[sconfig DEBUG] ========================================\n")
	fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Machine identity (%s)\n", version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] %s sources failed: %v\n", version, err)
		return
	}
	if len(sources) == 0 {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] %s sources: none available\n", version)
		return
	}
	for _, name := range sortedSourceNames(sources) {
		value := sources[name]
		if name == sourceKeyFile {
			value = "(secret, not shown)"
		}
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG]   %s=%s\n", name, value)
	}
}
