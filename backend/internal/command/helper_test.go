package command_test

import (
	"bytes"
	"strings"
	"testing"

	_ "gmssh-cli-plugin/internal/biz/hello"
	_ "gmssh-cli-plugin/internal/biz/ping"
	_ "gmssh-cli-plugin/internal/biz/version"

	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/paths"
)

type cliResult struct {
	stdout string
	stderr string
	code   int
}

func runCLI(t *testing.T, argv ...string) cliResult {
	t.Helper()
	return runCLIEnv(t, nil, argv...)
}

func runCLIEnv(t *testing.T, env map[string]string, argv ...string) cliResult {
	t.Helper()
	root := t.TempDir()
	paths.SetRoot(root)
	t.Setenv(i18n.EnvLang, "")
	t.Setenv(i18n.EnvI18nDir, "")
	t.Setenv(consts.EnvLogLevel, "off")
	t.Setenv(consts.EnvRoot, root)
	t.Setenv(consts.EnvConfig, "")
	t.Setenv(consts.EnvFormat, "")
	t.Setenv(consts.EnvEncoding, "")
	t.Setenv(consts.EnvLogDir, "")
	t.Setenv(consts.EnvLogFile, "")
	t.Setenv(consts.EnvLogFilePattern, "")
	t.Setenv(consts.EnvLogMaxDays, "")
	t.Setenv(consts.EnvLogConsole, "")
	for k, v := range env {
		t.Setenv(k, v)
	}
	i18n.SetLanguage("")
	t.Cleanup(func() { i18n.SetLanguage("") })

	var stdout, stderr bytes.Buffer
	code := command.Run(&stdout, &stderr, argv)
	return cliResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func (r cliResult) requireCode(t *testing.T, code int) cliResult {
	t.Helper()
	if r.code != code {
		t.Fatalf("exit=%d want=%d stdout=%q stderr=%q", r.code, code, r.stdout, r.stderr)
	}
	return r
}

func (r cliResult) requireStdout(t *testing.T, substr string) cliResult {
	t.Helper()
	if !strings.Contains(r.stdout, substr) {
		t.Fatalf("stdout missing %q: %q", substr, r.stdout)
	}
	return r
}

func (r cliResult) requireStderr(t *testing.T, substr string) cliResult {
	t.Helper()
	if !strings.Contains(r.stderr, substr) {
		t.Fatalf("stderr missing %q: %q", substr, r.stderr)
	}
	return r
}

func (r cliResult) requireExactStdout(t *testing.T, want string) cliResult {
	t.Helper()
	if strings.TrimSpace(r.stdout) != want {
		t.Fatalf("stdout=%q want=%q", r.stdout, want)
	}
	return r
}
