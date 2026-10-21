package command

import (
	"errors"
	"fmt"

	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/util"
)

// RuntimeError is a business/runtime failure (exit code 1).
//
// User-facing text is resolved via i18n key at Error() time (after --lang),
// so product teams customize copy in i18n/*.ini without changing Go code.
// Optional Cause holds the underlying technical error for logs only.
type RuntimeError struct {
	Key   string // i18n key, e.g. "cmd.demo_div.div_zero"
	Args  []any  // placeholders for i18n.T
	Cause error  // underlying error; logged, not shown to end users
	Site  string // source frame (file:line)
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Key != "" {
		return i18n.T(e.Key, e.Args...)
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "runtime error"
}

// Frame returns the capture site for runtime log attribution.
func (e *RuntimeError) Frame() string {
	if e == nil {
		return ""
	}
	return e.Site
}

// Unwrap exposes Cause for errors.Is / errors.As.
func (e *RuntimeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Fail builds a runtime error from an i18n key (exit code 1).
// Customize the message by editing i18n/en.ini and i18n/zh-CN.ini.
//
//	return nil, command.Fail("cmd.demo_div.div_zero", a)
func Fail(key string, args ...any) error {
	return &RuntimeError{Key: key, Args: args, Site: util.CallerSite(1)}
}

// FailCause wraps an underlying technical error with a user-facing i18n key.
// JSON/stderr show only the localized message; logs keep the cause.
//
//	if err := doWork(); err != nil {
//	    return nil, command.FailCause("cmd.demo_div.div_zero", err, a)
//	}
func FailCause(key string, cause error, args ...any) error {
	return &RuntimeError{Key: key, Args: args, Cause: cause, Site: util.CallerSite(1)}
}

// UsageFail builds a usage error (exit code 2) from an i18n key.
//
//	return nil, command.UsageFail("cli.missing_required", "text")
func UsageFail(key string, args ...any) error {
	return &input.UsageError{Key: key, Args: args, Site: util.CallerSite(1)}
}

// errorFrame extracts a captured source frame from err when available.
func errorFrame(err error) string {
	type framer interface{ Frame() string }
	var f framer
	if errors.As(err, &f) {
		return f.Frame()
	}
	return ""
}

// errorKey returns the i18n key when err is a RuntimeError or UsageError.
func errorKey(err error) string {
	var re *RuntimeError
	if errors.As(err, &re) && re != nil {
		return re.Key
	}
	var ue *input.UsageError
	if errors.As(err, &ue) && ue != nil {
		return ue.Key
	}
	return ""
}

// errorCause returns the wrapped technical cause when present.
func errorCause(err error) error {
	var re *RuntimeError
	if errors.As(err, &re) && re != nil {
		return re.Cause
	}
	return nil
}

// ensureUserError maps plain Go errors onto a localizable RuntimeError so
// clients never see raw library text unless the handler already wrapped it.
func ensureUserError(err error) error {
	if err == nil {
		return nil
	}
	var re *RuntimeError
	if errors.As(err, &re) {
		return err
	}
	var ue *input.UsageError
	if errors.As(err, &ue) {
		return err
	}
	return &RuntimeError{
		Key:   "cli.runtime_error",
		Cause: err,
		Site:  util.CallerSite(1),
		Args:  []any{fmt.Sprint(err)},
	}
}
