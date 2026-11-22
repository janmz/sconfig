//go:build linux

package sconfig

import "syscall"

// unameMachine returns the machine field of uname(2), the same value as
// PHP php_uname('m') and "uname -m", without starting a process.
func unameMachine() (string, error) {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return "", err
	}
	b := make([]byte, 0, len(u.Machine))
	for _, c := range u.Machine { // int8 or uint8 depending on the architecture
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b), nil
}
