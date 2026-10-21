package demo

import (
	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/validate"
)

// init registers demo commands with the fluent builder (same style as hello/ping).
func init() {
	command.Define("demo-echo").
		Summary("cmd.demo_echo.summary").
		Group("group.example").
		Arg(command.Arg{Name: "text", Type: command.TypeString, Required: true, Desc: "arg.demo.text"}).
		Examples("demo-echo --text hello", "demo-echo --text hello --json").
		Rules(validate.Required("text")).
		Handle(Echo).
		Register()

	command.Define("demo-upper").
		Summary("cmd.demo_upper.summary").
		Group("group.example").
		Arg(command.Arg{Name: "text", Type: command.TypeString, Required: true, Desc: "arg.demo.text"}).
		Examples("demo-upper --text hello", "demo-upper --text fail-usage").
		Rules(validate.Required("text"), validate.LengthBetween("text", 1, 64)).
		Use(Timing).
		Handle(Upper).
		Register()

	command.Define("demo-div").
		Summary("cmd.demo_div.summary").
		Group("group.example").
		Args(
			command.Arg{Name: "a", Type: command.TypeInt, Required: true, Desc: "arg.demo.a"},
			command.Arg{Name: "b", Type: command.TypeInt, Required: true, Desc: "arg.demo.b"},
		).
		Examples("demo-div --a 10 --b 2", "demo-div --a 10 --b 0").
		Rules(validate.Required("a"), validate.Required("b")).
		Use(Timing).
		Handle(Div).
		Register()
}
