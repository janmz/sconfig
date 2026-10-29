//go:build windows

package sconfig

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"unsafe"
)

// createNoWindow is the Windows process creation flag CREATE_NO_WINDOW.
const createNoWindow = 0x08000000

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemDirectory = kernel32.NewProc("GetSystemDirectoryW")
	systemRootPattern      = regexp.MustCompile(`^[A-Za-z]:\\[^;%"<>|?*]*$`)
)

// newCommandContext creates a command that runs without a console window.
// Without this, GUI programs (built with -H windowsgui) flash a console window
// for every hardware-ID query. NoDefaultCurrentDirectoryInExePath stops
// cmd.exe and other children from running programs out of the current
// directory.
func newCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	cmd.Env = append(os.Environ(), "NoDefaultCurrentDirectoryInExePath=1")
	return cmd
}

// windowsSystemDir returns the System32 directory from the Windows API
// (GetSystemDirectoryW). The environment variable SystemRoot is only used,
// after validation, if the API call fails.
func windowsSystemDir() string {
	buf := make([]uint16, syscall.MAX_PATH)
	n, _, _ := procGetSystemDirectory.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n > 0 && int(n) < len(buf) {
		return syscall.UTF16ToString(buf[:n])
	}
	if root := os.Getenv("SystemRoot"); systemRootPattern.MatchString(root) {
		return filepath.Join(root, "System32")
	}
	return `C:\Windows\System32`
}

// winTool returns the absolute path of a program below System32.
func winTool(relative string) string {
	return filepath.Join(windowsSystemDir(), relative)
}
