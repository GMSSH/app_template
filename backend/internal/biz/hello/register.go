package hello

import (
	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/validate"
)

// init self-registers with the command engine.
// The framework does not import this package — cmd/main blank-imports it.
func init() {
	command.Define("hello").
		Summary("cmd.hello.summary").
		Group("group.example").
		Arg(command.Arg{
			Name:     "text",
			Type:     command.TypeString,
			Required: true,
			Desc:     "arg.hello.text",
		}).
		Examples(
			"hello --text world",
			"hello --text world --json",
			"hello --text world --lang zh-CN",
		).
		Rules(
			validate.Required("text"),
			validate.LengthBetween("text", 1, 64),
		).
		Handle(Run).
		Register()
}
