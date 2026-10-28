package demo

import (
	"strings"

	"gmssh-cli-plugin/internal/command"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
)

// UpperResult is the structured payload for demo-upper.
type UpperResult struct {
	Text string `json:"text"`
}

// reservedUsage is a sentinel that triggers a usage error (exit code 2).
const reservedUsage = "fail-usage"

// Upper demonstrates Rules + Timing middleware + UsageFail (exit 2).
func Upper(ctx *middleware.Context, in input.Input) (any, error) {
	raw := strings.TrimSpace(in.Str("text"))
	if strings.EqualFold(raw, reservedUsage) {
		logging.Business().Warnf("demo-upper rejected reserved text")
		// UsageFail → engine prints help-ish usage path, exit 2.
		return nil, command.UsageFail("cli.invalid_enum", "text", raw, "any text except fail-usage")
	}
	out := strings.ToUpper(raw)
	logging.Business().Infof("demo-upper ok lang=%s", ctxLang(ctx))
	return UpperResult{Text: out}, nil
}

func ctxLang(ctx *middleware.Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.Lang
}
