// GMSSH plugin CLI entry point.
//
// The host runs app/bin/main once per user action, then the process exits.
// Success → stdout; failure → stderr. Exit codes: 0 ok, 1 runtime, 2 usage.
package main

import (
	"os"

	// Blank-import business commands so they self-register with the engine.
	_ "gmssh-cli-plugin/internal/biz/demo"
	_ "gmssh-cli-plugin/internal/biz/footprint"
	_ "gmssh-cli-plugin/internal/biz/hello"
	_ "gmssh-cli-plugin/internal/biz/ping"
	_ "gmssh-cli-plugin/internal/biz/version"

	"gmssh-cli-plugin/internal/command"
)

func main() {
	os.Exit(command.Main(os.Args[1:]))
}
