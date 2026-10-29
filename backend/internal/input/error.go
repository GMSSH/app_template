package input

import (
	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/util"
)

// UsageError represents a user-facing argument / usage failure (exit code 2).
//
// Translation is deferred to Error() so --lang can be applied before printing.
// The command engine detects *UsageError via errors.As and prints help.
type UsageError struct {
	Key  string
	Args []any
	Msg  string // optional pre-rendered message; wins over Key when set
	Site string // source frame where UsageFail was called (file:line)
}

func (e *UsageError) Error() string {
	if e == nil {
		return ""
	}
	if e.Msg != "" {
		return e.Msg
	}
	return i18n.T(e.Key, e.Args...)
}

// Frame returns the capture site for runtime log attribution.
func (e *UsageError) Frame() string {
	if e == nil {
		return ""
	}
	return e.Site
}

// UsageFail builds a UsageError from an i18n key and placeholder args.
// The caller's file:line is recorded for logging.
//
//	return nil, input.UsageFail("cli.missing_required", "text")
func UsageFail(key string, args ...any) error {
	return &UsageError{Key: key, Args: args, Site: util.CallerSite(1)}
}
