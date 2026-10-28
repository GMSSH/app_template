package util

import (
	"os"
	"path/filepath"
)

// Exists reports whether path exists (file or directory).
//
//	if util.Exists(dir) { ... }
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FileExists reports whether path exists and is a regular file (not a directory).
func FileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// WriteFile creates parent directories (0700) then writes data with the given mode.
//
//	err := util.WriteFile(path, data, 0o600)
func WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}
