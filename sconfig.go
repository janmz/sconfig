package sconfig

/*
 * Description: This package contains a function for managing config files with secure passwords.
 *
 * Version: 1.2.8.17 (in version.go zu ändern)
 *
 * ChangeLog:
 *  03.12.25	1.2.8	fix: use biggesr disk id
 *  03.12.25	1.2.7	fix: use biggesr disk id
 *  03.12.25	1.2.6	fix: use ipconfig for mac address
 *  03.12.25	1.2.5	fix: use route table for mac address
 *  02.12.25	1.2.3	fix: using volatile information on VM has voided the hardware id
 *  24.11.25	1.2.2	fixed missing password replacements
 * 24.11.25	1.2.1	fixed wrong composer settings and documentation
 * 24.11.25	1.2.0	included PHP variant
 *
 * Author: Jan Neuhaus, VAYA Consulting, https://vaya-consultig.de/development/ https://github.com/janmz
 *
 * Functions:
 * - LoadConfig(): Loads the configuration from a file and processes it, it may rewrite it to encode passwords.
 *
 * Dependencies:
 * - i18n.go and locales/*.json: For internationalization of error messages
 */

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathRand "math/rand"
	"net"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"crypto/aes"    // AES Encryption
	"crypto/cipher" // Cipher for GCM
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64" // Base64 Encoding
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

var encryptionKey []byte
var initialized = false

/*
 * Check if the system is running on a virtual machine
 * Uses multiple detection methods for reliability
 */
func isVirtualMachine() bool {
	if runtime.GOOS == "windows" {
		// Windows VM detection using WMI
		out, err := exec.Command("wmic", "computersystem", "get", "Manufacturer,Model", "/value").Output()
		if err == nil {
			manufacturer := ""
			model := ""
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
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

			// Check for Hyper-V (Virtual Machine in Model)
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
	out, err := exec.Command("systemd-detect-virt").Output()
	if err == nil {
		virt := strings.TrimSpace(string(out))
		// Returns "none" on bare metal, or VM type (kvm, vmware, qemu, etc.)
		if virt != "none" && virt != "" {
			return true
		}
	}

	// Method 2: Check DMI vendor/product
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

/*
 * getActiveNetworkInterface finds the network interface that has the active internet connection
 * by checking the default route. Returns the interface name or empty string if not found.
 */
func getActiveNetworkInterface(debugOutput bool) string {
	switch runtime.GOOS {
	case "windows":
		// On Windows, use "route print" to find the interface with default route (0.0.0.0)
		// First, get the interface index from the route table
		out, err := exec.Command("cmd", "/C", "route print 0.0.0.0").Output()
		if err == nil {
			output := string(out)
			lines := strings.Split(output, "\n")
			var interfaceIndex string
			// Look for the active route line (0.0.0.0 with gateway)
			for _, line := range lines {
				line = strings.TrimSpace(line)
				fields := strings.Fields(line)
				// Format: "0.0.0.0 0.0.0.0 <gateway> <gateway> <metric> <interface_index>"
				if len(fields) >= 6 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" && strings.Contains(fields[2], ".") {
					interfaceIndex = fields[len(fields)-1]
					break
				}
			}

			if interfaceIndex != "" {
				// Now find the interface name by index using "netsh interface show interface"
				out2, err2 := exec.Command("cmd", "/C", "netsh interface show interface").Output()
				if err2 == nil {
					lines2 := strings.Split(string(out2), "\n")
					for _, line := range lines2 {
						fields := strings.Fields(line)
						// Format: "Enabled/Disabled <connection_type> <interface_name>"
						// Or we can use wmic to get interface by index
						if len(fields) >= 3 {
							// Try to match by checking if this line contains relevant info
						}
					}
				}

				// Alternative: use wmic to get interface name by index
				cmd := fmt.Sprintf("wmic path Win32_NetworkAdapter where \"InterfaceIndex=%s\" get Name", interfaceIndex)
				out3, err3 := exec.Command("cmd", "/C", cmd).Output()
				if err3 == nil {
					lines3 := strings.Split(string(out3), "\n")
					for _, line := range lines3 {
						line = strings.TrimSpace(line)
						if line != "" && line != "Name" && !strings.HasPrefix(line, "---") {
							if debugOutput {
								fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found active network adapter (index %s): %s\n", interfaceIndex, line)
							}
							return line
						}
					}
				}
			}
		}

		// Fallback: use ipconfig to find the adapter with default gateway
		out, err = exec.Command("cmd", "/C", "ipconfig").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			var currentAdapter string
			hasGateway := false
			for _, line := range lines {
				line = strings.TrimSpace(line)
				// Check for adapter name (ends with ":")
				if strings.HasSuffix(line, ":") && !strings.Contains(line, "Windows IP") && !strings.Contains(line, "Configuration") {
					// Reset when we find a new adapter
					if hasGateway && currentAdapter != "" {
						// We already found one with gateway, return it
						if debugOutput {
							fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found active network adapter: %s\n", currentAdapter)
						}
						return currentAdapter
					}
					currentAdapter = strings.TrimSuffix(line, ":")
					hasGateway = false
				}
				// Check for default gateway
				if strings.HasPrefix(line, "Default Gateway") && strings.Contains(line, ".") {
					hasGateway = true
				}
			}
			// Check if the last adapter had a gateway
			if hasGateway && currentAdapter != "" {
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found active network adapter: %s\n", currentAdapter)
				}
				return currentAdapter
			}
		}

	case "linux", "darwin":
		// On Linux/Mac, use "ip route get" or "route get" to find the interface for default route
		var cmd *exec.Cmd
		if runtime.GOOS == "linux" {
			cmd = exec.Command("ip", "route", "get", "8.8.8.8")
		} else {
			// macOS
			cmd = exec.Command("route", "-n", "get", "8.8.8.8")
		}
		out, err := cmd.Output()
		if err == nil {
			output := string(out)
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if runtime.GOOS == "linux" {
					// Linux "ip route" output format: "8.8.8.8 via ... dev <interface> ..."
					if strings.Contains(line, "dev ") {
						parts := strings.Fields(line)
						for i, part := range parts {
							if part == "dev" && i+1 < len(parts) {
								iface := parts[i+1]
								if debugOutput {
									fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found active network interface: %s\n", iface)
								}
								return iface
							}
						}
					}
				} else {
					// macOS "route get" output format: "interface: <interface>"
					if strings.HasPrefix(line, "interface:") {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							iface := parts[1]
							if debugOutput {
								fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found active network interface: %s\n", iface)
							}
							return iface
						}
					}
				}
			}
		}
	}

	return ""
}

