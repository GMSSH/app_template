// Package command is the CLI engine (framework layer).
//
// It owns process bootstrap, global flags, command registry, help, and the
// execute pipeline. It must NEVER import internal/biz — business packages
// blank-import themselves from cmd/main and self-register via init.
//
// Files:
//   - command.go — Command type + registry
//   - builder.go — fluent Define(...).Register() API
//   - flags.go   — global flag catalog
//   - app.go     — Main → bootstrap → dispatch → execute
//   - help.go    — localized root / command help
//
// Preferred registration (fluent builder):
//
//	func init() {
//	    command.Define("hello").
//	        Summary("cmd.hello.summary").
//	        Group("group.example").
//	        Arg(command.Arg{
//	            Name: "text", Type: command.TypeString, Required: true,
//	            Desc: "arg.hello.text",
//	        }).
//	        Examples("hello --text world", "hello --text world --json").
//	        Rules(validate.Required("text"), validate.LengthBetween("text", 1, 64)).
//	        Handle(Run).
//	        Register()
//	}
//
// Handler signature (Strategy):
//
//	func Run(ctx *middleware.Context, in input.Input) (any, error)
//
// Return a value for Respond middleware to format; return error for exit 1/2.
package command

import (
	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

// Process exit codes returned by Main / Run.
const (
	ExitOK    = 0 // success
	ExitError = 1 // runtime / business failure
	ExitUsage = 2 // argument or usage error
)

// Error kind labels embedded in JSON failure payloads (--json).
const (
	ErrTypeUsage   = "usage"
	ErrTypeRuntime = "runtime"
)

// Re-export input argument types so biz packages can depend on command only.
const (
	TypeString  = input.TypeString
	TypeInt     = input.TypeInt
	TypeBool    = input.TypeBool
	TypeStrings = input.TypeStrings
)

// Convenience aliases so biz code stays import-light.
type (
	Arg        = input.Arg
	ArgType    = input.ArgType
	Input      = input.Input
	UsageError = input.UsageError
	Context    = middleware.Context
	Handler    = middleware.Handler
	Middleware = middleware.Middleware
)

// AppName is the display name used by help / version text.
var AppName = consts.AppName

// Command is a registered subcommand definition (Strategy target).
//
// Prefer building via Define(...).Register(). Fields Summary and Group are
// i18n keys, not raw display strings. Run is required.
type Command struct {
	Name     string
	Summary  string // i18n key for one-line help text
	Group    string // i18n key for help section grouping
	Args     []Arg
	Example  []string     // argv snippets shown under "Examples"
	Use      []Middleware // command-local middleware (after global chain)
	Validate func(ctx *Context, in Input) error
	Run      Handler
}

// registry holds commands in registration order (used by help).
var registry []*Command

func register(cmd *Command) {
	if cmd == nil || cmd.Name == "" {
		panic("command: name must not be empty")
	}
	if lookup(cmd.Name) != nil {
		panic("command: duplicate " + cmd.Name)
	}
	if cmd.Run == nil {
		panic("command: " + cmd.Name + " missing Handle")
	}
	registry = append(registry, cmd)
}

// Register appends pre-built commands (advanced / tests).
// Prefer Define(...).Register() in business packages.
func Register(cmds ...*Command) {
	for _, cmd := range cmds {
		register(cmd)
	}
}

func lookup(name string) *Command {
	for _, cmd := range registry {
		if cmd.Name == name {
			return cmd
		}
	}
	return nil
}

// Commands returns the registration order used by help output.
func Commands() []*Command { return registry }

// T is a thin alias for i18n.T used by help and error helpers.
func T(key string, args ...any) string { return i18n.T(key, args...) }
