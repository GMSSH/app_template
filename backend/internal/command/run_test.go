package command_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/paths"
)

func TestCLI(t *testing.T) {
	cases := []struct {
		name        string
		argv        []string
		code        int
		stdoutExact string
		stdoutHas   []string
		stdoutNot   []string
		stderrHas   []string
		emptyStdout bool
	}{
		{name: "ping default json", argv: []string{"ping"}, code: command.ExitOK, stdoutExact: `{"ok":true,"data":"pong"}`},
		{name: "ping text", argv: []string{"--format", "text", "ping"}, code: command.ExitOK, stdoutExact: "pong"},
		{name: "hello json envelope", argv: []string{"hello", "--text", "world"}, code: command.ExitOK, stdoutHas: []string{`"ok":true`, `"text":"hello, world"`}},
		{name: "hello utf8 chinese", argv: []string{"hello", "--text", "张三"}, code: command.ExitOK, stdoutHas: []string{`"ok":true`, `张三`}},
		{name: "pretty json", argv: []string{"--compact=false", "ping"}, code: command.ExitOK, stdoutHas: []string{"{\n", `"ok": true`, `"data": "pong"`}},
		{name: "compact version", argv: []string{"version"}, code: command.ExitOK, stdoutHas: []string{`"ok":true`, `"version"`}},
		{name: "missing required", argv: []string{"hello"}, code: command.ExitUsage, emptyStdout: true, stderrHas: []string{`"ok":false`, `"type":"usage"`, `"code":2`}},
		{name: "unknown command", argv: []string{"no_such_cmd"}, code: command.ExitUsage, stderrHas: []string{"no_such_cmd", `"ok":false`}},
		{name: "unknown flag", argv: []string{"ping", "--bogus", "1"}, code: command.ExitUsage},
		{name: "lang zh error", argv: []string{"--lang", "zh-CN", "no_such_cmd"}, code: command.ExitUsage, stderrHas: []string{"未知命令"}},
		{name: "lang en error", argv: []string{"--lang", "en", "no_such_cmd"}, code: command.ExitUsage, stderrHas: []string{"Unknown command"}},
		{name: "help zh", argv: []string{"--lang", "zh-CN", "-h"}, code: command.ExitOK, stdoutHas: []string{"全局参数:", "--encoding"}},
		{name: "help en", argv: []string{"--lang", "en", "-h"}, code: command.ExitOK, stdoutHas: []string{"Global options:", "--encoding"}, stdoutNot: []string{"全局参数"}},
		{name: "command help en", argv: []string{"--lang", "en", "hello", "-h"}, code: command.ExitOK, stdoutHas: []string{"Arguments:", "required"}},
		{name: "command help zh", argv: []string{"--lang", "zh-CN", "hello", "-h"}, code: command.ExitOK, stdoutHas: []string{"参数:", "必填"}},
		{name: "invalid lang", argv: []string{"--format", "text", "--lang", "fr", "ping"}, code: command.ExitUsage, stderrHas: []string{"fr"}},
		{name: "space separated arg text", argv: []string{"--format", "text", "hello", "--text", "world"}, code: command.ExitOK, stdoutHas: []string{"text: hello, world"}},
		{name: "json false means text", argv: []string{"ping", "--json", "false"}, code: command.ExitOK, stdoutExact: "pong"},
		{name: "default lang english", argv: []string{"-h"}, code: command.ExitOK, stdoutHas: []string{"Global options:"}, stdoutNot: []string{"全局参数"}},
		{name: "invalid log level", argv: []string{"--format", "text", "--log-level", "nope", "ping"}, code: command.ExitUsage, stderrHas: []string{"nope"}},
		{name: "invalid format", argv: []string{"--format", "xml", "ping"}, code: command.ExitUsage, stderrHas: []string{"xml"}},
		{name: "invalid encoding", argv: []string{"--encoding", "latin1", "ping"}, code: command.ExitUsage, stderrHas: []string{"latin1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLI(t, tc.argv...)
			got.requireCode(t, tc.code)
			if tc.emptyStdout && got.stdout != "" {
				t.Fatalf("error wrote stdout: %q", got.stdout)
			}
			if tc.stdoutExact != "" {
				got.requireExactStdout(t, tc.stdoutExact)
			}
			for _, s := range tc.stdoutHas {
				got.requireStdout(t, s)
			}
			for _, s := range tc.stdoutNot {
				if strings.Contains(got.stdout, s) {
					t.Fatalf("stdout should not contain %q: %q", s, got.stdout)
				}
			}
			for _, s := range tc.stderrHas {
				got.requireStderr(t, s)
			}
			if tc.name == "compact version" && strings.Count(strings.TrimSpace(got.stdout), "\n") != 0 {
				t.Fatalf("default json should be one line: %q", got.stdout)
			}
		})
	}
}

