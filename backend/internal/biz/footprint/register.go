package footprint

import (
	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/validate"
)

// init 注册 ssh_footprint 业务命令
func init() {
	command.Define("ssh_footprint").
		Summary("SSH security footprint log analysis").
		Group("group.security").
		Args(
			command.Arg{
				Name:    "action",
				Type:    command.TypeString,
				Desc:    "Action to perform: probe (check log sources & permissions) or scan (analyze logs)",
				Default: "probe",
				Enum:    []string{"probe", "scan"},
			},
			command.Arg{
				Name:    "range",
				Type:    command.TypeString,
				Desc:    "Time range for scan: 24h, 7d, 30d",
				Default: "7d",
				Enum:    []string{"24h", "7d", "30d"},
			},
		).
		Examples(
			"ssh_footprint --action probe --json",
			"ssh_footprint --action scan --range 24h --json",
			"ssh_footprint --action scan --range 7d --json",
			"ssh_footprint --action scan --range 30d --json",
		).
		Rules(
			validate.OneOf("action", "probe", "scan"),
			validate.OneOf("range", "24h", "7d", "30d"),
		).
		Handle(Run).
		Register()
}
