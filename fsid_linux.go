//go:build linux

package sconfig

import "syscall"

// fileSystemID returns statfs f_fsid as val[0]<<32 | val[1], the value that
// "stat -f -c %i" prints (GNU coreutils and busybox), which the PHP
// implementation uses.
func fileSystemID(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(uint32(st.Fsid.X__val[0]))<<32 | uint64(uint32(st.Fsid.X__val[1])), nil
}
