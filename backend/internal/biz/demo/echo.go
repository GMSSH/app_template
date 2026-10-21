package demo

import (
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
)

// EchoResult is formatted by Respond middleware (text or --json).
// Handlers must return data — never fmt.Print.
type EchoResult struct {
	Text   string `json:"text"`
	Format string `json:"format"` // mirrors ctx.Format so learners can see it
}

// Echo demonstrates response formatting: return a struct, Respond renders it.
func Echo(ctx *middleware.Context, in input.Input) (any, error) {
	text := in.Str("text")
	logging.Business().Debugf("demo-echo text_len=%d", len(text))
	return EchoResult{Text: text, Format: formatName(ctx)}, nil
}

func formatName(ctx *middleware.Context) string {
	if ctx == nil {
		return "text"
	}
	if ctx.Format == middleware.FormatJSON {
		if ctx.Compact {
			return "json-compact"
		}
		return "json"
	}
	return "text"
}
