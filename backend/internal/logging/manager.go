package logging

import (
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"gmssh-cli-plugin/internal/consts"
)

// Manager owns the process-wide Runtime and Business loggers (singleton).
// External code should use Runtime() / Business() / Configure, not Manager fields.
type Manager struct {
	mu         sync.Mutex
	level      atomic.Int32
	consoleOn  atomic.Bool
	consoleOut io.Writer
	logDir     string
	retainDays int
	runtime    *fileLogger
	business   *fileLogger
}

var global = &Manager{
	consoleOut: os.Stderr,
	retainDays: defaultMaxDays,
}

func init() {
	global.level.Store(int32(Off))
	global.runtime = newChannel(KindRuntime, global, consts.DefaultRuntimeLogFile)
	global.business = newChannel(KindBusiness, global, consts.DefaultBusinessLogFile)
}

func newChannel(kind Kind, m *Manager, layout string) *fileLogger {
	namer, _ := NewFileNamer(layout, "")
	return &fileLogger{
		kind:       kind,
		level:      &m.level,
		consoleOn:  &m.consoleOn,
		consoleOut: &m.consoleOut,
		logDir:     &m.logDir,
		retainDays: &m.retainDays,
		namer:      namer,
	}
}

// Runtime returns the framework / process logger.
//
//	logging.Runtime().Debugf("bootstrap complete")
func Runtime() Logger { return global.runtime }

// Business returns the domain / command logger.
// Prefer this inside handlers under internal/biz.
//
//	logging.Business().Infof("order created id=%s", id)
func Business() Logger { return global.business }

// For returns a logger by kind (extensible for future channels).
//
//	log := logging.For(logging.KindBusiness)
func For(kind Kind) Logger {
	switch kind {
	case KindBusiness:
		return Business()
	default:
		return Runtime()
	}
}

// Package-level helpers write to the Runtime channel (framework default).
func Debugf(format string, args ...any) { Runtime().Debugf(format, args...) }
func Infof(format string, args ...any)  { Runtime().Infof(format, args...) }
func Warnf(format string, args ...any)  { Runtime().Warnf(format, args...) }
func Errorf(format string, args ...any) { Runtime().Errorf(format, args...) }

// Configure applies shared settings and per-kind file layouts.
// Safe to call multiple times (closes previous file handles first).
// When Dir is empty or Level is Off / invalid, logging is disabled.
func Configure(s Settings) {
	global.mu.Lock()
	defer global.mu.Unlock()

	global.runtime.close()
	global.business.close()
	applySharedSettings(s)
	applyChannelNamers(s)

	if global.logDir == "" || s.Level < Error || s.Level > Debug {
		global.level.Store(int32(Off))
		return
	}
	global.level.Store(int32(s.Level))
	_ = os.MkdirAll(global.logDir, 0o700)
	global.runtime.prune()
	global.business.prune()
}

// applySharedSettings updates retention, directory, and console mirroring.
func applySharedSettings(s Settings) {
	maxDays := s.MaxDays
	if maxDays <= 0 {
		maxDays = defaultMaxDays
	}
	global.retainDays = maxDays
	global.logDir = strings.TrimSpace(s.Dir)
	global.consoleOn.Store(s.Console)
	if s.ConsoleOut != nil {
		global.consoleOut = s.ConsoleOut
	} else {
		global.consoleOut = os.Stderr
	}
}

// applyChannelNamers installs daily filename layouts for each channel.
func applyChannelNamers(s Settings) {
	runtimeFile := strings.TrimSpace(s.RuntimeFile)
	if runtimeFile == "" {
		runtimeFile = consts.DefaultRuntimeLogFile
	}
	businessFile := strings.TrimSpace(s.BusinessFile)
	if businessFile == "" {
		businessFile = consts.DefaultBusinessLogFile
	}
	rn, err := NewFileNamer(runtimeFile, s.Pattern)
	if err != nil {
		rn, _ = NewFileNamer(defaultRuntimeLay, "")
	}
	bn, err := NewFileNamer(businessFile, s.Pattern)
	if err != nil {
		bn, _ = NewFileNamer(consts.DefaultBusinessLogFile, "")
	}
	global.runtime.namer = rn
	global.business.namer = bn
}
