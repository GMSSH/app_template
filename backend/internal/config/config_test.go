package config

import (
	"os"
	"path/filepath"
	"testing"

	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/paths"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level != "error" || cfg.Log.Dir != "../../logs" || cfg.Log.RuntimeFileOrDefault() != consts.DefaultRuntimeLogFile || cfg.Log.MaxDays != 7 || cfg.Log.Console {
		t.Fatalf("default config mismatch: %+v", cfg)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := File{Log: Log{Level: "debug", Dir: "/var/log/x", MaxDays: 3}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Log.Level != "debug" || got.Log.Dir != "/var/log/x" || got.Log.MaxDays != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestPathPrefersDataOverConfigs(t *testing.T) {
	root := t.TempDir()
	paths.SetRoot(root)
	t.Setenv(consts.EnvConfig, "")
	dataDir := filepath.Join(root, "data")
	configsDir := filepath.Join(root, "configs")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dataCfg := filepath.Join(dataDir, "config.json")
	sourceCfg := filepath.Join(configsDir, "config.json")
	if err := os.WriteFile(dataCfg, []byte(`{"log":{"level":"info"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceCfg, []byte(`{"log":{"level":"debug"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Path(); got != dataCfg {
		t.Fatalf("Path()=%s want data %s", got, dataCfg)
	}
}

func TestPathFallsBackToConfigs(t *testing.T) {
	root := t.TempDir()
	paths.SetRoot(root)
	t.Setenv(consts.EnvConfig, "")
	configsDir := filepath.Join(root, "configs")
	if err := os.MkdirAll(configsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sourceCfg := filepath.Join(configsDir, "config.json")
	if err := os.WriteFile(sourceCfg, []byte("{}"+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Path(); got != sourceCfg {
		t.Fatalf("Path()=%s want %s", got, sourceCfg)
	}
}

func TestPathUsesEnv(t *testing.T) {
	root := t.TempDir()
	paths.SetRoot(root)
	custom := filepath.Join(root, "custom.json")
	t.Setenv(consts.EnvConfig, custom)
	if got := Path(); got != custom {
		t.Fatalf("Path()=%s want %s", got, custom)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("期望 JSON 错误")
	}
}
