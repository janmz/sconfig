package sconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// errCommandTimeout is returned by runCommand when an external command does
// not finish in time. Callers must treat it as an error and never as a
// "missing identifier", otherwise a slow system would silently get another key.
var errCommandTimeout = errors.New("external command timed out")

// defaultCommandTimeout limits every external command used for the machine
// identity (wmic, reg, ipconfig, ip, route, ioreg, sysctl, ...).
const defaultCommandTimeout = 15 * time.Second

// powershellCommandTimeout is longer because a cold PowerShell start can take
// several seconds on slow or freshly booted machines.
const powershellCommandTimeout = 30 * time.Second

// commandWaitDelay bounds the wait for output pipes after the process was
// killed (a grandchild could otherwise keep them open and defeat the timeout).
const commandWaitDelay = 2 * time.Second

// maxCommandOutput limits the captured standard output of a command.
const maxCommandOutput = 1 << 20

// runCommand runs an external command with defaultCommandTimeout and returns
// its standard output.
func runCommand(name string, args ...string) ([]byte, error) {
	return runCommandTimeout(defaultCommandTimeout, name, args...)
}

// limitedBuffer collects at most maxCommandOutput bytes and drops the rest.
type limitedBuffer struct {
	data []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := maxCommandOutput - len(b.data); room > 0 {
		if len(p) > room {
			b.data = append(b.data, p[:room]...)
		} else {
			b.data = append(b.data, p...)
		}
	}
	return len(p), nil
}

// runCommandTimeout runs an external command (without a console window on
// Windows, with a fixed search path elsewhere) and returns its standard
// output. A timeout yields an error that wraps errCommandTimeout.
func runCommandTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := newCommandContext(ctx, name, args...)
	cmd.WaitDelay = commandWaitDelay
	out := &limitedBuffer{}
	cmd.Stdout = out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%w: %s (%s)", errCommandTimeout, name, timeout)
	}
	return out.data, err
}

// resolveTool returns the first existing absolute candidate path, or name
// itself (then os/exec searches PATH, never the current directory).
func resolveTool(name string, candidates ...string) string {
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return name
}
