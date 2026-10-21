package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"gmssh-cli-plugin/internal/config"
	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
	"gmssh-cli-plugin/internal/output"
	"gmssh-cli-plugin/internal/paths"
	"gmssh-cli-plugin/internal/store"
)

// Main is the process entry used by cmd/main.
// argv excludes the program name (pass os.Args[1:]).
//
//	func main() { os.Exit(command.Main(os.Args[1:])) }
func Main(argv []string) int { return Run(os.Stdout, os.Stderr, argv) }

// Run executes one CLI invocation against the given writers.
// Tests inject buffers; production uses os.Stdout / os.Stderr via Main.
//
// Pipeline: recover panic → bootstrap (globals, i18n, config, logging)
// → dispatch (help / lookup) → execute (parse → validate → middleware chain).
func Run(stdout, stderr io.Writer, argv []string) int {
	return newApp(stdout, stderr).run(argv)
}

// app is the per-invocation engine state (writers + output mode).
type app struct {
	stdout, stderr io.Writer
	format         middleware.Format
	compact        bool
	encoding       string
}

// session carries post-bootstrap state so dispatch / execute stay focused.
type session struct {
	globals globalOptions
	command string
	rest    []string
	cfg     config.File
}

func newApp(stdout, stderr io.Writer) *app {
	return &app{stdout: stdout, stderr: stderr}
}

// run is a short pipeline: recover → bootstrap → dispatch.
func (a *app) run(argv []string) (code int) {
	defer a.recoverPanic(&code)
	s, code, ok := a.bootstrap(argv)
	if !ok {
		return code
	}
	return a.dispatch(s)
}

// recoverPanic turns panics into ExitError with a localized internal-error message.
func (a *app) recoverPanic(code *int) {
	recovered := recover()
	if recovered == nil {
		return
	}
	logging.Errorf("panic: %v\n%s", recovered, debug.Stack())
	*code = a.fail(ExitError, ErrTypeRuntime, T("cli.internal_error", recovered), "cli.internal_error", "", nil)
}

// bootstrap parses globals, applies language, loads config, and configures logging.
// ok=false means the caller must return code immediately.
func (a *app) bootstrap(argv []string) (session, int, bool) {
	g, command, rest, gerr := parseGlobals(argv)

	format, compact, ferr := resolveOutputFormat(g)
	a.format = format
	a.compact = compact
	if ferr != nil && gerr == nil {
		gerr = ferr
	}

	encName, encErr := resolveEncoding(g)
	a.encoding = encName
	if encErr != nil && gerr == nil {
		gerr = encErr
	}
	if enc, _, err := output.ResolveEncoding(encName); err == nil {
		a.stdout = output.NewEncodedWriter(a.stdout, enc)
		a.stderr = output.NewEncodedWriter(a.stderr, enc)
	}

	if err := applyLanguage(g.lang); err != nil {
		return session{}, a.failErr(ExitUsage, ErrTypeUsage, err), false
	}

	cfg, cfgErr := config.LoadAll()
	settings, logErr := resolveLog(g, cfg)
	settings.ConsoleOut = a.stderr
	logging.Configure(settings)

	if code, ok := a.bootstrapErrors(cfgErr, logErr, gerr); !ok {
		return session{}, code, false
	}
	return session{globals: g, command: command, rest: rest, cfg: cfg}, ExitOK, true
}

// bootstrapErrors maps config / log / global-flag errors to exit codes.
func (a *app) bootstrapErrors(cfgErr, logErr, gerr error) (int, bool) {
	if cfgErr != nil {
		return a.fail(ExitError, ErrTypeRuntime, T("cli.config_error", cfgErr), "cli.config_error", "", cfgErr), false
	}
	if logErr != nil {
		return a.failErr(ExitUsage, ErrTypeUsage, ensureUserError(logErr)), false
	}
	if gerr != nil {
		return a.failErr(ExitUsage, ErrTypeUsage, gerr), false
	}
	return ExitOK, true
}

// dispatch handles help, command lookup, validation, execution, and rendering.
// Prefer middleware for new cross-cutting behavior instead of growing this method.
func (a *app) dispatch(s session) int {
	if code, done := a.handleHelp(s); done {
		return code
	}
	cmd := lookup(s.command)
	if cmd == nil {
		return a.fail(ExitUsage, ErrTypeUsage, T("cli.unknown_command", s.command), "cli.unknown_command", "", nil)
	}
	if s.globals.help {
		printCommandHelp(a.stdout, cmd)
		return ExitOK
	}
	return a.execute(cmd, s)
}

// handleHelp prints root help when --help or when no command was given.
func (a *app) handleHelp(s session) (int, bool) {
	if s.globals.help && s.command == "" {
		printRootHelp(a.stdout)
		return ExitOK, true
	}
	if s.command == "" {
		printRootHelp(a.stderr)
		return a.fail(ExitUsage, ErrTypeUsage, T("cli.hint_missing_command"), "cli.hint_missing_command", "", nil), true
	}
	return ExitOK, false
}

