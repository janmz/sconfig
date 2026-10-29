package sconfig

/*
 * Legacy hardware ID ("v1"), FROZEN.
 *
 * Encrypted values without the "v2:" prefix were encrypted with a key derived
 * from this hardware ID. The derivation must never change, otherwise existing
 * config files become unreadable. It is only used to decrypt such legacy
 * values (they are re-encrypted with v2 on the next LoadConfig).
 *
 * External programs are started with absolute paths (since 3.0.0; before,
 * they were looked up via PATH and, for "cmd /C ...", the current directory).
 * The output and therefore the ID are the same as long as the tool was found
 * before.
 *
 * Known weaknesses (see README "How the hardware ID is created"): the result
 * depends on volatile data (active network adapter), on installed tools
 * (wmic) and on process rights, and deriveKeyV1 has only about 2^31 distinct
 * keys. Changes in this file are limited to refactorings that provably keep
 * the result identical (Known-Answer-Tests in sconfig_test.go).
 */

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"slices"
	"strings"
)

// wmicPath returns the absolute path of wmic.exe (System32\wbem).
func wmicPath() string {
	return winTool(`wbem\WMIC.exe`)
}

// v1Runner runs the external commands of the legacy hardware-ID collection and
// remembers the first timeout, so that a hanging command yields an error
// instead of a silently different key.
type v1Runner struct {
	timeoutErr error
}

func (r *v1Runner) output(name string, args ...string) ([]byte, error) {
	out, err := runCommand(name, args...)
	if err != nil && errors.Is(err, errCommandTimeout) && r.timeoutErr == nil {
		r.timeoutErr = err
	}
	return out, err
}

// isVirtualMachineV1 checks whether the system runs on a virtual machine,
// using several detection methods (legacy v1 rules).
func isVirtualMachineV1(r *v1Runner) bool {
	if runtime.GOOS == "windows" {
		// Windows VM detection using WMI
		out, err := r.output(wmicPath(), "computersystem", "get", "Manufacturer,Model", "/value")
		if err == nil {
			manufacturer := ""
			model := ""
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "Manufacturer=") {
					manufacturer = strings.ToLower(strings.TrimSpace(line[13:]))
				} else if strings.HasPrefix(line, "Model=") {
					model = strings.ToLower(strings.TrimSpace(line[6:]))
				}
			}
			vmIndicators := []string{
				"vmware", "virtualbox", "microsoft corporation", "xen",
				"parallels", "qemu", "kvm", "innotek", "bochs",
			}
			for _, indicator := range vmIndicators {
				if strings.Contains(manufacturer, indicator) || strings.Contains(model, indicator) {
					return true
				}
			}
			// Hyper-V ("Virtual Machine" in the model). Note: physical Microsoft
			// devices (Surface) match "microsoft corporation" above and are
			// therefore treated as VMs; kept for compatibility.
			if strings.Contains(model, "virtual") {
				return true
			}
		}
		return false
	}

	if runtime.GOOS != "linux" {
		return false
	}

	// Method 1: systemd-detect-virt (most reliable)
	out, err := r.output(resolveTool("systemd-detect-virt", "/usr/bin/systemd-detect-virt", "/bin/systemd-detect-virt"))
	if err == nil {
		virt := strings.TrimSpace(string(out))
		// Returns "none" on bare metal, or the VM type (kvm, vmware, qemu, ...)
		if virt != "none" && virt != "" {
			return true
		}
	}

	// Method 2: DMI vendor/product
	checks := []string{
		"/sys/class/dmi/id/sys_vendor",
		"/sys/class/dmi/id/product_name",
		"/sys/class/dmi/id/chassis_vendor",
	}
	vmIndicators := []string{
		"qemu", "kvm", "vmware", "virtualbox", "xen",
		"parallels", "microsoft", "bochs", "bhyve",
	}
	for _, file := range checks {
		content, err := os.ReadFile(file)
		if err == nil {
			contentStr := strings.ToLower(strings.TrimSpace(string(content)))
			for _, indicator := range vmIndicators {
				if strings.Contains(contentStr, indicator) {
					return true
				}
			}
		}
	}
	return false
}

// macForInterfaceV1 returns the MAC address of the interface with the given
// name, or "" if it is not found or has no MAC.
func macForInterfaceV1(interfaces []net.Interface, ifaceName string, debugOutput bool) string {
	for _, iface := range interfaces {
		if iface.Name == ifaceName {
			if iface.HardwareAddr != nil && iface.HardwareAddr.String() != "" {
				mac := iface.HardwareAddr.String()
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found MAC from active interface '%s': %s\n", ifaceName, mac)
				}
				return mac
			}
		}
	}
	return ""
}

