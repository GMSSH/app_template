// Package paths resolves the plugin root and standard runtime directories.
//
// Single responsibility: locate the install / runtime root and expose helpers
// for data/, logs/, and config paths. It does not read config contents or write
// business data.
//
// Root detection order:
//  1. GMCLI_ROOT
//  2. parent of app/bin when the executable lives at <root>/app/bin/main
//  3. process working directory (go run / tests)
//
// Typical layout after make bundle:
//
//	<root>/
//	  app/bin/main
//	  data/config.json
//	  configs/config.json          (source / templates)
//	  configs/config.development.json
//	  logs/
//
// Usage:
//
//	root := paths.Root()
//	cfg  := paths.ConfigFile()       // <root>/data/config.json
//	src  := paths.SourceConfigFile() // <root>/configs/config.json
//	data := paths.DataDir()
//	logs := paths.LogDir()
//
// Tests may override the root:
//
//	paths.SetRoot(t.TempDir())
package paths

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gmssh-cli-plugin/internal/consts"
)

var (
	mu   sync.RWMutex
	root string
)

func init() {
	root = detect()
}

func detect() string {
	if v := strings.TrimSpace(os.Getenv(consts.EnvRoot)); v != "" {
		return filepath.Clean(v)
	}
	exe, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		// Installed layout: <root>/app/bin/main → plugin root is <root>.
		if filepath.Base(dir) == "bin" && filepath.Base(filepath.Dir(dir)) == "app" {
			return filepath.Dir(filepath.Dir(dir))
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// SetRoot overrides the plugin root (tests only).
func SetRoot(p string) {
	mu.Lock()
	defer mu.Unlock()
	root = filepath.Clean(p)
}

// Root returns the current plugin root directory.
func Root() string {
	mu.RLock()
	defer mu.RUnlock()
	if root == "" {
		return detect()
	}
	return root
}

// DataDir is <root>/data (runtime JSON and installed config.json).
func DataDir() string { return filepath.Join(Root(), "data") }

// ConfigFile is the runtime config path: data/config.json.
func ConfigFile() string { return filepath.Join(DataDir(), "config.json") }

// SourceConfigFile is the template config.json.
// Prefers packaged layout config/, then source-tree configs/.
func SourceConfigFile() string {
	for _, dir := range []string{"config", "configs"} {
		p := filepath.Join(Root(), dir, "config.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(Root(), "configs", "config.json")
}

// ProfileConfigFile is the profile overlay config.<profile>.json.
// Prefers packaged layout config/, then source-tree configs/.
func ProfileConfigFile(profile string) string {
	name := "config." + profile + ".json"
	for _, dir := range []string{"config", "configs"} {
		p := filepath.Join(Root(), dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(Root(), "configs", name)
}

// LogDir is <root>/logs.
func LogDir() string { return filepath.Join(Root(), "logs") }
