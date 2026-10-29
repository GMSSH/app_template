package command

import (
	"os"
	"strconv"
	"strings"

	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
	"gmssh-cli-plugin/internal/output"
)

// Global flags are declared in globalCatalog and applied via setter tables so
// new flags do not require edits to the scan loop (Open/Closed).
//
// To add a global flag:
//  1. Append a globalSpec to globalCatalog
//  2. Add a setter in boolSetters or valueSetters
//  3. Add an i18n help key to rootGlobalHelpKeys in help.go
//
// Example — new boolean --trace:
//
//	globalCatalog: {key: "trace", names: []string{"trace"}, kind: kindBool}
//	boolSetters:   "trace": func(g *globalOptions, on bool) { g.trace = on }

type flagKind int

const (
	kindBool flagKind = iota
	kindValue
)

// globalSpec describes one process-wide flag and its aliases.
type globalSpec struct {
	key   string   // stable key used by setter maps
	names []string // CLI names without leading dashes (e.g. "lang", "l")
	kind  flagKind
}

// globalCatalog is the single directory of process-wide flags.
var globalCatalog = []globalSpec{
	{key: "format", names: []string{"format"}, kind: kindValue},
	{key: "json", names: []string{"json"}, kind: kindBool}, // legacy; prefer --format
	{key: "compact", names: []string{"compact"}, kind: kindBool},
	{key: "encoding", names: []string{"encoding"}, kind: kindValue},
	{key: "debug", names: []string{"debug"}, kind: kindBool},
	{key: "lang", names: []string{"lang", "l"}, kind: kindValue},
	{key: "log-level", names: []string{"log-level"}, kind: kindValue},
	{key: "log-dir", names: []string{"log-dir"}, kind: kindValue},
	{key: "log-file", names: []string{"log-file"}, kind: kindValue},
	{key: "log-max-days", names: []string{"log-max-days"}, kind: kindValue},
	{key: "log-console", names: []string{"log-console"}, kind: kindBool},
}

var globalByName = func() map[string]globalSpec {
	out := make(map[string]globalSpec, len(globalCatalog)*2)
	for _, spec := range globalCatalog {
		for _, name := range spec.names {
			out[name] = spec
		}
	}
	return out
}()

// globalOptions holds parsed process-wide flags for one invocation.
type globalOptions struct {
	format                                 string
	formatSet                              bool
	json, compact, debug, help, logConsole bool
	jsonSet, compactSet, logConsoleSet     bool
	encoding                               string
	encodingSet                            bool
	lang, logLevel, logDir, logFile        string
	logMaxDays                             int
	logMaxDaysSet                          bool
}

// boolSetters / valueSetters map catalog keys to option writers (Strategy tables).
var boolSetters = map[string]func(*globalOptions, bool){
	"json":        func(g *globalOptions, on bool) { g.json = on; g.jsonSet = true },
	"compact":     func(g *globalOptions, on bool) { g.compact = on; g.compactSet = true },
	"debug":       func(g *globalOptions, on bool) { g.debug = on },
	"log-console": func(g *globalOptions, on bool) { g.logConsole = on; g.logConsoleSet = true },
}

var valueSetters = map[string]func(*globalOptions, string) error{
	"format": func(g *globalOptions, raw string) error {
		g.format = raw
		g.formatSet = true
		return nil
	},
	"encoding": func(g *globalOptions, raw string) error {
		g.encoding = raw
		g.encodingSet = true
		return nil
	},
	"lang":      func(g *globalOptions, raw string) error { g.lang = raw; return nil },
	"log-level": func(g *globalOptions, raw string) error { g.logLevel = raw; return nil },
	"log-dir":   func(g *globalOptions, raw string) error { g.logDir = raw; return nil },
	"log-file":  func(g *globalOptions, raw string) error { g.logFile = raw; return nil },
	"log-max-days": func(g *globalOptions, raw string) error {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return UsageFail("cli.invalid_log_max_days", raw)
		}
		g.logMaxDays = n
		g.logMaxDaysSet = true
		return nil
	},
}

func lookupGlobal(name string) (globalSpec, bool) {
	spec, ok := globalByName[name]
	return spec, ok
}

func isHelpToken(token string) bool {
	switch token {
	case "-h", "--h", "-help", "--help":
		return true
	default:
		return false
	}
}

// parseGlobals scans argv for process-wide flags and the first command token.
// Remaining tokens after the command name are returned as rest for input.Parse.
func parseGlobals(argv []string) (g globalOptions, command string, rest []string, err error) {
	rest = make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		next, stop, applyErr := consumeGlobalToken(&g, &command, &rest, argv, i)
		if applyErr != nil && err == nil {
			err = applyErr
		}
		if stop {
			break
		}
		i = next
	}
	return g, command, rest, err
}