// activeMACWindowsV1 parses "ipconfig /all" and returns the MAC address of the
// last adapter block that has a default-gateway line (legacy v1 rules: the dot
// leaders make every gateway line count, only EN/DE labels are recognized).
func activeMACWindowsV1(r *v1Runner, debugOutput bool) string {
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Using ipconfig /all to find active adapter\n")
	}
	out, err := r.output(winTool("ipconfig.exe"), "/all")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	var currentAdapterName, adapterMAC, bestAdapterMAC, bestAdapterName string
	var hasGateway bool

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Adapter name (line ends with ":")
		if strings.HasSuffix(line, ":") && !strings.Contains(line, "Windows IP") && !strings.Contains(line, "Configuration") {
			// If the previous adapter had a gateway, keep it as candidate
			if hasGateway && currentAdapterName != "" && adapterMAC != "" {
				bestAdapterMAC = adapterMAC
				bestAdapterName = currentAdapterName
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found adapter with gateway: %s (MAC: %s)\n", currentAdapterName, adapterMAC)
				}
			}
			currentAdapterName = strings.TrimSuffix(line, ":")
			hasGateway = false
			adapterMAC = ""
		}

		// Physical address (MAC)
		if strings.HasPrefix(line, "Physical Address") || strings.HasPrefix(line, "Physische Adresse") || strings.HasPrefix(line, "Physikalische Adresse") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				adapterMAC = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(parts[1]), "-", ":"))
			}
		}

		// Default gateway
		if strings.HasPrefix(line, "Default Gateway") || strings.HasPrefix(line, "Standardgateway") {
			if strings.Contains(line, ".") || (strings.Contains(line, ":") && i+1 < len(lines) && strings.Contains(lines[i+1], ".")) {
				hasGateway = true
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Adapter %s has default gateway\n", currentAdapterName)
				}
			}
		}
	}

	// Last adapter
	if hasGateway && currentAdapterName != "" && adapterMAC != "" {
		bestAdapterMAC = adapterMAC
		bestAdapterName = currentAdapterName
		if debugOutput {
			fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Last adapter has gateway: %s (MAC: %s)\n", currentAdapterName, adapterMAC)
		}
	}
	if bestAdapterMAC != "" && debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Using MAC address from active adapter '%s': %s\n", bestAdapterName, bestAdapterMAC)
	}
	return bestAdapterMAC
}

// routeInterfaceV1 returns the interface name used for the route to 8.8.8.8
// (Linux: "ip route get", macOS: "route -n get").
func routeInterfaceV1(r *v1Runner) string {
	var out []byte
	var err error
	if runtime.GOOS == "linux" {
		out, err = r.output(resolveTool("ip", "/usr/sbin/ip", "/sbin/ip", "/usr/bin/ip", "/bin/ip"), "route", "get", "8.8.8.8")
	} else {
		out, err = r.output(resolveTool("route", "/sbin/route", "/usr/sbin/route"), "-n", "get", "8.8.8.8")
	}
	if err != nil {
		return ""
	}
	var ifaceName string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if runtime.GOOS == "linux" {
			// "8.8.8.8 via ... dev <interface> ..."; the last matching line wins
			if strings.Contains(line, "dev ") {
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "dev" && i+1 < len(parts) {
						ifaceName = parts[i+1]
						break
					}
				}
			}
		} else if strings.HasPrefix(line, "interface:") {
			// macOS: "interface: <interface>"; the first matching line wins
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ifaceName
}

// macAddressV1 returns the normalized MAC address used by the legacy hardware
// ID, or "" if none is available.
func macAddressV1(r *v1Runner, debugOutput bool) string {
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) == 0 {
		return ""
	}
	var macAddress string
	switch runtime.GOOS {
	case "windows":
		macAddress = activeMACWindowsV1(r, debugOutput)
	case "linux", "darwin":
		if ifaceName := routeInterfaceV1(r); ifaceName != "" {
			macAddress = macForInterfaceV1(interfaces, ifaceName, debugOutput)
		}
	}

	// Fallback: smallest MAC address (string order) of all interfaces
	if macAddress == "" {
		var macAddresses []string
		for _, iface := range interfaces {
			if iface.HardwareAddr != nil && iface.HardwareAddr.String() != "" {
				macAddresses = append(macAddresses, iface.HardwareAddr.String())
			}
		}
		if len(macAddresses) > 0 {
			macAddress = slices.Min(macAddresses)
			if debugOutput {
				fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Active interface not found, using first MAC (sorted): %s\n", macAddress)
			}
		}
	}
	if macAddress == "" {
		return ""
	}
	// Normalize: lower case, ":" as separator, no spaces
	macAddress = strings.ToLower(strings.ReplaceAll(macAddress, "-", ":"))
	macAddress = strings.ReplaceAll(macAddress, " ", "")
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Using MAC address (normalized): %s\n", macAddress)
	}
	return macAddress
}