/*
 * This function is a default function that can be overridden and generates a 64-bit number
 * that uniquely identifies a system in such a way that it is unlikely that someone can simply build
 * a second system that gets the identical ID. This allows system-specific keys to be generated
 * that are used for encrypting passwords in config files.
 * On virtual machines, prioritizes stable identifiers like machine-id and product_uuid.
 *
 */
func secure_config_getHardwareID() (uint64, error) {
	return secure_config_getHardwareID_debug(false)
}

func secure_config_getHardwareID_debug(debugOutput bool) (uint64, error) {
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] ========================================\n")
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] sconfig Version: %s\n", Version)
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] sconfig BuildTime: %s\n", BuildTime)
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] ========================================\n")
	}

	var identifiers []string
	isVM := isVirtualMachine()

	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] VM detection: %v\n", isVM)
	}

	// MAC address of the network interface with active internet connection
	// Get all interfaces first
	interfaces, err := net.Interfaces()
	if err == nil && len(interfaces) > 0 {
		var macAddress string

		// Try to find MAC address of the active interface by interface index
		switch runtime.GOOS {
		case "windows":
			// On Windows, use ipconfig /all to find the adapter with default gateway
			// This is more reliable than parsing route tables with varying formats
			if debugOutput {
				fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Using ipconfig /all to find active adapter\n")
			}
			out, err := exec.Command("cmd", "/C", "ipconfig /all").Output()
			if err == nil {
				output := string(out)
				lines := strings.Split(output, "\n")
				var currentAdapterName string
				var hasGateway bool
				var adapterMAC string
				var bestAdapterMAC string
				var bestAdapterName string

				for i, line := range lines {
					line = strings.TrimSpace(line)

					// Check for adapter name (ends with ":")
					if strings.HasSuffix(line, ":") && !strings.Contains(line, "Windows IP") && !strings.Contains(line, "Configuration") {
						// If previous adapter had gateway, save it as candidate
						if hasGateway && currentAdapterName != "" && adapterMAC != "" {
							bestAdapterMAC = adapterMAC
							bestAdapterName = currentAdapterName
							if debugOutput {
								fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found adapter with gateway: %s (MAC: %s)\n", currentAdapterName, adapterMAC)
							}
						}
						// Start new adapter
						currentAdapterName = strings.TrimSuffix(line, ":")
						hasGateway = false
						adapterMAC = ""
					}

					// Check for physical address (MAC)
					if strings.HasPrefix(line, "Physical Address") || strings.HasPrefix(line, "Physische Adresse") || strings.HasPrefix(line, "Physikalische Adresse") {
						parts := strings.Split(line, ":")
						if len(parts) >= 2 {
							adapterMAC = strings.TrimSpace(parts[1])
							// Normalize MAC address format
							adapterMAC = strings.ToLower(strings.ReplaceAll(adapterMAC, "-", ":"))
						}
					}

					// Check for default gateway
					if strings.HasPrefix(line, "Default Gateway") || strings.HasPrefix(line, "Standardgateway") {
						// Check if it contains an IP address (has dots)
						if strings.Contains(line, ".") || (strings.Contains(line, ":") && i < len(lines) && strings.Contains(lines[i+1], ".")) {
							hasGateway = true
							if debugOutput {
								fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Adapter %s has default gateway\n", currentAdapterName)
							}
						}
					}
				}

				// Check last adapter
				if hasGateway && currentAdapterName != "" && adapterMAC != "" {
					bestAdapterMAC = adapterMAC
					bestAdapterName = currentAdapterName
					if debugOutput {
						fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Last adapter has gateway: %s (MAC: %s)\n", currentAdapterName, adapterMAC)
					}
				}

				// Use the MAC address directly if found
				if bestAdapterMAC != "" {
					macAddress = bestAdapterMAC
					if debugOutput {
						fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Using MAC address from active adapter '%s': %s\n", bestAdapterName, macAddress)
					}
				} else {
					// Fallback: try to match adapter name with net.Interfaces()
					if bestAdapterName != "" {
						for _, iface := range interfaces {
							ifaceNameLower := strings.ToLower(iface.Name)
							adapterNameLower := strings.ToLower(bestAdapterName)
							if strings.Contains(adapterNameLower, ifaceNameLower) || strings.Contains(ifaceNameLower, adapterNameLower) {
								if iface.HardwareAddr != nil && iface.HardwareAddr.String() != "" {
									macAddress = iface.HardwareAddr.String()
									if debugOutput {
										fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Matched adapter name to interface, using MAC: %s\n", macAddress)
									}
									break
								}
							}
						}
					}
				}
			}

		case "linux":
			// On Linux, get interface name from route, then find MAC
			cmd := exec.Command("ip", "route", "get", "8.8.8.8")
			out, err := cmd.Output()
			if err == nil {
				output := string(out)
				lines := strings.Split(output, "\n")
				var ifaceName string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.Contains(line, "dev ") {
						parts := strings.Fields(line)
						for i, part := range parts {
							if part == "dev" && i+1 < len(parts) {
								ifaceName = parts[i+1]
								break
							}
						}
					}
				}
				if ifaceName != "" {
					for _, iface := range interfaces {
						if iface.Name == ifaceName {
							if iface.HardwareAddr != nil && iface.HardwareAddr.String() != "" {
								macAddress = iface.HardwareAddr.String()
								if debugOutput {
									fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found MAC from active interface '%s': %s\n", ifaceName, macAddress)
								}
								break
							}
						}
					}
				}
			}

		case "darwin":
			// On macOS, get interface name from route, then find MAC
			cmd := exec.Command("route", "-n", "get", "8.8.8.8")
			out, err := cmd.Output()
			if err == nil {
				output := string(out)
				lines := strings.Split(output, "\n")
				var ifaceName string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "interface:") {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							ifaceName = parts[1]
							break
						}
					}
				}
				if ifaceName != "" {
					for _, iface := range interfaces {
						if iface.Name == ifaceName {
							if iface.HardwareAddr != nil && iface.HardwareAddr.String() != "" {
								macAddress = iface.HardwareAddr.String()
								if debugOutput {
									fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Found MAC from active interface '%s': %s\n", ifaceName, macAddress)
								}
								break
							}
						}
					}
				}
			}
		}

		// Fallback: if we couldn't find the active interface, use the first available MAC address (sorted)
		if macAddress == "" {
			var macAddresses []string
			for _, iface := range interfaces {
				if iface.HardwareAddr != nil && iface.HardwareAddr.String() != "" {
					macAddresses = append(macAddresses, iface.HardwareAddr.String())
				}
			}
			// Sort to ensure consistent ordering
			if len(macAddresses) > 0 {
				for i := 0; i < len(macAddresses)-1; i++ {
					for j := i + 1; j < len(macAddresses); j++ {
						if macAddresses[i] > macAddresses[j] {
							macAddresses[i], macAddresses[j] = macAddresses[j], macAddresses[i]
						}
					}
				}
				macAddress = macAddresses[0]
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Active interface not found, using first MAC (sorted): %s\n", macAddress)
				}
			}
		}

		if macAddress != "" {
			// Normalize MAC address: convert to lowercase and ensure consistent format
			macAddress = strings.ToLower(strings.ReplaceAll(macAddress, "-", ":"))
			// Ensure it's in standard format (xx:xx:xx:xx:xx:xx)
			macAddress = strings.ReplaceAll(macAddress, " ", "")
			identifiers = append(identifiers, macAddress)
			if debugOutput {
				fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Using MAC address (normalized): %s\n", macAddress)
			}
		}
	}

	// CPU ID and other hardware information depending on the operating system
	switch runtime.GOOS {
	case "windows":
		if isVM {
			// For Windows VMs: prioritize stable identifiers
			// 1. MachineGuid from Registry (very stable on Windows)
			out, err := exec.Command("reg", "query", "HKLM\\SOFTWARE\\Microsoft\\Cryptography", "/v", "MachineGuid").Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					if strings.Contains(line, "MachineGuid") {
						parts := strings.Fields(line)
						for i, part := range parts {
							if part == "REG_SZ" && i+1 < len(parts) {
								machineGuid := strings.TrimSpace(parts[i+1])
								if machineGuid != "" {
									identifiers = append(identifiers, machineGuid)
									break
								}
							}
						}
					}
				}
			}

			// 2. SMBIOS UUID (usually stable on VMs)
			out, err = exec.Command("wmic", "csproduct", "get", "UUID", "/value").Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "UUID=") {
						uuid := strings.TrimSpace(line[5:])
						if uuid != "" {
							// Normalize UUID: replace underscores with hyphens for consistency
							uuid = strings.ReplaceAll(uuid, "_", "-")
							// Remove trailing dots or other characters
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

		// Common identifiers (for both VM and physical)
		// Windows-specific hardware IDs
		// Baseboard and Product (usually single values)
		// Parse them separately to handle empty values and ensure stability
		baseboardCmds := []struct {
			cmd  string
			name string
		}{
			{"wmic baseboard get SerialNumber", "Baseboard SerialNumber"},
			{"wmic baseboard get Product", "Baseboard Product"},
		}

		for _, cmdInfo := range baseboardCmds {
			out, err := exec.Command("cmd", "/C", cmdInfo.cmd).Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				if len(lines) > 1 {
					value := strings.TrimSpace(lines[1])
					if value != "" && value != cmdInfo.name {
						identifiers = append(identifiers, value)
						if debugOutput {
							fmt.Fprintf(os.Stderr, "[sconfig DEBUG] %s: %s\n", cmdInfo.name, value)
						}
					}
				}
			}
		}

		// Handle diskdrive SerialNumber separately to ensure stable ordering
		out, err := exec.Command("cmd", "/C", "wmic diskdrive get SerialNumber").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			var diskSerials []string
			for i, line := range lines {
				if i == 0 {
					continue // Skip header
				}
				value := strings.TrimSpace(line)
				if value != "" && value != "SerialNumber" {
					diskSerials = append(diskSerials, value)
				}
			}
			// Sort disk serials to ensure consistent ordering
			if len(diskSerials) > 0 {
				for i := 0; i < len(diskSerials)-1; i++ {
					for j := i + 1; j < len(diskSerials); j++ {
						if diskSerials[i] < diskSerials[j] {
							diskSerials[i], diskSerials[j] = diskSerials[j], diskSerials[i]
						}
					}
				}
				identifiers = append(identifiers, diskSerials[0])
				if debugOutput {
					fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Disk SerialNumbers found: %d, using first (sorted): %s\n", len(diskSerials), diskSerials[0])
				}
			}
		}

		// On VMs, skip CPU ProcessorId as it's often unreliable
		if !isVM {
			// For CPU ProcessorId, collect all values and use the first one (sorted) for stability
			out, err := exec.Command("cmd", "/C", "wmic cpu get ProcessorId").Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				var cpuIds []string
				for i, line := range lines {
					if i == 0 {
						continue // Skip header
					}
					value := strings.TrimSpace(line)
					if value != "" && value != "ProcessorId" {
						cpuIds = append(cpuIds, value)
					}
				}
				// Sort CPU IDs to ensure consistent ordering across runs
				if len(cpuIds) > 0 {
					for i := 0; i < len(cpuIds)-1; i++ {
						for j := i + 1; j < len(cpuIds); j++ {
							if cpuIds[i] > cpuIds[j] {
								cpuIds[i], cpuIds[j] = cpuIds[j], cpuIds[i]
							}
						}
					}
					identifiers = append(identifiers, cpuIds[0])
					if debugOutput {
						fmt.Fprintf(os.Stderr, "[sconfig DEBUG] CPU ProcessorIds found: %d, using first (sorted): %s\n", len(cpuIds), cpuIds[0])
					}
				}
			}
		}

	case "linux":
		if isVM {
			// For VMs: prioritize stable identifiers
			// 1. machine-id (very stable on VMs)
			machineId, err := os.ReadFile("/etc/machine-id")
			if err == nil {
				machineIdStr := strings.TrimSpace(string(machineId))
				if machineIdStr != "" {
					identifiers = append(identifiers, machineIdStr)
				}
			}

			// 2. product_uuid (usually stable on VMs)
			productUuid, err := os.ReadFile("/sys/class/dmi/id/product_uuid")
			if err == nil {
				productUuidStr := strings.TrimSpace(string(productUuid))
				if productUuidStr != "" {
					identifiers = append(identifiers, productUuidStr)
				}
			}
		}

		// Common identifiers (for both VM and physical)
		cmds := []string{
			"cat /sys/class/dmi/id/board_serial",
		}

		// Only add CPU serial if not on VM (often unreliable on VMs)
		if !isVM {
			// For CPU serial, collect all values and use the first one (sorted) for stability
			out, err := exec.Command("sh", "-c", "cat /proc/cpuinfo | grep 'Serial'").Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				var cpuSerials []string
				for _, line := range lines {
					// Extract serial number from lines like "Serial          : xxxxxxxx"
					if strings.Contains(line, "Serial") {
						parts := strings.Split(line, ":")
						if len(parts) > 1 {
							value := strings.TrimSpace(parts[1])
							if value != "" {
								cpuSerials = append(cpuSerials, value)
							}
						}
					}
				}
				// Remove duplicates and sort for consistency
				if len(cpuSerials) > 0 {
					// Remove duplicates
					seen := make(map[string]bool)
					unique := []string{}
					for _, serial := range cpuSerials {
						if !seen[serial] {
							seen[serial] = true
							unique = append(unique, serial)
						}
					}
					// Sort
					for i := 0; i < len(unique)-1; i++ {
						for j := i + 1; j < len(unique); j++ {
							if unique[i] > unique[j] {
								unique[i], unique[j] = unique[j], unique[i]
							}
						}
					}
					if len(unique) > 0 {
						identifiers = append(identifiers, unique[0])
						if debugOutput {
							fmt.Fprintf(os.Stderr, "[sconfig DEBUG] CPU Serial numbers found: %d (unique: %d), using first (sorted): %s\n", len(cpuSerials), len(unique), unique[0])
						}
					}
				}
			}
		}

		for _, cmd := range cmds {
			out, err := exec.Command("sh", "-c", cmd).Output()
			if err == nil {
				value := strings.TrimSpace(string(out))
				if value != "" {
					identifiers = append(identifiers, value)
				}
			}
		}
	}

	if len(identifiers) == 0 {
		return 0, fmt.Errorf("no hardware identifiers found")
	}

	// Sort identifiers to ensure consistent ordering regardless of collection order
	// This is critical because the order affects the hash
	for i := 0; i < len(identifiers)-1; i++ {
		for j := i + 1; j < len(identifiers); j++ {
			if identifiers[i] > identifiers[j] {
				identifiers[i], identifiers[j] = identifiers[j], identifiers[i]
			}
		}
	}

	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Hardware identifiers found: %d (sorted)\n", len(identifiers))
		for i, id := range identifiers {
			fmt.Fprintf(os.Stderr, "[sconfig DEBUG]   Identifier %d: %s\n", i+1, id)
		}
	}

	// Combine all identifiers and create a hash
	combined := strings.Join(identifiers, "|")
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Combined identifiers: %s\n", combined)
	}
	hash := sha256.Sum256([]byte(combined))
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] SHA256 hash: %x\n", hash)
	}
	// Return first 64 bits as an uint64 ==> this is the pseudo-unique identifier of the system
	hardwareID := uint64(hash[7])<<56 + uint64(hash[6])<<48 + uint64(hash[5])<<40 + uint64(hash[4])<<32 + uint64(hash[3])<<24 + uint64(hash[2])<<16 + uint64(hash[1])<<8 + uint64(hash[0])
	if debugOutput {
		fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Hardware ID (uint64): %d (0x%016x)\n", hardwareID, hardwareID)
	}
	return hardwareID, nil
}

