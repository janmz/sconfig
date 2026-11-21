//go:build windows

package sconfig

import "syscall"

// fileReadAttributes is the access right FILE_READ_ATTRIBUTES.
const fileReadAttributes = 0x80

// fileSystemID returns the volume serial number of the volume that holds
// path (the same value as PHP stat()['dev'] on Windows).
func fileSystemID(path string) (uint64, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	// FILE_FLAG_BACKUP_SEMANTICS is required to open directories.
	h, err := syscall.CreateFile(name, fileReadAttributes,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(h)
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		return 0, err
	}
	return uint64(info.VolumeSerialNumber), nil
}
