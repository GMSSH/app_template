package toolbox

import (
	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/validate"
)

// init 注册站长百宝箱的唯一业务命令。
// 所有工具都是同一个命令的不同 action：宿主每次执行一条，打印 JSON 后退出。
func init() {
	command.Define("toolbox").
		Summary("cmd.toolbox.summary").
		Group("group.toolbox").
		Args(
			command.Arg{
				Name: "action", Type: command.TypeString, Default: "probe",
				Desc: "arg.toolbox.action", Required: true,
				Enum: []string{"probe", "dns", "ping", "port", "http", "ssl", "whois"},
			},
			command.Arg{Name: "target", Type: command.TypeString, Desc: "arg.toolbox.target"},
			command.Arg{Name: "type", Type: command.TypeString, Default: "A", Desc: "arg.toolbox.type",
				Enum: []string{
					"ALL", "A", "AAAA", "CNAME", "MX", "TXT", "NS",
					"all", "a", "aaaa", "cname", "mx", "txt", "ns",
				}},
			command.Arg{Name: "server", Type: command.TypeString, Desc: "arg.toolbox.server"},
			command.Arg{Name: "port", Type: command.TypeString, Desc: "arg.toolbox.port"},
			command.Arg{Name: "count", Type: command.TypeInt, Default: "5", Desc: "arg.toolbox.count"},
			command.Arg{Name: "timeout", Type: command.TypeInt, Default: "10", Desc: "arg.toolbox.timeout"},
		).
		Examples(
			"toolbox --action probe --json",
			"toolbox --action dns --target example.com --type ALL --json",
			"toolbox --action ping --target example.com --count 5 --json",
			"toolbox --action port --target example.com --json",
			"toolbox --action http --target example.com --json",
			"toolbox --action ssl --target example.com --port 443 --json",
			"toolbox --action whois --target example.com --json",
		).
		Rules(validate.OneOf("action", "probe", "dns", "ping", "port", "http", "ssl", "whois")).
		Handle(Run).
		Register()
}
