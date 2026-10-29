package sconfig

/*
 * Machine identity "v2" – used for all newly encrypted values.
 *
 * Specification (identical in the Go and the PHP implementation, see
 * README "How the hardware ID is created" and the shared test vectors):
 *
 *   Sources (name = normalized value, only those that can be read):
 *     Windows: machineguid  HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid
 *                           (64-bit registry view)
 *              smbiosuuid   Win32_ComputerSystemProduct.UUID via PowerShell CIM
 *     Linux:   machineid    /etc/machine-id (fallback /var/lib/dbus/machine-id)
 *     macOS:   ioplatformuuid  IOPlatformUUID from ioreg
 *     FreeBSD: hostuuid     sysctl kern.hostuuid
 *     optional: keyfile     hex of a secret key file (SetKeyFile)
 *   Normalization: trimmed, lower case; UUIDs without braces and validated,
 *   well-known placeholder UUIDs are ignored.
 *   Input keying material: for every used source, sorted by name,
 *     "<name>=<value>\n"
 *   Key: HKDF-SHA256(ikm, salt "sconfig-v2", info "sconfig v2 aes-256-gcm"),
 *        32 bytes.
 *   AAD: "sconfig:" + password field prefix in lower case, only a-z and 0-9
 *   Stored value: "v2:<name>[,<name>...]:" + base64(nonce(12) | ciphertext | tag(16)),
 *     names strictly ascending, at most 8 names and 256 characters
 *
 * The source names are stored with each value. Decryption derives the key
 * from exactly these sources and reports a missing source as its own error
 * instead of failing with a generic "wrong key".
 */

import (
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

const (
	v2Prefix   = "v2:"
	v2KDFSalt  = "sconfig-v2"
	v2KDFInfo  = "sconfig v2 aes-256-gcm"
	v2KeyBytes = 32
	// v2AADPrefix starts the additional authenticated data (field binding).
	v2AADPrefix = "sconfig:"
	// v2MaxSourceNames and v2MaxNamesLength bound the source list of a value.
	v2MaxSourceNames = 8
	v2MaxNamesLength = 256

	sourceMachineGUID    = "machineguid"
	sourceSMBIOSUUID     = "smbiosuuid"
	sourceMachineID      = "machineid"
	sourceIOPlatformUUID = "ioplatformuuid"
	sourceHostUUID       = "hostuuid"
	sourceOverride       = "override"
	sourceKeyFile        = "keyfile"
)

var (
	uuidPattern        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	machineIDPattern   = regexp.MustCompile(`^[0-9a-f]{32}$`)
	sourceNamePattern  = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
	ioregUUIDPattern   = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)
	errNoMachineSource = errors.New("no machine identity source available")
)

// placeholderUUIDs are values that firmware vendors ship unchanged on many
// machines; they do not identify a machine and are ignored.
var placeholderUUIDs = map[string]bool{
	"00000000-0000-0000-0000-000000000000": true,
	"ffffffff-ffff-ffff-ffff-ffffffffffff": true,
	"03000200-0400-0500-0006-000700080009": true,
	"00020003-0004-0005-0006-000700080009": true,
}

// normalizeUUID trims, lower-cases and removes braces. It returns "" for
// malformed or placeholder UUIDs.
func normalizeUUID(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	v = strings.TrimSuffix(strings.TrimPrefix(v, "{"), "}")
	if !uuidPattern.MatchString(v) || placeholderUUIDs[v] {
		return ""
	}
	return v
}

// normalizeMachineID validates a systemd/dbus machine ID (32 hex digits).
func normalizeMachineID(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if !machineIDPattern.MatchString(v) || strings.Trim(v, "0") == "" {
		return ""
	}
	return v
}

// parseRegQueryValue extracts the REG_SZ data of valueName from "reg query"
// output.
func parseRegQueryValue(out, valueName string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], valueName) && fields[1] == "REG_SZ" {
			return strings.Join(fields[2:], " ")
		}
	}
	return ""
}

