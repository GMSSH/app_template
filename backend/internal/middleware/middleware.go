// Package middleware defines Context and the execution chain (Chain of Responsibility).
//
// Default global order registered in init():
//
//	Locale → RequestLog → ResponseLog → Respond
//
// Respond formats successful handler results onto ctx.Stdout via the output package.
// Handlers must return data (or error); they must not fmt.Print themselves.
//
// Framework / process logs → logging.Runtime()
// Domain / command logs    → logging.Business()
//
// Handler example:
//
//	func Run(ctx *middleware.Context, in input.Input) (any, error) {
//	    logging.Business().Infof("work start id=%s", in.Str("id"))
//	    return map[string]any{"ok": true}, nil
//	}
//
// Custom middleware example:
//
//	func Audit(next middleware.Handler) middleware.Handler {
//	    return func(ctx *middleware.Context, in input.Input) (any, error) {
//	        // before
//	        out, err := next(ctx, in)
//	        // after
//	        return out, err
//	    }
//	}
//
// Register globally (once, e.g. in an init):
//
//	middleware.Use(Audit)
//
// Or per-command:
//
//	command.Use(Audit)
package middleware

import (
	"encoding/json"
	"fmt"
	"io"

	"gmssh-cli-plugin/internal/config"
	"gmssh-cli-plugin/internal/i18n"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/output"
	"gmssh-cli-plugin/internal/store"
)

// Format selects the response rendering strategy used by Respond.
type Format int

const (
	FormatJSON Format = iota // JSON envelope (default)
	FormatText               // human-readable lines (--format text)
)

// Context is the per-invocation bag passed to handlers and middleware.
// The engine builds one Context in command.execute before running the chain.
type Context struct {
	Name     string      // registered command name
	Lang     string      // resolved language (en / zh-CN / ...)
	Stdout   io.Writer   // success output (already charset-wrapped)
	Stderr   io.Writer   // diagnostics / usage errors (already charset-wrapped)
	Config   config.File // loaded profile + env overlays
	Store    store.Store // persistence rooted at data/
	Format   Format      // text or JSON
	Compact  bool        // compact JSON when FormatJSON (default true)
	Encoding string      // resolved charset name (utf-8, gbk, ...)
	// SkipRespond disables Respond (unit tests / nested calls).
	SkipRespond bool
}

// Handler is the Strategy signature for a command's Run function.
// Return a value for Respond to print; return error for exit mapping.
type Handler func(ctx *Context, in input.Input) (any, error)

// Middleware wraps a Handler (onion / Chain of Responsibility).
type Middleware func(Handler) Handler

// globalMW is the process-wide middleware list (outer → inner registration order).
var globalMW []Middleware

// Use appends global middleware. Prefer registering in package init.
func Use(mw ...Middleware) {
	globalMW = append(globalMW, mw...)
}

// Chain composes global middleware then command-specific extra around h.
// Composition is reverse-order so the first registered MW runs outermost.
func Chain(h Handler, extra []Middleware) Handler {
	all := make([]Middleware, 0, len(globalMW)+len(extra))
	all = append(all, globalMW...)
	all = append(all, extra...)
	for i := len(all) - 1; i >= 0; i-- {
		h = all[i](h)
	}
	return h
}

// Locale applies ctx.Lang to the process i18n state before calling next.
func Locale(next Handler) Handler {
	return func(ctx *Context, in input.Input) (any, error) {
		lang := ""
		if ctx != nil {
			lang = ctx.Lang
		}
		i18n.SetLanguage(lang)
		if ctx != nil {
			ctx.Lang = i18n.Language()
		}
		return next(ctx, in)
	}
}

// RequestLog writes a runtime debug line with command args (before handler).
func RequestLog(next Handler) Handler {
	return func(ctx *Context, in input.Input) (any, error) {
		logging.Runtime().Debugf("request command=%s lang=%s args=%s", ctxName(ctx), ctxLang(ctx), formatInput(in))
		return next(ctx, in)
	}
}

// ResponseLog writes a runtime debug line after the handler returns.
func ResponseLog(next Handler) Handler {
	return func(ctx *Context, in input.Input) (any, error) {
		out, err := next(ctx, in)
		if err != nil {
			logging.Runtime().Debugf("response command=%s err=%s", ctxName(ctx), err)
		} else {
			logging.Runtime().Debugf("response command=%s result=%s", ctxName(ctx), formatResult(out))
		}
		return out, err
	}
}

// Respond writes a successful result using the output package (response stage).
// Errors are NOT rendered here — the command engine's fail() formats them.
//
// JSON (default, compact): {"ok":true,"data":<result>}
// Text: human-readable rendering of <result> only.
// Charset conversion is applied by the engine on Stdout/Stderr writers.
func Respond(next Handler) Handler {
	return func(ctx *Context, in input.Input) (any, error) {
		out, err := next(ctx, in)
		if err != nil || ctx == nil || ctx.SkipRespond {
			return out, err
		}
		switch ctx.Format {
		case FormatText:
			if err := output.WriteText(ctx.Stdout, out); err != nil {
				return out, err
			}
		default:
			if err := output.WriteJSON(ctx.Stdout, ctx.Stderr, output.OK(out), ctx.Compact); err != nil {
				return out, err
			}
		}
		return out, nil
	}
}

func ctxName(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.Name
}

func ctxLang(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.Lang
}

func formatInput(in input.Input) string {
	b, err := json.Marshal(in.Values())
	if err != nil {
		return "{}"
	}
	return string(b)
}

func formatResult(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func init() {
	Use(Locale, RequestLog, ResponseLog, Respond)
}
