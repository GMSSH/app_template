package logging

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/paths"
	"gmssh-cli-plugin/internal/util"
)

// Overlay carries CLI-flag overrides applied after config and environment.
// MaxDaysSet / ConsoleSet distinguish "flag absent" from "flag set to zero/false".
type Overlay struct {
	Level      string
	Dir        string
	File       string // optional override for runtime log file layout
	Pattern    string
	MaxDays    int
	MaxDaysSet bool
	Debug      bool
	Console    bool
	ConsoleSet bool
}

// ResolveError names which settings field failed validation.
// The command engine maps Field to an i18n UsageFail key.
type ResolveError struct {
	Field string
	Value string
}

func (e *ResolveError) Error() string {
	if e == nil {
		return ""
	}
	return e.Field + "=" + e.Value
}

// Resolve merges settings in order: base (config) → env → Overlay (flags).
//
//	base, _ := logging settings from config.LoadAll()
//	settings, err := logging.Resolve(base, logging.Overlay{Debug: g.debug})
//	logging.Configure(settings)
func Resolve(base Settings, o Overlay) (Settings, error) {
	s := base
	if s.MaxDays <= 0 {
		s.MaxDays = consts.DefaultLogMaxDays
	}
	if err := applyEnv(&s); err != nil {
		return s, err
	}
	if err := applyOverlay(&s, o); err != nil {
		return s, err
	}
	return finalizeSettings(s)
}

func applyEnv(s *Settings) error {
	if err := applyEnvLevel(s); err != nil {
		return err
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogDir)); v != "" {
		s.Dir = v
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogFile)); v != "" {
		s.RuntimeFile = v
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogFilePattern)); v != "" {
		s.Pattern = v
	}
	if err := applyEnvMaxDays(s); err != nil {
		return err
	}
	return applyEnvConsole(s)
}

func applyEnvLevel(s *Settings) error {
	v := strings.TrimSpace(os.Getenv(consts.EnvLogLevel))
	if v == "" {
		return nil
	}
	parsed, ok := ParseLevel(v)
	if !ok {
		return &ResolveError{Field: "level", Value: v}
	}
	s.Level = parsed
	return nil
}

func applyEnvMaxDays(s *Settings) error {
	v := strings.TrimSpace(os.Getenv(consts.EnvLogMaxDays))
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return &ResolveError{Field: "max_days", Value: v}
	}
	s.MaxDays = n
	return nil
}

func applyEnvConsole(s *Settings) error {
	v := strings.TrimSpace(os.Getenv(consts.EnvLogConsole))
	if v == "" {
		return nil
	}
	on, ok := util.ParseBool(v)
	if !ok {
		return &ResolveError{Field: "console", Value: v}
	}
	s.Console = on
	return nil
}

func applyOverlay(s *Settings, o Overlay) error {
	if err := applyOverlayLevel(s, o.Level); err != nil {
		return err
	}
	if strings.TrimSpace(o.Dir) != "" {
		s.Dir = o.Dir
	}
	if strings.TrimSpace(o.File) != "" {
		s.RuntimeFile = o.File
	}
	if strings.TrimSpace(o.Pattern) != "" {
		s.Pattern = o.Pattern
	}
	if o.MaxDaysSet {
		s.MaxDays = o.MaxDays
	}
	if o.ConsoleSet {
		s.Console = o.Console
	}
	applyDebug(s, o)
	return nil
}

func applyOverlayLevel(s *Settings, level string) error {
	if strings.TrimSpace(level) == "" {
		return nil
	}
	parsed, ok := ParseLevel(level)
	if !ok {
		return &ResolveError{Field: "level", Value: level}
	}
	s.Level = parsed
	return nil
}

func applyDebug(s *Settings, o Overlay) {
	if !o.Debug {
		return
	}
	if s.Level < Debug {
		s.Level = Debug
	}
	if !o.ConsoleSet {
		s.Console = true
	}
}

func finalizeSettings(s Settings) (Settings, error) {
	if strings.TrimSpace(s.RuntimeFile) == "" {
		s.RuntimeFile = consts.DefaultRuntimeLogFile
	}
	if strings.TrimSpace(s.BusinessFile) == "" {
		s.BusinessFile = consts.DefaultBusinessLogFile
	}
	s.Dir = resolveDir(s.Dir)
	if _, err := NewFileNamer(s.RuntimeFile, s.Pattern); err != nil {
		return s, &ResolveError{Field: "file_pattern", Value: s.Pattern}
	}
	if _, err := NewFileNamer(s.BusinessFile, s.Pattern); err != nil {
		return s, &ResolveError{Field: "file_pattern", Value: s.Pattern}
	}
	return s, nil
}

func resolveDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = consts.DefaultLogDir
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Clean(filepath.Join(logBase(), dir))
}

func logBase() string {
	exe, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		if filepath.Base(dir) == "bin" && filepath.Base(filepath.Dir(dir)) == "app" {
			return dir
		}
	}
	return filepath.Join(paths.Root(), "app", "bin")
}
