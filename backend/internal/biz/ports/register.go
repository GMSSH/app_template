package ports

import (
	"gmssh-cli-plugin/internal/command"
)

func init() {
	// ports 命令：支持 --action (scan / release / kill) 或直接无参默认 scan
	command.Define("ports").
		Summary("cmd.ports.summary").
		Group("group.network").
		Args(
			command.Arg{
				Name: "action",
				Type: command.TypeString,
				Desc: "arg.ports.action",
				Default: "scan",
			},
			command.Arg{
				Name: "port",
				Type: command.TypeInt,
				Desc: "arg.ports.port",
			},
			command.Arg{
				Name: "proto",
				Type: command.TypeString,
				Desc: "arg.ports.proto",
			},
			command.Arg{
				Name: "pid",
				Type: command.TypeInt,
				Desc: "arg.ports.pid",
			},
		).
		Examples(
			"ports",
			"ports --json",
			"ports --action scan --json",
			"ports --action kill --pid 1234 --json",
			"ports --action release --port 8080 --proto tcp --json",
		).
		Handle(Run).
		Register()
}
