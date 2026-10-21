// Package input defines command-argument schemas, parsing, and usage errors.
//
// Single responsibility: turn argv + Arg definitions into a typed Input, or a
// UsageError. It does not print help/output and does not own business rules
// beyond required/enum/range checks declared on Arg.
//
// Composable business validation belongs in package validate (Builder.Rules).
//
// Example:
//
//	args := []input.Arg{{
//	    Name: "text", Type: input.TypeString, Required: true, Desc: "arg.hello.text",
//	}}
//	in, err := input.Parse("hello", args, []string{"--text", "world"})
//	if err != nil { ... }
//	name := in.Str("text")
//
// Usage errors (exit code 2):
//
//	return input.UsageFail("cli.missing_required", "text")
package input

import (
	"flag"
	"strconv"
	"strings"
	"unicode/utf8"

	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/util"
)

type listValue struct{ vals []string }

func (l *listValue) String() string { return strings.Join(l.vals, ",") }

func (l *listValue) Set(val string) error {
	l.vals = append(l.vals, util.SplitCSV(val)...)
	return nil
}

type flagBags struct {
	strings map[string]*string
	ints    map[string]*int
	bools   map[string]*bool
	lists   map[string]*listValue
}

// Parse builds a FlagSet from args, normalizes argv, then validates into Input.
// name is the command name (used only for FlagSet identity). argv is the
// command-local remainder after global flags were stripped by the engine.
func Parse(name string, args []Arg, argv []string) (Input, error) {
	fs, bags := newFlagSet(name, args)
	normalized, err := normalizeArgv(args, argv)
	if err != nil {
		return Input{}, err
	}
	if err := fs.Parse(normalized); err != nil {
		return Input{}, &UsageError{Msg: i18n.T("cli.flag_error", strings.TrimPrefix(err.Error(), "flag: "))}
	}
	if fs.NArg() > 0 {
		return Input{}, UsageFail("cli.unknown_arg", fs.Arg(0))
	}
	return collectInput(fs, bags, args)
}

func newFlagSet(name string, args []Arg) (*flag.FlagSet, flagBags) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	fs.Usage = func() {}
	bags := flagBags{
		strings: map[string]*string{},
		ints:    map[string]*int{},
		bools:   map[string]*bool{},
		lists:   map[string]*listValue{},
	}
	for _, arg := range args {
		registerArg(fs, &bags, arg)
	}
	return fs, bags
}

func registerArg(fs *flag.FlagSet, bags *flagBags, arg Arg) {
	switch arg.Type {
	case TypeInt:
		def, _ := strconv.Atoi(arg.Default)
		bags.ints[arg.Name] = fs.Int(arg.Name, def, arg.Desc)
	case TypeBool:
		def, _ := strconv.ParseBool(arg.Default)
		bags.bools[arg.Name] = fs.Bool(arg.Name, def, arg.Desc)
	case TypeStrings:
		bags.lists[arg.Name] = &listValue{}
		fs.Var(bags.lists[arg.Name], arg.Name, arg.Desc)
	default:
		bags.strings[arg.Name] = fs.String(arg.Name, arg.Default, arg.Desc)
	}
}

func collectInput(fs *flag.FlagSet, bags flagBags, args []Arg) (Input, error) {
	in := Input{values: map[string]string{}, set: map[string]bool{}}
	seen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	for _, arg := range args {
		in.values[arg.Name] = readFlagValue(bags, arg)
		in.set[arg.Name] = seen[arg.Name]
		if err := validateArg(arg, in); err != nil {
			return Input{}, err
		}
	}
	return in, nil
}

func readFlagValue(bags flagBags, arg Arg) string {
	switch arg.Type {
	case TypeInt:
		return strconv.Itoa(*bags.ints[arg.Name])
	case TypeBool:
		return strconv.FormatBool(*bags.bools[arg.Name])
	case TypeStrings:
		return bags.lists[arg.Name].String()
	default:
		return *bags.strings[arg.Name]
	}
}

func validateArg(arg Arg, in Input) error {
	if arg.Required && strings.TrimSpace(in.values[arg.Name]) == "" {
		return UsageFail("cli.missing_required", arg.Name)
	}
	if len(arg.Enum) > 0 && in.set[arg.Name] && !util.ContainsString(arg.Enum, in.values[arg.Name]) {
		return UsageFail("cli.invalid_enum", arg.Name, in.values[arg.Name], strings.Join(arg.Enum, ", "))
	}
	return checkRange(arg, in)
}

func checkRange(arg Arg, in Input) error {
	if arg.Min == 0 && arg.Max == 0 {
		return nil
	}
	switch arg.Type {
	case TypeInt:
		return checkIntRange(arg, in.Int(arg.Name))
	case TypeStrings:
		return checkLenRange(arg, len(in.List(arg.Name)))
	default:
		// String length uses trimmed runes (aligned with validate.LengthBetween).
		n := utf8.RuneCountInString(strings.TrimSpace(in.Str(arg.Name)))
		return checkLenRange(arg, n)
	}
}

func checkIntRange(arg Arg, n int) error {
	if arg.Min != 0 && n < arg.Min {
		return UsageFail("cli.too_small", arg.Name, arg.Min)
	}
	if arg.Max != 0 && n > arg.Max {
		return UsageFail("cli.too_large", arg.Name, arg.Max)
	}
	return nil
}

func checkLenRange(arg Arg, n int) error {
	if arg.Min != 0 && n < arg.Min {
		return UsageFail("cli.too_short", arg.Name, arg.Min)
	}
	if arg.Max != 0 && n > arg.Max {
		return UsageFail("cli.too_long", arg.Name, arg.Max)
	}
	return nil
}

func normalizeArgv(args []Arg, argv []string) ([]string, error) {
	argType := make(map[string]ArgType, len(args))
	for _, arg := range args {
		argType[arg.Name] = arg.Type
	}
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		next, stop, err := normalizeToken(argType, argv, i, &out)
		if err != nil {
			return nil, err
		}
		if stop {
			break
		}
		i = next
	}
	return out, nil
}

func normalizeToken(argType map[string]ArgType, argv []string, i int, out *[]string) (int, bool, error) {
	item := argv[i]
	if item == "--" {
		*out = append(*out, argv[i:]...)
		return i, true, nil
	}
	if !strings.HasPrefix(item, "-") || item == "-" {
		return i, false, UsageFail("cli.unknown_arg", item)
	}
	name, _, hasValue := SplitFlag(item)
	if hasValue {
		*out = append(*out, item)
		return i, false, nil
	}
	return normalizeNamedFlag(argType, argv, i, name, out)
}

func normalizeNamedFlag(argType map[string]ArgType, argv []string, i int, name string, out *[]string) (int, bool, error) {
	typ, isArg := argType[name]
	if isArg && typ == TypeBool {
		if literal, ok := PeekBoolLiteral(argv, i); ok {
			*out = append(*out, "--"+name+"="+literal)
			return i + 1, false, nil
		}
		*out = append(*out, argv[i])
		return i, false, nil
	}
	if isArg {
		next, ok := PeekFlagValue(argv, i)
		if !ok {
			return i, false, UsageFail("cli.missing_value", name)
		}
		*out = append(*out, "--"+name+"="+next)
		return i + 1, false, nil
	}
	*out = append(*out, argv[i])
	return i, false, nil
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
