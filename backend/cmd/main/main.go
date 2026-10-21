// GMSSH plugin entry point — pure CLI, one-shot per invocation.
//
// The GMSSH host executes app/bin/main <command> [flags] for every frontend
// action (via /api/command/exec_shell). The process prints the result to
// stdout and exits immediately. There is no resident service, no socket,
// no $gm.init: that whole layer was a misunderstanding of the platform.
//
// Exit codes: 0 ok, 1 business failure, 2 usage error.
package main

import (
	"os"

	// Blank-import business commands so they self-register with the engine.
	_ "gmssh-cli-plugin/internal/biz/demo"
	_ "gmssh-cli-plugin/internal/biz/hello"
	_ "gmssh-cli-plugin/internal/biz/ping"
	_ "gmssh-cli-plugin/internal/biz/score"
	_ "gmssh-cli-plugin/internal/biz/version"

	"gmssh-cli-plugin/internal/command"
)

func main() {
	os.Exit(command.Main(os.Args[1:]))
}