// parseIoregPlatformUUID extracts IOPlatformUUID from "ioreg" output.
func parseIoregPlatformUUID(out string) string {
	if m := ioregUUIDPattern.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// collectMachineSourcesV2 reads all v2 identity sources of the current system.
// Unreadable sources are left out; a timeout is returned as error.
func collectMachineSourcesV2() (map[string]string, error) {
	sources := map[string]string{}
	switch runtime.GOOS {
	case "windows":
		out, err := runCommand(winTool("reg.exe"), "query",
			`HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid", "/reg:64")
		if errors.Is(err, errCommandTimeout) {
			return nil, err
		}
		if err == nil {
			if v := normalizeUUID(parseRegQueryValue(string(out), "MachineGuid")); v != "" {
				sources[sourceMachineGUID] = v
			}
		}
		out, err = runCommandTimeout(powershellCommandTimeout,
			winTool(`WindowsPowerShell\v1.0\powershell.exe`),
			"-NoProfile", "-NonInteractive", "-Command",
			"(Get-CimInstance -ClassName Win32_ComputerSystemProduct).UUID")
		if errors.Is(err, errCommandTimeout) {
			return nil, err
		}
		if err == nil {
			if v := normalizeUUID(string(out)); v != "" {
				sources[sourceSMBIOSUUID] = v
			}
		}
	case "linux":
		for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
			if data, err := os.ReadFile(path); err == nil {
				if v := normalizeMachineID(string(data)); v != "" {
					sources[sourceMachineID] = v
					break
				}
			}
		}
	case "darwin":
		out, err := runCommand("/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
		if errors.Is(err, errCommandTimeout) {
			return nil, err
		}
		if err == nil {
			if v := normalizeUUID(parseIoregPlatformUUID(string(out))); v != "" {
				sources[sourceIOPlatformUUID] = v
			}
		}
	case "freebsd":
		out, err := runCommand("/sbin/sysctl", "-n", "kern.hostuuid")
		if errors.Is(err, errCommandTimeout) {
			return nil, err
		}
		if err == nil {
			if v := normalizeUUID(string(out)); v != "" {
				sources[sourceHostUUID] = v
			}
		}
	}
	return sources, nil
}

// sortedSourceNames returns the names of all sources in ascending order.
func sortedSourceNames(sources map[string]string) []string {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// deriveKeyV2 derives the AES-256 key from the given sources (all of them).
func deriveKeyV2(sources map[string]string) ([]byte, error) {
	if len(sources) == 0 {
		return nil, errNoMachineSource
	}
	var ikm strings.Builder
	for _, name := range sortedSourceNames(sources) {
		ikm.WriteString(name)
		ikm.WriteByte('=')
		ikm.WriteString(sources[name])
		ikm.WriteByte('\n')
	}
	return hkdf.Key(sha256.New, []byte(ikm.String()), []byte(v2KDFSalt), v2KDFInfo, v2KeyBytes)
}

// parseV2Value splits "v2:<names>:<base64>" into the source names and the
// base64 part.
func parseV2Value(value string) ([]string, string, error) {
	parts := strings.SplitN(strings.TrimPrefix(value, v2Prefix), ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return nil, "", fmt.Errorf("%w: malformed v2 value", errInvalidCiphertext)
	}
	if len(parts[0]) > v2MaxNamesLength {
		return nil, "", fmt.Errorf("%w: source list too long", errInvalidCiphertext)
	}
	names := strings.Split(parts[0], ",")
	if len(names) > v2MaxSourceNames {
		return nil, "", fmt.Errorf("%w: too many sources", errInvalidCiphertext)
	}
	for i, name := range names {
		if !sourceNamePattern.MatchString(name) {
			return nil, "", fmt.Errorf("%w: invalid source name %q", errInvalidCiphertext, name)
		}
		// canonical form: strictly ascending, no duplicates
		if i > 0 && names[i-1] >= name {
			return nil, "", fmt.Errorf("%w: source names not sorted or duplicated", errInvalidCiphertext)
		}
	}
	return names, parts[1], nil
}

// printMachineSourcesV2 writes the v2 sources to stderr (debug only).
func printMachineSourcesV2(sources map[string]string, err error) {
	fmt.Fprintf(os.Stderr, "[sconfig DEBUG] ========================================\n")
	fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Machine identity (v2)\n")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] v2 sources failed: %v\n", err)
		return
	}
	if len(sources) == 0 {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] v2 sources: none available\n")
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
