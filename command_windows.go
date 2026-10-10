//go:build windows

package sconfig

import (
	"os/exec"
	"syscall"
)

// createNoWindow is the Windows process creation flag CREATE_NO_WINDOW.
const createNoWindow = 0x08000000

// newCommand creates a command that runs without a console window. Without
// this, GUI programs (built with -H windowsgui) flash a console window for
// every hardware-ID query (wmic, reg, ipconfig, route, netsh, ...).
func newCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}
