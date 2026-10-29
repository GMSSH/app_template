package ping

import "gmssh-cli-plugin/internal/command"

// init registers the ping command (blank-imported from cmd/main).
func init() {
	command.Define("ping").
		Summary("cmd.ping.summary").
		Group("group.basic").
		Examples("ping", "ping --json").
		Handle(Run).
		Register()
}
