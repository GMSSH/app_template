package input

import (
	"strconv"
	"strings"

	"gmssh-cli-plugin/internal/util"
)

// ArgType is the declared type of a command flag.
type ArgType string

const (
	TypeString  ArgType = "string"
	TypeInt     ArgType = "int"
	TypeBool    ArgType = "bool"
	TypeStrings ArgType = "string[]"
)

// Arg describes one command option shown in help and parsed by Parse.
// Desc is an i18n key. Min/Max of 0 means "no bound".
//
// Semantics of Min/Max:
//   - TypeInt     → numeric range
//   - TypeString  → trimmed rune length
//   - TypeStrings → CSV item count
//
// Example:
//
//	command.Arg{Name: "text", Type: command.TypeString, Required: true, Desc: "arg.hello.text"}
type Arg struct {
	Name     string
	Type     ArgType
	Required bool
	Default  string
	Desc     string
	Enum     []string
	Min      int
	Max      int
}

// Input is the typed view of parsed command flags for handlers.
//
//	text := in.Str("text")
//	n    := in.Int("count")
//	on   := in.Bool("force")
//	tags := in.List("tags")
//	if in.IsSet("mode") { ... }
type Input struct {
	values map[string]string
	set    map[string]bool
}

// Str returns the raw string value for name (empty if unset).
func (in Input) Str(name string) string { return in.values[name] }

// Int parses name as int; invalid values become 0.
func (in Input) Int(name string) int {
	val, err := strconv.Atoi(strings.TrimSpace(in.values[name]))
	if err != nil {
		return 0
	}
	return val
}

// Bool parses name as bool; invalid values become false.
func (in Input) Bool(name string) bool {
	val, ok := util.ParseBool(in.values[name])
	if !ok {
		return false
	}
	return val
}

// List splits a comma-separated string[] value.
func (in Input) List(name string) []string {
	return util.SplitCSV(in.values[name])
}

// IsSet reports whether the flag appeared on the command line.
func (in Input) IsSet(name string) bool { return in.set[name] }

// Values returns a map of all flag values (never nil). Callers must not mutate.
func (in Input) Values() map[string]string {
	if in.values == nil {
		return map[string]string{}
	}
	return in.values
}
