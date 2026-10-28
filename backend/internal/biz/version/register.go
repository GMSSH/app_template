package version

import "gmssh-cli-plugin/internal/command"

// init registers the version command (blank-imported from cmd/main).
func init() {
	command.Define("version").
		Summary("cmd.version.summary").
		Group("group.basic").
		Examples("version", "version --json").
		Handle(Run).
		Register()
}
