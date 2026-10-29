//go:build !windows

package sconfig

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// fixedSearchPath is the PATH of all external commands, so that a manipulated
// PATH cannot substitute the tools used for the machine identity.
const fixedSearchPath = "PATH=/usr/sbin:/usr/bin:/sbin:/bin"

// newCommandContext creates a command with a fixed search path. Locale
// variables are kept so that tool output does not change.
func newCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	env := []string{fixedSearchPath}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "LANG=") || strings.HasPrefix(kv, "LC_") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	return cmd
}

// windowsSystemDir is only meaningful on Windows.
func windowsSystemDir() string { return "" }

// winTool is only meaningful on Windows.
func winTool(relative string) string { return relative }
