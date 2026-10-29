package sconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// newConfigFileMode is used for config files that do not exist yet. They may
// contain plaintext passwords (cleanConfig), so only the owner may read them.
const newConfigFileMode os.FileMode = 0600

// Size limits for files that are read completely into memory.
const (
	maxConfigFileSize = 16 << 20
	maxLocaleFileSize = 1 << 20
)

// readFileLimited reads a file that must not be larger than limit bytes.
func readFileLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s", t("config.file_too_large", path, limit))
	}
	return data, nil
}

// configFileMode returns the permissions of an existing file, or
// newConfigFileMode if it does not exist.
func configFileMode(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return newConfigFileMode
}

// writeFileAtomic writes data to a temporary file next to the target, flushes
// it and replaces the target with it, so that an interrupted write never
// leaves a truncated config behind.
//
//   - A symbolic link is kept: the file it points to is replaced.
//   - Existing files keep their permissions and owner (Unix) or their
//     security descriptor and attributes (Windows, ReplaceFileW).
//   - New files get perm; on Windows their DACL allows only owner, SYSTEM and
//     Administrators.
//   - A warning is issued if an existing file is readable by other users.
func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	target := path
	if resolved, evalErr := filepath.EvalSymlinks(path); evalErr == nil {
		target = resolved
	}
	existing, statErr := os.Stat(target)
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if statErr != nil {
		// new file
		if err = restrictNewFile(tmpName); err != nil {
			return err
		}
		return os.Rename(tmpName, target)
	}
	copyOwnership(existing, tmpName)
	if runtime.GOOS != "windows" && existing.Mode().Perm()&0o077 != 0 {
		warn(t("config.warn_permissions", target, existing.Mode().Perm()))
	}
	return replaceFile(tmpName, target)
}