// execute parses args, builds Context, validates, then runs the middleware chain.
func (a *app) execute(cmd *Command, s session) int {
	in, err := input.Parse(cmd.Name, cmd.Args, s.rest)
	if err != nil {
		return a.usage(cmd, err)
	}
	ctx := &Context{
		Name:     cmd.Name,
		Lang:     i18n.Language(),
		Stdout:   a.stdout,
		Stderr:   a.stderr,
		Config:   s.cfg,
		Store:    store.New(paths.DataDir()),
		Compact:  a.compact,
		Format:   a.format,
		Encoding: a.encoding,
	}
	if err := a.validate(cmd, ctx, in); err != nil {
		return a.mapExecError(cmd, err)
	}
	_, err = middleware.Chain(cmd.Run, cmd.Use)(ctx, in)
	if err != nil {
		return a.mapExecError(cmd, err)
	}
	return ExitOK
}

// validate runs the command's Validate hook when present.
func (a *app) validate(cmd *Command, ctx *Context, in Input) error {
	if cmd.Validate == nil {
		return nil
	}
	return cmd.Validate(ctx, in)
}

// mapExecError routes UsageError → exit 2 (with help), other errors → exit 1.
// Plain errors are wrapped with cli.runtime_error so clients see i18n text.
func (a *app) mapExecError(cmd *Command, err error) int {
	var usage *UsageError
	if errors.As(err, &usage) {
		return a.usage(cmd, err)
	}
	return a.failErr(ExitError, ErrTypeRuntime, ensureUserError(err))
}

// usage prints command help (text mode only) then a usage failure message.
func (a *app) usage(cmd *Command, err error) int {
	if a.format == middleware.FormatText {
		printCommandHelp(a.stderr, cmd)
		_, _ = fmt.Fprintln(a.stderr)
	}
	return a.failErr(ExitUsage, ErrTypeUsage, err)
}

// failErr writes a failure using localized message, i18n key, and frame.
func (a *app) failErr(code int, kind string, err error) int {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return a.fail(code, kind, msg, errorKey(err), errorFrame(err), errorCause(err))
}

// fail writes a failure to stderr and returns code.
// JSON (default): {"ok":false,"error":{code,type,message,key?,frame?}}
// Text: plain localized message. Logs keep frame + underlying cause.
func (a *app) fail(code int, kind, message, key, frame string, cause error) int {
	switch {
	case frame != "" && cause != nil:
		logging.Errorf("exit %d (%s): %s at %s cause=%v", code, kind, message, frame, cause)
	case frame != "":
		logging.Errorf("exit %d (%s): %s at %s", code, kind, message, frame)
	case cause != nil:
		logging.Errorf("exit %d (%s): %s cause=%v", code, kind, message, cause)
	default:
		logging.Errorf("exit %d (%s): %s", code, kind, message)
	}
	if a.format == middleware.FormatText {
		_, _ = fmt.Fprintln(a.stderr, message)
		return code
	}
	if err := output.WriteJSON(a.stderr, a.stderr, output.Err(code, kind, message, key, frame), a.compact); err != nil {
		_, _ = fmt.Fprintln(a.stderr, message)
	}
	return code
}

// applyLanguage resolves --lang / GMCLI_LANG and updates the i18n package state.
func applyLanguage(flagLang string) error {
	lang := strings.TrimSpace(flagLang)
	if lang == "" {
		lang = os.Getenv(i18n.EnvLang)
	}
	if lang != "" && !i18n.Supported(lang) {
		i18n.SetLanguage("")
		return UsageFail("cli.invalid_lang", lang, strings.Join(i18n.Available(), ", "))
	}
	i18n.SetLanguage(lang)
	return nil
}

// resolveLog merges config + env + CLI flag overlays into logging.Settings.
func resolveLog(g globalOptions, cfg config.File) (logging.Settings, error) {
	base := logging.Settings{
		Dir:          strings.TrimSpace(cfg.Log.Dir),
		RuntimeFile:  strings.TrimSpace(cfg.Log.RuntimeFileOrDefault()),
		BusinessFile: strings.TrimSpace(cfg.Log.BusinessFileOrDefault()),
		Pattern:      strings.TrimSpace(cfg.Log.FilePattern),
		MaxDays:      cfg.Log.MaxDays,
		Console:      cfg.Log.Console,
	}
	if level, ok := logging.ParseLevel(cfg.Log.Level); ok {
		base.Level = level
	} else {
		base.Level = logging.Error
	}
	settings, err := logging.Resolve(base, logging.Overlay{
		Level:      g.logLevel,
		Dir:        g.logDir,
		File:       g.logFile,
		MaxDays:    g.logMaxDays,
		MaxDaysSet: g.logMaxDaysSet,
		Debug:      g.debug,
		Console:    g.logConsole,
		ConsoleSet: g.logConsoleSet,
	})
	if err != nil {
		return settings, mapLogResolveError(err)
	}
	return settings, nil
}

// mapLogResolveError turns logging.ResolveError into localized UsageFail errors.
func mapLogResolveError(err error) error {
	var re *logging.ResolveError
	if !errors.As(err, &re) {
		return err
	}
	switch re.Field {
	case "level":
		return UsageFail("cli.invalid_log_level", re.Value)
	case "max_days":
		return UsageFail("cli.invalid_log_max_days", re.Value)
	case "file_pattern":
		return UsageFail("cli.invalid_log_file_pattern", re.Value)
	case "console":
		return UsageFail("cli.invalid_log_console", re.Value)
	default:
		return err
	}
}