// LoadConfig reads a JSON configuration file into the provided struct, applies
// default values from struct tags, synchronizes an optional `Version` field,
// and manages password encryption/decryption.
//
// Behavior:
//   - If the file does not exist, an empty configuration is assumed.
//   - Fields named `<Name>Password` and `<Name>SecurePassword` are treated as a
//     pair. If the plaintext password differs from the recognized marker,
//     it will be encrypted into `<Name>SecurePassword` and the plaintext field
//     will be replaced by the marker string.
//   - When `cleanConfig` is true, the function writes back a config file where
//     passwords are present as plaintext (use with care), primarily for
//     migration or inspection purposes.
//   - On successful completion and when `cleanConfig` is false, passwords are
//     decrypted in memory so callers can use the plaintext values directly.
//   - When `debugOutput` is true, all intermediate results and the final encryption
//     key are printed to stderr for debugging purposes.
//
// The optional `getHardwareID_func` allows overriding the hardware-ID based key
// derivation used for encryption, which is primarily intended for testing.
func LoadConfig(config interface{}, version int, path string, cleanConfig bool, debugOutput bool, getHardwareID_func ...func() (uint64, error)) error {

	var file []byte

	// Create wrapper function for hardware ID retrieval with debug support
	var hardwareIDFunc func() (uint64, error)
	if len(getHardwareID_func) > 0 {
		hardwareIDFunc = getHardwareID_func[0]
	} else {
		// Create wrapper that calls the debug version
		hardwareIDFunc = func() (uint64, error) {
			return secure_config_getHardwareID_debug(debugOutput)
		}
	}
	config_init(hardwareIDFunc, debugOutput)

	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		file, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf(t("config.read_failed"), err)
		}
	} else {
		file = []byte("{}")
	}

	// Analyze config type
	configValue := reflect.ValueOf(config)
	if configValue.Kind() == reflect.Ptr {
		configValue = configValue.Elem()
	} else {
		return fmt.Errorf("%s", t("config.config_no_struct"))
	}
	if configValue.Kind() != reflect.Struct {
		return fmt.Errorf("%s", t("config.config_no_struct"))
	}

	if err := updateDefaultValues(configValue); err != nil {
		return fmt.Errorf(t("config.failed_defaulting"), err)
	}

	if err := json.Unmarshal(file, config); err != nil {
		return fmt.Errorf(t("config.failed_parsing"), err)
	}
	changed := false
	if err := updateVersionAndPasswords(configValue, version, &changed); err != nil {
		return fmt.Errorf(t("config.failed_checking"), err)
	}
	if cleanConfig {
		/* Decrypt passwords before writing */
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
		changed = true
	}
	if changed {
		configJSON, err := json.MarshalIndent(config, "", "\t")
		if err != nil {
			return fmt.Errorf(t("config.failed_build_json"), err)
		}
		if err := os.WriteFile(path, configJSON, 0644); err != nil {
			return fmt.Errorf(t("config.failed_writing"), path, err)
		}
	}
	if !cleanConfig {
		/* Decrypt passwords after writing */
		if err := decodePasswords(configValue); err != nil {
			return fmt.Errorf(t("config.failed_decode_pw"), err)
		}
	}
	return nil
}