func TestFormatEnv(t *testing.T) {
	got := runCLIEnv(t, map[string]string{consts.EnvFormat: "text"}, "ping")
	got.requireCode(t, command.ExitOK)
	got.requireExactStdout(t, "pong")
}

func TestEncodingGBK(t *testing.T) {
	got := runCLI(t, "--encoding", "gbk", "--format", "text", "hello", "--text", "张三")
	got.requireCode(t, command.ExitOK)
	// text mode: "text: 你好，张三" encoded as GBK (not UTF-8 bytes for 张)
	if strings.Contains(got.stdout, "张") {
		t.Fatalf("expected GBK bytes, got UTF-8 literal: %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "text:") {
		t.Fatalf("stdout=%q", got.stdout)
	}
}

func TestEncodingEnv(t *testing.T) {
	got := runCLIEnv(t, map[string]string{consts.EnvEncoding: "utf-8"}, "hello", "--text", "李四")
	got.requireCode(t, command.ExitOK)
	got.requireStdout(t, "李四")
}

func TestInvalidConfig(t *testing.T) {
	root := t.TempDir()
	paths.SetRoot(root)
	bad := filepath.Join(root, "config.json")
	if err := os.WriteFile(bad, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(consts.EnvConfig, bad)
	t.Setenv(consts.EnvLogLevel, "off")
	t.Setenv(i18n.EnvLang, "")
	i18n.SetLanguage("")
	t.Cleanup(func() { i18n.SetLanguage("") })
	var stdout, stderr bytes.Buffer
	code := command.Run(&stdout, &stderr, []string{"--compact", "ping"})
	if code != command.ExitError {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `"ok":false`) {
		t.Fatalf("expected json error envelope: %s", stderr.String())
	}
}

func TestDebugLogsRequestAndResponse(t *testing.T) {
	dir := t.TempDir()
	got := runCLI(t, "--format", "text", "--debug", "--log-dir", dir, "ping")
	got.requireCode(t, command.ExitOK)
	data, err := os.ReadFile(filepath.Join(dir, time.Now().Format("runtime-2006-01-02.log")))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "request command=ping") {
		t.Fatalf("missing request log: %s", body)
	}
	if !strings.Contains(body, "response command=ping") || !strings.Contains(body, "pong") {
		t.Fatalf("missing response log: %s", body)
	}
}

func TestCustomLogFileName(t *testing.T) {
	dir := t.TempDir()
	layout := "app-2006-01-02.log"
	got := runCLI(t, "--format", "text", "--debug", "--log-dir", dir, "--log-file", layout, "ping")
	got.requireCode(t, command.ExitOK)
	path := filepath.Join(dir, time.Now().Format(layout))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected custom log file %s: %v", path, err)
	}
}

func TestDebugImpliesConsole(t *testing.T) {
	got := runCLI(t, "--format", "text", "--debug", "ping")
	got.requireCode(t, command.ExitOK)
	got.requireExactStdout(t, "pong")
	got.requireStderr(t, "request command=ping")
	got.requireStderr(t, "response command=ping")
}

func TestDebugLogConsoleFalse(t *testing.T) {
	got := runCLI(t, "--format", "text", "--debug", "--log-console=false", "ping")
	got.requireCode(t, command.ExitOK)
	got.requireExactStdout(t, "pong")
	if strings.Contains(got.stderr, "request command=") {
		t.Fatalf("console should be off: %q", got.stderr)
	}
}

func TestInvalidLogConsoleEnv(t *testing.T) {
	root := t.TempDir()
	paths.SetRoot(root)
	t.Setenv(consts.EnvConfig, "")
	t.Setenv(consts.EnvLogLevel, "off")
	t.Setenv(consts.EnvLogConsole, "maybe")
	t.Setenv(i18n.EnvLang, "")
	i18n.SetLanguage("")
	t.Cleanup(func() { i18n.SetLanguage("") })
	var stdout, stderr bytes.Buffer
	code := command.Run(&stdout, &stderr, []string{"--format", "text", "ping"})
	if code != command.ExitUsage {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "maybe") {
		t.Fatalf("stderr=%s", stderr.String())
	}
}