// wmicColumnValues returns the trimmed, non-empty lines of a table-style wmic
// output after the header line.
func wmicColumnValues(out []byte, header string) []string {
	var values []string
	for i, line := range strings.Split(string(out), "\n") {
		if i == 0 {
			continue // header
		}
		value := strings.TrimSpace(line)
		if value != "" && value != header {
			values = append(values, value)
		}
	}
	return values
}

// windowsIdentifiersV1 collects the Windows-specific legacy identifiers.
func windowsIdentifiersV1(r *v1Runner, isVM bool, debugOutput bool) []string {
	var identifiers []string
	if isVM {
		// 1. MachineGuid from the registry
		if out, err := r.output(winTool("reg.exe"), "query", "HKLM\\SOFTWARE\\Microsoft\\Cryptography", "/v", "MachineGuid"); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "MachineGuid") {
					parts := strings.Fields(line)
					for i, part := range parts {
						if part == "REG_SZ" && i+1 < len(parts) {
							if machineGuid := strings.TrimSpace(parts[i+1]); machineGuid != "" {
								identifiers = append(identifiers, machineGuid)
								break
							}
						}
					}
				}
			}
		}
		// 2. SMBIOS UUID
		if out, err := r.output(wmicPath(), "csproduct", "get", "UUID", "/value"); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "UUID=") {
					if uuid := strings.TrimSpace(line[5:]); uuid != "" {
						uuid = strings.ReplaceAll(uuid, "_", "-")
						uuid = strings.TrimRight(uuid, ".! ")
						identifiers = append(identifiers, uuid)
						if debugOutput {
							fmt.Fprintf(os.Stderr, "[sconfig DEBUG] SMBIOS UUID (normalized): %s\n", uuid)
						}
						break
					}
				}
			}
		}
	}

	// Baseboard serial and product: second output line
	for _, cmdInfo := range []struct{ property, name string }{
		{"SerialNumber", "Baseboard SerialNumber"},
		{"Product", "Baseboard Product"},
	} {
		if out, err := r.output(wmicPath(), "baseboard", "get", cmdInfo.property); err == nil {
			lines := strings.Split(string(out), "\n")
			if len(lines) > 1 {
				if value := strings.TrimSpace(lines[1]); value != "" {
					identifiers = append(identifiers, value)
					if debugOutput {
						fmt.Fprintf(os.Stderr, "[sconfig DEBUG] %s: %s\n", cmdInfo.name, value)
					}
				}
			}
		}
	}

	// Disk serial: the LARGEST value in string order (includes USB drives)
	if out, err := r.output(wmicPath(), "diskdrive", "get", "SerialNumber"); err == nil {
		if diskSerials := wmicColumnValues(out, "SerialNumber"); len(diskSerials) > 0 {
			largest := slices.Max(diskSerials)
			identifiers = append(identifiers, largest)
			if debugOutput {
				fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Disk SerialNumbers found: %d, using largest: %s\n", len(diskSerials), largest)
			}
		}
	}

	// CPU ProcessorId (physical machines only): the smallest value
	if !isVM {
		if out, err := r.output(wmicPath(), "cpu", "get", "ProcessorId"); err == nil {
			if cpuIds := wmicColumnValues(out, "ProcessorId"); len(cpuIds) > 0 {
				smallest := slices.Min(cpuIds)
				identifiers = append(identifiers, smallest)
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] CPU ProcessorIds found: %d, using smallest: %s\n", len(cpuIds), smallest)
				}
			}
		}
	}
	return identifiers
}

