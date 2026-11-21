//go:build !windows && !linux

package sconfig

import (
	"errors"
	"os"
	"runtime"
	"syscall"
)

// fileSystemID returns st_dev of path (the same value as PHP stat()['dev']).
// On macOS st_dev equals statfs f_fsid.val[0]; only its lower 32 bits are
// used there (dev_t is a signed 32-bit integer).
func fileSystemID(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("stat: no device number")
	}
	id := uint64(st.Dev) //nolint:unconvert // dev_t differs between systems
	if runtime.GOOS == "darwin" {
		id &= 0xffffffff
	}
	return id, nil
}
