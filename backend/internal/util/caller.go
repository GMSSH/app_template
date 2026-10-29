package util

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// CallerSite returns "relative/file.go:line" for the call stack frame skip levels
// above this function (skip=1 → direct caller of CallerSite).
//
// Paths under .../internal/ are shortened to internal/... for stable logs.
func CallerSite(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1) // +1 ignores CallerSite itself
	if !ok || file == "" {
		return ""
	}
	if i := strings.LastIndex(file, string(filepath.Separator)+"internal"+string(filepath.Separator)); i >= 0 {
		file = file[i+1:]
	} else {
		file = filepath.Base(file)
	}
	return fmt.Sprintf("%s:%d", file, line)
}
