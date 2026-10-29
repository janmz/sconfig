//go:build !windows

package sconfig

import (
	"os"
	"syscall"
)

// replaceFile renames tmp over target.
func replaceFile(tmp, target string) error {
	return os.Rename(tmp, target)
}

// copyOwnership gives tmp the owner and group of the existing file. Changing
// the owner needs privileges; failures are ignored (the file then belongs to
// the current user, as any newly written file).
func copyOwnership(existing os.FileInfo, tmp string) {
	if st, ok := existing.Sys().(*syscall.Stat_t); ok {
		_ = os.Chown(tmp, int(st.Uid), int(st.Gid))
	}
}

// restrictNewFile is not needed outside Windows (the mode 0600 applies).
func restrictNewFile(string) error { return nil }
