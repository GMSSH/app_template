// Package config loads JSON profiles with environment overlays.
//
// Load order (later wins):
//  1. Built-in defaults (Default)
//  2. Base file — data/config.json, else configs/config.json
//  3. Profile overlay — configs/config.<profile>.json when present
//  4. Environment variables (GMCLI_*)
//
// Profile selection: GMCLI_PROFILE → config.profile → production.
// Supported profiles: production | development | test (aliases: dev, local, testing).
//
// CLI flags (--log-level, --debug, ...) are applied later by the command engine
// via logging.Resolve Overlay — not here.
//
// Example (engine bootstrap):
//
//	cfg, err := config.LoadAll()
//	if err != nil { ... }
//	fmt.Println(cfg.Profile, cfg.Log.Level)
//
// Example (force a profile in shells / CI):
//
//	export GMCLI_PROFILE=development
//	export GMCLI_LOG_LEVEL=debug
//	./main ping
//
// Example JSON (configs/config.development.json):
//
//	{
//	  "log": {
//	    "level": "debug",
//	    "console": true,
//	    "runtime_file": "runtime-2006-01-02.log",
//	    "business_file": "business-2006-01-02.log"
//	  }
//	}
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/paths"
	"gmssh-cli-plugin/internal/util"
)

// Profile selects environment-specific overlays.
type Profile string

const (
	ProfileProduction  Profile = "production"
	ProfileDevelopment Profile = "development"
	ProfileTest        Profile = "test"
)

// Log holds logging-related runtime knobs stored in JSON.
type Log struct {
	Level        string `json:"level"`
	Dir          string `json:"dir"`
	File         string `json:"file"`          // legacy runtime layout key (compat)
	RuntimeFile  string `json:"runtime_file"`  // preferred over file
	BusinessFile string `json:"business_file"` // business channel layout
	FilePattern  string `json:"file_pattern"`  // retention match regexp
	MaxDays      int    `json:"max_days"`
	Console      bool   `json:"console"`
}

// RuntimeFileOrDefault prefers runtime_file, then file, then consts default.
func (l Log) RuntimeFileOrDefault() string {
	if v := strings.TrimSpace(l.RuntimeFile); v != "" {
		return v
	}
	if v := strings.TrimSpace(l.File); v != "" {
		return v
	}
	return consts.DefaultRuntimeLogFile
}

// BusinessFileOrDefault returns the business log layout or the scaffold default.
func (l Log) BusinessFileOrDefault() string {
	if v := strings.TrimSpace(l.BusinessFile); v != "" {
		return v
	}
	return consts.DefaultBusinessLogFile
}

// File is the on-disk config document shape.
type File struct {
	Profile string `json:"profile,omitempty"`
	Log     Log    `json:"log"`
}

// Default returns production-oriented built-ins used when no file exists.
func Default() File {
	return File{
		Profile: string(ProfileProduction),
		Log: Log{
			Level:        consts.DefaultLogLevel,
			Dir:          consts.DefaultLogDir,
			RuntimeFile:  consts.DefaultRuntimeLogFile,
			BusinessFile: consts.DefaultBusinessLogFile,
			FilePattern:  consts.DefaultLogFilePattern,
			MaxDays:      consts.DefaultLogMaxDays,
		},
	}
}

// ActiveProfile resolves GMCLI_PROFILE, then cfg.Profile, else production.
func ActiveProfile(cfg File) Profile {
	if v := strings.TrimSpace(os.Getenv(consts.EnvProfile)); v != "" {
		return NormalizeProfile(v)
	}
	if v := strings.TrimSpace(cfg.Profile); v != "" {
		return NormalizeProfile(v)
	}
	return ProfileProduction
}

// NormalizeProfile maps aliases onto the three supported profiles.
func NormalizeProfile(v string) Profile {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "dev", "development", "local":
		return ProfileDevelopment
	case "test", "testing":
		return ProfileTest
	default:
		return ProfileProduction
	}
}

// Path picks the base config file:
// GMCLI_CONFIG > data/config.json > configs/config.json > data/config.json.
func Path() string {
	if v := strings.TrimSpace(os.Getenv(consts.EnvConfig)); v != "" {
		return v
	}
	data := paths.ConfigFile()
	if util.FileExists(data) {
		return data
	}
	source := paths.SourceConfigFile()
	if util.FileExists(source) {
		return source
	}
	return data
}

// ProfilePath returns config/config.<profile>.json (packaged) or
// configs/config.<profile>.json (source tree), whichever exists.
func ProfilePath(profile Profile) string {
	return paths.ProfileConfigFile(string(profile))
}

// LoadAll loads defaults → base file → profile file → env overlays.
// This is the function the command engine should call at bootstrap.
func LoadAll() (File, error) {
	cfg, err := Load(Path())
	if err != nil {
		return cfg, err
	}
	profile := ActiveProfile(cfg)
	cfg.Profile = string(profile)
	if overlay := ProfilePath(profile); util.FileExists(overlay) {
		cfg, err = mergeFile(cfg, overlay)
		if err != nil {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	return cfg, nil
}

// Load reads JSON from path. A missing file returns Default() with a nil error.
func Load(path string) (File, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// mergeFile unmarshals path into base (JSON overlay; later keys win).
func mergeFile(base File, path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return base, err
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return base, err
	}
	return base, nil
}

// applyEnv overlays GMCLI_LOG_* environment variables onto cfg.
func applyEnv(cfg *File) {
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogLevel)); v != "" {
		cfg.Log.Level = v
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogDir)); v != "" {
		cfg.Log.Dir = v
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogFile)); v != "" {
		cfg.Log.RuntimeFile = v
		cfg.Log.File = v
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogFilePattern)); v != "" {
		cfg.Log.FilePattern = v
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogMaxDays)); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.Log.MaxDays = n
		}
	}
	if v := strings.TrimSpace(os.Getenv(consts.EnvLogConsole)); v != "" {
		if on, ok := util.ParseBool(v); ok {
			cfg.Log.Console = on
		}
	}
}

// Save writes cfg as indented JSON with mode 0600 (creates parent dirs).
func Save(path string, cfg File) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return util.WriteFile(path, data, 0o600)
}
