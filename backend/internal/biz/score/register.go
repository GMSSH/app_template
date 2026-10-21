package score

import (
	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/validate"
)

func init() {
	command.Define("score").
		Summary("cmd.score.summary").
		Group("group.basic").
		Examples("score", "score --json").
		Handle(RunScore).
		Register()

	command.Define("probe").
		Summary("cmd.probe.summary").
		Group("group.basic").
		Arg(command.Arg{Name: "key", Type: command.TypeString, Required: true, Desc: "arg.probe.key"}).
		Arg(command.Arg{Name: "dir", Type: command.TypeString, Required: false, Desc: "arg.probe.dir"}).
		Arg(command.Arg{Name: "force", Type: command.TypeBool, Required: false, Desc: "arg.probe.force"}).
		Examples("probe --key cpu", "probe --key mem", "probe --key disk", "probe --key disk --dir /data", "probe --key cpu --force").
		Rules(validate.Required("key"), validate.OneOf("key", "cpu", "mem", "disk")).
		Handle(RunProbe).
		Register()

	command.Define("calibrate").
		Summary("cmd.calibrate.summary").
		Group("group.basic").
		Arg(command.Arg{Name: "n", Type: command.TypeInt, Required: false, Desc: "arg.calibrate.n"}).
		Arg(command.Arg{Name: "dir", Type: command.TypeString, Required: false, Desc: "arg.probe.dir"}).
		Examples("calibrate --n 5", "calibrate --n 20 --dir /data").
		Handle(Calibrate).
		Register()

	command.Define("check").
		Summary("cmd.check.summary").
		Group("group.basic").
		Examples("check", "check --json").
		Handle(RunCheck).
		Register()

	command.Define("history").
		Summary("cmd.history.summary").
		Group("group.basic").
		Examples("history", "history --json").
		Handle(RunHistory).
		Register()

	command.Define("overview").
		Summary("cmd.overview.summary").
		Group("group.basic").
		Examples("overview", "overview --json").
		Handle(RunOverview).
		Register()
}
