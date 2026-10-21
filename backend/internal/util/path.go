package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveUnderRoot joins root and name, rejecting empty root and ".." escapes.
// Used by store.FileStore to keep writes inside the data directory.
//
//	full, err := util.ResolveUnderRoot("/data", "prefs.json")
func ResolveUnderRoot(root, name string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("util: empty root directory")
	}
	full := filepath.Clean(filepath.Join(root, name))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("util: illegal path %q", name)
	}
	return full, nil
}