// resolveOutputFormat picks the response format.
// Priority: --format; --json/--json=false; GMCLI_FORMAT; default json (compact).
// Compact defaults to true for JSON; override with --compact=false for pretty indent.
func resolveOutputFormat(g globalOptions) (middleware.Format, bool, error) {
	raw := ""
	switch {
	case g.formatSet:
		raw = g.format
	case g.jsonSet:
		if g.json {
			raw = "json"
		} else {
			raw = "text"
		}
	default:
		raw = strings.TrimSpace(os.Getenv(consts.EnvFormat))
	}
	if raw == "" {
		raw = "json"
	}
	format, err := parseOutputFormat(raw)
	if err != nil {
		return middleware.FormatJSON, true, err
	}
	compact := true
	if format == middleware.FormatText {
		compact = false
	} else if g.compactSet {
		compact = g.compact
	}
	return format, compact, nil
}

func parseOutputFormat(raw string) (middleware.Format, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "json":
		return middleware.FormatJSON, nil
	case "text":
		return middleware.FormatText, nil
	default:
		return middleware.FormatJSON, UsageFail("cli.invalid_format", raw)
	}
}

// resolveEncoding picks stdout/stderr charset.
// Priority: --encoding; GMCLI_ENCODING; default utf-8.
func resolveEncoding(g globalOptions) (string, error) {
	raw := ""
	if g.encodingSet {
		raw = g.encoding
	} else {
		raw = strings.TrimSpace(os.Getenv(consts.EnvEncoding))
	}
	_, name, err := output.ResolveEncoding(raw)
	if err != nil {
		return "", UsageFail("cli.invalid_encoding", raw)
	}
	return name, nil
}

// consumeGlobalToken handles one argv item. stop=true ends the scan.
func consumeGlobalToken(g *globalOptions, command *string, rest *[]string, argv []string, i int) (int, bool, error) {
	item := argv[i]
	if item == "--" {
		return i, true, handleDoubleDash(command, rest, argv, i)
	}
	if isHelpToken(item) {
		g.help = true
		return i, false, nil
	}
	if strings.HasPrefix(item, "-") && item != "-" {
		return consumeGlobalFlag(g, command, rest, argv, i, item)
	}
	if *command == "" {
		*command = item
		return i, false, nil
	}
	*rest = append(*rest, item)
	return i, false, nil
}

func handleDoubleDash(command *string, rest *[]string, argv []string, i int) error {
	if *command == "" && i+1 < len(argv) {
		return UsageFail("cli.unknown_arg", argv[i+1])
	}
	*rest = append(*rest, argv[i:]...)
	return nil
}

func consumeGlobalFlag(g *globalOptions, command *string, rest *[]string, argv []string, i int, item string) (int, bool, error) {
	name, value, hasValue := input.SplitFlag(item)
	spec, ok := lookupGlobal(name)
	if !ok {
		if *command == "" {
			return i, false, UsageFail("cli.unknown_flag", input.UnknownFlagLabel(item))
		}
		*rest = append(*rest, item)
		return i, false, nil
	}
	next, err := applyGlobal(g, spec, name, value, hasValue, argv, i)
	return next, false, err
}

func applyGlobal(g *globalOptions, spec globalSpec, name, value string, hasValue bool, argv []string, i int) (int, error) {
	if spec.kind == kindBool {
		return applyBoolGlobal(g, spec, value, hasValue, argv, i)
	}
	return applyValueGlobal(g, spec, name, value, hasValue, argv, i)
}

func applyBoolGlobal(g *globalOptions, spec globalSpec, value string, hasValue bool, argv []string, i int) (int, error) {
	on := true
	if hasValue {
		if parsed, ok := input.BoolLiteralValue(value); ok {
			on = parsed == "true"
		}
	} else if literal, ok := input.PeekBoolLiteral(argv, i); ok {
		on = literal == "true"
		i++
	}
	if set, ok := boolSetters[spec.key]; ok {
		set(g, on)
	}
	return i, nil
}

func applyValueGlobal(g *globalOptions, spec globalSpec, name, value string, hasValue bool, argv []string, i int) (int, error) {
	raw := value
	if !hasValue {
		next, ok := input.PeekFlagValue(argv, i)
		if !ok {
			return i, UsageFail("cli.missing_value", name)
		}
		raw = next
		i++
	}
	if set, ok := valueSetters[spec.key]; ok {
		return i, set(g, raw)
	}
	return i, nil
}
