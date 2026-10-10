//go:build !windows

package sconfig

import "os/exec"

// newCommand creates a command (no special handling outside Windows).
func newCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
