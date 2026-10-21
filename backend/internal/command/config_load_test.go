package command_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/paths"
)

func TestRootConfigCreatesLogs(t *testing.T) {
	root := t.TempDir()
	paths.SetRoot(root)
	t.Setenv(consts.EnvRoot, root)
	t.Setenv(consts.EnvConfig, "")
	t.Setenv(consts.EnvLogLevel, "")
	t.Setenv(consts.EnvLogDir, "")
	t.Setenv(consts.EnvLogFile, "")
	t.Setenv(consts.EnvLogFilePattern, "")
	t.Setenv(consts.EnvLogMaxDays, "")
	t.Setenv(consts.EnvLogConsole, "")
	t.Setenv(i18n.EnvLang, "")
	i18n.SetLanguage("")
	t.Cleanup(func() { i18n.SetLanguage("") })

	cfg := "{\"log\":{\"level\":\"debug\",\"dir\":\"../../logs\",\"file\":\"2006-01-02.log\",\"max_days\":7,\"console\":false}}\n"
	configsDir := filepath.Join(root, "configs")
	if err := os.MkdirAll(configsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configsDir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := command.Run(&stdout, &stderr, []string{"ping"})
	if code != command.ExitOK {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	dir := filepath.Join(root, "logs")
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		t.Fatalf("logs dir not created: %s %v", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected log file in %s: %v", dir, err)
	}
}
