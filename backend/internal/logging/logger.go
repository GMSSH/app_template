// Package logging provides dual-channel logging for the CLI scaffold.
//
// Two kinds are exposed via a small Logger interface (Strategy / polymorphism):
//
//	logging.Runtime()  — framework / process logs (bootstrap, panic, exit)
//	logging.Business() — command / domain logs written by user handlers
//
// Both channels share level / dir / retention / console settings but write to
// separate daily files so operators can filter noise. Logging I/O failures are
// swallowed and never change process exit codes.
//
// Business handler usage:
//
//	logging.Business().Debugf("start id=%s", id)
//	logging.Business().Infof("done count=%d", n)
//	logging.Business().Warnf("retry n=%d", n)
//	logging.Business().Errorf("failed: %v", err)
//
// Framework code may use the package helpers (Runtime channel):
//
//	logging.Errorf("exit %d: %s", code, message)
//
// Configuration is applied once at bootstrap via Configure after Resolve:
//
//	settings, err := logging.Resolve(baseFromConfig, logging.Overlay{Debug: true})
//	logging.Configure(settings)
package logging

import (
	"io"
)

// Level is the severity threshold. Messages at or below Level are emitted.
type Level int32

const (
	Off Level = iota
	Error
	Warn
	Info
	Debug
)

// String returns the uppercase level token used in log lines.
func (l Level) String() string {
	switch l {
	case Off:
		return "OFF"
	case Error:
		return "ERROR"
	case Warn:
		return "WARN"
	case Info:
		return "INFO"
	case Debug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel maps a config / env / flag token to Level.
// Accepted: off, error, warn, info, debug (case-insensitive).
func ParseLevel(name string) (Level, bool) {
	switch normalizeLevel(name) {
	case "off":
		return Off, true
	case "error":
		return Error, true
	case "warn":
		return Warn, true
	case "info":
		return Info, true
	case "debug":
		return Debug, true
	default:
		return Off, false
	}
}

// Logger is the polymorphic logging API used by framework and business code.
type Logger interface {
	Kind() Kind
	Enabled(Level) bool
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Kind identifies a log channel.
type Kind string

const (
	KindRuntime  Kind = "runtime"
	KindBusiness Kind = "business"
)

// Settings configures both channels at process start.
// RuntimeFile / BusinessFile are Go time layouts for daily file names
// (e.g. "runtime-2006-01-02.log").
type Settings struct {
	Level        Level
	Dir          string
	RuntimeFile  string
	BusinessFile string
	Pattern      string    // optional regexp for retention pruning
	MaxDays      int       // retention window; <=0 → default
	Console      bool      // mirror lines to ConsoleOut
	ConsoleOut   io.Writer // usually stderr; set by the command engine
}

// Nop returns a discarded logger (tests / logging Off).
func Nop() Logger { return nopLogger{} }

type nopLogger struct{}

func (nopLogger) Kind() Kind            { return "" }
func (nopLogger) Enabled(Level) bool    { return false }
func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Infof(string, ...any)  {}
func (nopLogger) Warnf(string, ...any)  {}
func (nopLogger) Errorf(string, ...any) {}
