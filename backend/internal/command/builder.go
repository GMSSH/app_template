package command

import "gmssh-cli-plugin/internal/validate"

// Builder is a fluent command constructor (preferred registration API).
//
// Example:
//
//	command.Define("hello").
//	    Summary("cmd.hello.summary").
//	    Group("group.example").
//	    Arg(command.Arg{
//	        Name: "text", Type: command.TypeString, Required: true,
//	        Desc: "arg.hello.text",
//	    }).
//	    Examples("hello --text world", "hello --text world --json").
//	    Rules(validate.Required("text"), validate.LengthBetween("text", 1, 64)).
//	    Handle(Run).
//	    Register()
type Builder struct {
	cmd Command
}

// Define starts a fluent builder for a named command.
func Define(name string) *Builder {
	return &Builder{cmd: Command{Name: name}}
}

// Summary sets the i18n key for the command one-liner in help.
func (b *Builder) Summary(key string) *Builder {
	b.cmd.Summary = key
	return b
}

// Group sets the i18n key for the help section (e.g. "group.example").
func (b *Builder) Group(key string) *Builder {
	b.cmd.Group = key
	return b
}

// Arg appends one argument schema entry.
func (b *Builder) Arg(arg Arg) *Builder {
	b.cmd.Args = append(b.cmd.Args, arg)
	return b
}

// Args replaces the argument schema (use Arg for incremental adds).
func (b *Builder) Args(args ...Arg) *Builder {
	b.cmd.Args = append([]Arg(nil), args...)
	return b
}

// Examples lists argv snippets printed under the help "Examples" section.
func (b *Builder) Examples(ex ...string) *Builder {
	b.cmd.Example = append([]string(nil), ex...)
	return b
}

// Use appends command-local middleware (after the global chain).
func (b *Builder) Use(mw ...Middleware) *Builder {
	b.cmd.Use = append(b.cmd.Use, mw...)
	return b
}

// Validate attaches a custom validator that receives Context + Input.
// Prefer Rules for composable validate.Func chains.
func (b *Builder) Validate(fn func(*Context, Input) error) *Builder {
	b.cmd.Validate = fn
	return b
}

// Rules attaches composable validate.Func checks (extensible validation pipeline).
func (b *Builder) Rules(rules ...validate.Func) *Builder {
	return b.Validate(func(_ *Context, in Input) error {
		return validate.RunAll(in, rules...)
	})
}

// Handle wires the business Strategy. Pass a named package function (e.g. Run).
func (b *Builder) Handle(run Handler) *Builder {
	b.cmd.Run = run
	return b
}

// Cmd returns a copy of the built Command without registering it (tests).
func (b *Builder) Cmd() *Command {
	c := b.cmd
	return &c
}

// Register commits the command into the global registry (call from init).
func (b *Builder) Register() {
	c := b.cmd
	register(&c)
}
