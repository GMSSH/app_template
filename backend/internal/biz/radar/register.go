package radar

import (
	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/validate"
)

func init() {
	command.Define("radar").
		Summary("cmd.radar.summary").
		Group("group.security").
		Args(
			command.Arg{
				Name:    "action",
				Type:    command.TypeString,
				Desc:    "arg.radar.action",
				Default: "server",
				Enum:    []string{"server", "global", "heat", "paths"},
			},
			command.Arg{
				Name:    "metric",
				Type:    command.TypeString,
				Desc:    "arg.radar.metric",
				Default: "target",
				Enum:    []string{"target", "origin"},
			},
			command.Arg{
				Name:    "range",
				Type:    command.TypeString,
				Desc:    "arg.radar.range",
				Default: "3d",
			},
		).
		Examples(
			"radar --action server --json",
			"radar --action global --json",
			"radar --action heat --metric target --range 3d --json",
			"radar --action paths --range 3d --json",
		).
		Rules(
			validate.OneOf("action", "server", "global", "heat", "paths"),
		).
		Handle(Run).
		Register()
}
