// Package hello is the sample business command for the scaffold.
//
// Copy this package when adding a new command:
//  1. Create internal/biz/<name>/ with register.go + <name>.go
//  2. Blank-import the package from cmd/main/main.go
//  3. Register via command.Define(...).Handle(Run).Register() in init()
//
// It demonstrates: registration, argument schema, validate.Rules, middleware
// chain (automatic), structured Result, i18n messages, and business logging.
//
// CLI examples:
//
//	main hello --text world
//	main hello --text world --json
//	main hello --text world --lang zh-CN
package hello

import (
	"strings"

	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
)

// Result is the structured value returned to the output layer.
// JSON tags control both --json output and text field names.
type Result struct {
	Text string `json:"text"`
}

// Run is the command Strategy hooked by Handle(Run).
// Do not print here — return data and let Respond middleware format it.
func Run(ctx *middleware.Context, in input.Input) (any, error) {
	logging.Business().Debugf("hello start text_len=%d", len(in.Str("text")))
	out, err := Greet(in.Str("text"))
	if err != nil {
		logging.Business().Errorf("hello failed: %v", err)
		return nil, err
	}
	if ctx != nil {
		logging.Business().Infof("hello ok lang=%s", ctx.Lang)
	}
	return out, nil
}

// Greet builds the localized greeting; kept separate for unit testing.
func Greet(text string) (Result, error) {
	name := strings.TrimSpace(text)
	if name == "" {
		return Result{}, input.UsageFail("cli.missing_required", "text")
	}
	return Result{Text: i18n.T("cmd.hello.message", name)}, nil
}
