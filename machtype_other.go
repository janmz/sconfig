//go:build !linux

package sconfig

import "errors"

// unameMachine runs "uname -m" with an absolute path only (no search in
// PATH, so a manipulated PATH cannot supply the value). Without uname the
// source is not available. On Windows the architecture is read from the
// registry instead.
func unameMachine() (string, error) {
	tool := resolveTool("", "/usr/bin/uname", "/bin/uname")
	if tool == "" {
		return "", errors.New("uname not found")
	}
	out, err := runCommand(tool, "-m")
	return string(out), err
}