/*
 * Password key initialization
 *
 * This involves initializing from the computer's hardware properties.
 * This makes the file unusable on another computer - this is an
 * additional security feature.
 *
 * For transferring files of the first version of this application, an old,
 * insecure key generation procedure can also be used.
 */
func config_init(getHardwareID_func func() (uint64, error), debugOutput bool) {
	if !initialized {
		// Generate encryption key based on Hardware IS
		hardwareID, err := getHardwareID_func()
		if err != nil {
			log.Fatalf("%s", t("config.hardware_id_failed"))
		}
		if debugOutput {
			fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Hardware ID used for key generation: %d (0x%016x)\n", hardwareID, hardwareID)
		}
		randGenSeeded := mathRand.NewSource(int64(hardwareID))
		encryptionKey = make([]byte, 32)
		for i := range encryptionKey {
			encryptionKey[i] = byte(randGenSeeded.Int63() >> 16 & 0xff)
		}
		if debugOutput {
			fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Encryption key (32 bytes): %x\n", encryptionKey)
			fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Encryption key (hex string): %s\n", fmt.Sprintf("%x", encryptionKey))
		}
		curr_lang := getCurrentLanguage()
		setLanguage("de")
		PASSWORD_IS_SECURE_de = t("config.password_message")
		setLanguage("en")
		PASSWORD_IS_SECURE_en = t("config.password_message")
		setLanguage(curr_lang)
		PASSWORD_IS_SECURE = t("config.password_message")
		if debugOutput {
			fmt.Fprintf(os.Stderr, "[sconfig DEBUG] Password secure marker: %s\n", PASSWORD_IS_SECURE)
		}
	}
	initialized = true
}

