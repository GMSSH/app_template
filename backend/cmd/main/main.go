package main

import (
	"os"

	// Blank-import business commands
	_ "gmssh-cli-plugin/internal/biz/domain"
	_ "gmssh-cli-plugin/internal/biz/ping"
	_ "gmssh-cli-plugin/internal/biz/version"

	"gmssh-cli-plugin/internal/command"
)

func main() {
	os.Exit(command.Main(os.Args[1:]))
}
