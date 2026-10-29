//go:build windows

package sconfig

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	replaceFileIgnoreMergeErrors = 0x00000002
	daclSecurityInformation      = 0x00000004
	// ownerOnlySDDL grants full access to the owner, SYSTEM and the
	// Administrators group and blocks inherited permissions.
	ownerOnlySDDL = "D:P(A;;FA;;;OW)(A;;FA;;;SY)(A;;FA;;;BA)"
)

var (
	advapi32                        = syscall.NewLazyDLL("advapi32.dll")
	procReplaceFile                 = kernel32.NewProc("ReplaceFileW")
	procLocalFree                   = kernel32.NewProc("LocalFree")
	procConvertStringSDToSD         = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procSetFileSecurity             = advapi32.NewProc("SetFileSecurityW")
	sddlRevision1           uintptr = 1
)

// replaceFile replaces target with tmp. ReplaceFileW keeps the security
// descriptor, attributes and creation time of the target; if it fails (e.g.
// on file systems without support), os.Rename is used.
func replaceFile(tmp, target string) error {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	tmpPtr, err := syscall.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	r, _, _ := procReplaceFile.Call(uintptr(unsafe.Pointer(targetPtr)), uintptr(unsafe.Pointer(tmpPtr)),
		0, replaceFileIgnoreMergeErrors, 0, 0)
	if r != 0 {
		return nil
	}
	return os.Rename(tmp, target)
}

// copyOwnership is not needed on Windows (ReplaceFileW keeps the owner).
func copyOwnership(os.FileInfo, string) {}

// restrictNewFile sets a DACL that allows only owner, SYSTEM and
// Administrators (chmod 0600 has no effect on Windows ACLs).
func restrictNewFile(path string) error {
	sddl, err := syscall.UTF16PtrFromString(ownerOnlySDDL)
	if err != nil {
		return err
	}
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var sd uintptr
	r, _, callErr := procConvertStringSDToSD.Call(uintptr(unsafe.Pointer(sddl)), sddlRevision1,
		uintptr(unsafe.Pointer(&sd)), 0)
	if r == 0 {
		return callErr
	}
	defer procLocalFree.Call(sd)
	r, _, callErr = procSetFileSecurity.Call(uintptr(unsafe.Pointer(pathPtr)), daclSecurityInformation, sd)
	if r == 0 {
		return callErr
	}
	return nil
}
