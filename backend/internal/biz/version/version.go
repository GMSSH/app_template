// Package version exposes build identity for the "version" command.
//
// Single responsibility: assemble version metadata. Number / Commit / Date are
// intended to be overridden via -ldflags at build time (see Makefile):
//
//	-X gmssh-cli-plugin/internal/biz/version.Number=1.2.3
//
//	main version
//	main version --json
package version

import (
	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

var (
	// Number is the semantic version string (ldflags override).
	Number = "dev"
	// Commit is the source revision when provided by the build.
	Commit = "unknown"
	// Date is the build timestamp when provided by the build.
	Date = "unknown"
)

// Info returns a stable map suitable for JSON / text output.
func Info(lang string) map[string]any {
	return map[string]any{
		"name":    consts.AppName,
		"version": Number,
		"commit":  Commit,
		"date":    Date,
		"lang":    lang,
	}
}

// Run implements the version command strategy.
func Run(ctx *middleware.Context, _ input.Input) (any, error) {
	lang := ""
	if ctx != nil {
		lang = ctx.Lang
	}
	return Info(lang), nil
}