/*
 * Go through the structure and set the default values present
 * in the annotations
 */
func updateDefaultValues(v reflect.Value) error {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	type_info := v.Type()
	// Iterate through all fields
	for i := 0; i < type_info.NumField(); i++ {
		field := type_info.Field(i)
		fieldValue := v.Field(i)
		if field.Type.Kind() == reflect.Struct {
			if err := updateDefaultValues(fieldValue); err != nil {
				return fmt.Errorf(t("config.default_error"), err)
			}
		} else if field.Type.Kind() == reflect.Slice {
			for i := 0; i < fieldValue.Len(); i++ {
				if fieldValue.Index(i).Kind() == reflect.Struct {
					if err := updateDefaultValues(fieldValue.Index(i)); err != nil {
						return err
					}
				}
			}
		} else {
			defaultValue, found := field.Tag.Lookup("default")
			if found {
				switch fieldValue.Kind() {
				case reflect.String:
					fieldValue.SetString(defaultValue)
				case reflect.Int, reflect.Int64:
					value, err := strconv.Atoi(defaultValue)
					if err != nil {
						return fmt.Errorf(t("config.default_error"), err)
					}
					fieldValue.SetInt(int64(value))
				case reflect.Bool:
					boolValue, err := strconv.ParseBool(defaultValue)
					if err != nil {
						return fmt.Errorf(t("config.default_error"), err)
					}
					fieldValue.SetBool(boolValue)
				default:
					return fmt.Errorf(t("config.default_unsupported"), fieldValue.Kind())
				}
			}
		}
	}
	return nil
}