// linuxIdentifiersV1 collects the Linux-specific legacy identifiers.
func linuxIdentifiersV1(r *v1Runner, isVM bool, debugOutput bool) []string {
	var identifiers []string
	if isVM {
		if machineID, err := os.ReadFile("/etc/machine-id"); err == nil {
			if s := strings.TrimSpace(string(machineID)); s != "" {
				identifiers = append(identifiers, s)
			}
		}
		if productUUID, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
			if s := strings.TrimSpace(string(productUUID)); s != "" {
				identifiers = append(identifiers, s)
			}
		}
	}

	// CPU serial (physical machines only): smallest unique value
	if !isVM {
		if out, err := r.output("/bin/sh", "-c", "cat /proc/cpuinfo | grep 'Serial'"); err == nil {
			var cpuSerials []string
			for _, line := range strings.Split(string(out), "\n") {
				// "Serial          : xxxxxxxx"
				if strings.Contains(line, "Serial") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						if value := strings.TrimSpace(parts[1]); value != "" {
							cpuSerials = append(cpuSerials, value)
						}
					}
				}
			}
			if len(cpuSerials) > 0 {
				unique := slices.Compact(slices.Sorted(slices.Values(cpuSerials)))
				identifiers = append(identifiers, unique[0])
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] CPU Serial numbers found: %d (unique: %d), using first (sorted): %s\n", len(cpuSerials), len(unique), unique[0])
				}
			}
		}
	}

	if out, err := r.output("/bin/sh", "-c", "cat /sys/class/dmi/id/board_serial"); err == nil {
		if value := strings.TrimSpace(string(out)); value != "" {
			identifiers = append(identifiers, value)
		}
	}
	return identifiers
}

// hardwareIDFromIdentifiersV1 sorts the identifiers, joins them with "|",
// hashes them with SHA-256 and returns the first 8 hash bytes read as
// little-endian uint64, plus the combined string (for debug output).
func hardwareIDFromIdentifiersV1(identifiers []string) (uint64, string) {
	sorted := slices.Clone(identifiers)
	slices.Sort(sorted)
	combined := strings.Join(sorted, "|")
	hash := sha256.Sum256([]byte(combined))
	var id uint64
	for i := 7; i >= 0; i-- {
		id = id<<8 | uint64(hash[i])
	}
	return id, combined
}

// deriveKeyV1 expands the legacy hardware ID into the 32-byte AES key with the
// frozen Go-1.23 math/rand source (key_rand_go123.go). The source reduces the
// seed to seed % (2^31-1), so only about 2^31 distinct keys exist.
func deriveKeyV1(hardwareID uint64) []byte {
	keyRNG := newGo123KeySource(int64(hardwareID & 0x7fffffffffffffff))
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(keyRNG.Int63() >> 16 & 0xff)
	}
	return key
}

// secure_config_getHardwareID_debug computes the legacy (v1) hardware ID. With
// debugOutput all intermediate values are printed to stderr and the result is
// logged to sconfig.debug.txt.
func secure_config_getHardwareID_debug(debugOutput bool) (uint64, error) {
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] ========================================\n")
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] sconfig Version: %s\n", Version)
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] sconfig BuildTime: %s\n", BuildTime)
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Legacy hardware ID (v1)\n")
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] ========================================\n")
	}

	r := &v1Runner{}
	var identifiers []string
	isVM := isVirtualMachineV1(r)
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] VM detection: %v\n", isVM)
	}

	if mac := macAddressV1(r, debugOutput); mac != "" {
		identifiers = append(identifiers, mac)
	}
	switch runtime.GOOS {
	case "windows":
		identifiers = append(identifiers, windowsIdentifiersV1(r, isVM, debugOutput)...)
	case "linux":
		identifiers = append(identifiers, linuxIdentifiersV1(r, isVM, debugOutput)...)
	}

	if r.timeoutErr != nil {
		return 0, r.timeoutErr
	}
	if len(identifiers) == 0 {
		return 0, fmt.Errorf("no hardware identifiers found")
	}

	hardwareID, combined := hardwareIDFromIdentifiersV1(identifiers)
	if debugOutput {
		sorted := slices.Sorted(slices.Values(identifiers))
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Hardware identifiers found: %d (sorted)\n", len(sorted))
		for i, id := range sorted {
			fmt.Fprintf(os.Stderr, "[sconfig DEBUG]   Identifier %d: %s\n", i+1, id)
		}
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Combined identifiers: %s\n", combined)
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] SHA256 hash: %x\n", sha256.Sum256([]byte(combined)))
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Hardware ID (uint64): %d (0x%016x)\n", hardwareID, hardwareID)
		_ = os.Stderr.Sync() // flush so debug output is visible even if the process exits
		writeDebugLog(hardwareID, combined, true)
		lastDebugHardwareID = hardwareID
		lastDebugIdentifiers = combined
	}
	return hardwareID, nil
}