/*
 * Check new content and update encrypted passwords and version as needed
 * If changes are made, the modified file will be written back at the end
 */
func updateVersionAndPasswords(v reflect.Value, version int, changed *bool) error {
	if v.Kind() == reflect.Ptr {
		//fmt.Printf("Pointer\n")
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	// Iterate through all fields
	for i := 0; i < t.NumField(); i++ {

		field := t.Field(i)
		fieldValue := v.Field(i)

		// Process nested structures recursively
		if field.Type.Kind() == reflect.Struct {
			if err := updateVersionAndPasswords(fieldValue, version, changed); err != nil {
				return err
			}
		} else if field.Type.Kind() == reflect.Slice {
			//fmt.Printf("Slice[0..%d]\n", fieldValue.Len()-1)
			for i := 0; i < fieldValue.Len(); i++ {
				//fmt.Printf("Slice-Element %d:\n", i)
				if fieldValue.Index(i).Kind() == reflect.Struct {
					if err := updateVersionAndPasswords(fieldValue.Index(i), version, changed); err != nil {
						return err
					}
				} else {
					//fmt.Printf(" is '%v' (%s)\n", fieldValue.Index(i), fieldValue.Index(i).Kind().String())
				}
			}
		} else {
			// Version check
			if field.Name == "Version" {
				if fieldValue.Int() != int64(version) {
					fieldValue.SetInt(int64(version))
					*changed = true
				}
			}
			// Password handling
			if strings.HasSuffix(field.Name, "SecurePassword") {
				pw_prefix := strings.TrimSuffix(field.Name, "SecurePassword")
				for j := 0; j < t.NumField(); j++ {
					if t.Field(j).Name == pw_prefix+"Password" {
						field2Value := v.Field(j)
						if field2Value.String() != PASSWORD_IS_SECURE_de && field2Value.String() != PASSWORD_IS_SECURE_en {
							// New password found in plain text
							// New Secure_Password is calculated
							password := encrypt(field2Value.String())
							fieldValue.SetString(password)
							field2Value.SetString(PASSWORD_IS_SECURE)
							//fmt.Printf(" new value %s\n", password)
							*changed = true
						}
						break
					}
				}
			}
		}
	}
	return nil
}

/*
 * Decrypt the encrypted passwords so that the encryption is transparent in the main program.
 */
func decodePasswords(v reflect.Value) error {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	type_info := v.Type()
	// Iterate through all fields
	for i := 0; i < type_info.NumField(); i++ {
		field := type_info.Field(i)
		fieldValue := v.Field(i)

		// Process recursively nested structures
		if field.Type.Kind() == reflect.Struct {
			if err := decodePasswords(fieldValue); err != nil {
				return err
			}
		} else if field.Type.Kind() == reflect.Slice {
			for i := 0; i < fieldValue.Len(); i++ {
				if fieldValue.Index(i).Kind() == reflect.Struct {
					if err := decodePasswords(fieldValue.Index(i)); err != nil {
						return err
					}
				}
			}
		} else {
			// Password processing
			if strings.HasSuffix(field.Name, "SecurePassword") {
				pw_prefix := strings.TrimSuffix(field.Name, "SecurePassword")
				for j := 0; j < type_info.NumField(); j++ {
					if type_info.Field(j).Name == pw_prefix+"Password" {
						field2Value := v.Field(j)
						password, err := decrypt(fieldValue.String())
						if err != nil {
							return fmt.Errorf(t("config.decrypt_failed", pw_prefix), err)
						}
						field2Value.SetString(password)
						break
					}
				}
			}
		}
	}
	return nil
}

func encrypt(text string) string {
	block, _ := aes.NewCipher(encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext)
}

func decrypt(text string) (string, error) {
	block, _ := aes.NewCipher(encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	data, _ := base64.StdEncoding.DecodeString(text)
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	return string(plaintext), err
}
