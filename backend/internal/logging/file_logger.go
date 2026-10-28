package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	defaultMaxDays    = 7
	maxMessageRunes   = 8192
	defaultRuntimeLay = "runtime-2006-01-02.log"
)

// fileLogger is one Kind's concrete Logger (daily file + optional stderr mirror).
// Write failures set broken=true so subsequent calls skip disk I/O silently.
type fileLogger struct {
	kind       Kind
	mu         sync.Mutex
	level      *atomic.Int32
	broken     atomic.Bool
	consoleOn  *atomic.Bool
	consoleOut *io.Writer
	logDir     *string
	retainDays *int
	namer      *FileNamer
	handle     *os.File
	filePath   string
}

func (l *fileLogger) Kind() Kind { return l.kind }

func (l *fileLogger) Enabled(want Level) bool {
	current := Level(l.level.Load())
	if want < Error || want > current {
		return false
	}
	if l.consoleOn.Load() {
		return true
	}
	return !l.broken.Load()
}

func (l *fileLogger) Debugf(format string, args ...any) { l.log(Debug, format, args...) }
func (l *fileLogger) Infof(format string, args ...any)  { l.log(Info, format, args...) }
func (l *fileLogger) Warnf(format string, args ...any)  { l.log(Warn, format, args...) }
func (l *fileLogger) Errorf(format string, args ...any) { l.log(Error, format, args...) }

func (l *fileLogger) log(level Level, format string, args ...any) {
	if !l.Enabled(level) {
		return
	}
	// Capture the first frame outside this package (Python-style file:line).
	l.write(level, callSite(), fmt.Sprintf(format, args...))
}

func (l *fileLogger) write(level Level, site, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Example: 2026-10-12 16:25:00.000 [business] [ERROR] internal/biz/demo/div.go:28 demo-div ...
	if site == "" {
		site = "?"
	}
	line := fmt.Sprintf("%s [%s] [%s] %s %s\n",
		time.Now().Format("2006-01-02 15:04:05.000"),
		l.kind,
		level.String(),
		site,
		limitMessage(sanitize(msg)),
	)
	if l.consoleOn.Load() && l.consoleOut != nil && *l.consoleOut != nil {
		_, _ = io.WriteString(*l.consoleOut, line)
	}
	if l.broken.Load() {
		return
	}
	if err := l.ensureOpen(); err != nil {
		l.broken.Store(true)
		return
	}
	if _, err := l.handle.WriteString(line); err != nil {
		l.broken.Store(true)
	}
}

func (l *fileLogger) ensureOpen() error {
	want := filepath.Join(*l.logDir, l.namer.Name(time.Now()))
	if l.handle != nil && l.filePath == want {
		return nil
	}
	if l.handle != nil {
		_ = l.handle.Close()
		l.handle = nil
	}
	if strings.TrimSpace(*l.logDir) == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(*l.logDir, 0o700); err != nil {
		return err
	}
	l.prune()
	file, err := os.OpenFile(want, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	l.handle = file
	l.filePath = want
	return nil
}

func (l *fileLogger) prune() {
	if strings.TrimSpace(*l.logDir) == "" || *l.retainDays <= 0 || l.namer == nil {
		return
	}
	entries, err := os.ReadDir(*l.logDir)
	if err != nil {
		return
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, -(*l.retainDays - 1))
	for _, entry := range entries {
		name := entry.Name()
		if !l.namer.Match(name) {
			continue
		}
		day, ok := l.namer.Day(name, now.Location())
		if !ok {
			continue
		}
		if day.Before(cutoff) {
			_ = os.Remove(filepath.Join(*l.logDir, name))
		}
	}
}

func (l *fileLogger) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.handle != nil {
		_ = l.handle.Close()
		l.handle = nil
	}
	l.filePath = ""
	l.broken.Store(false)
}

func sanitize(msg string) string {
	if !strings.ContainsAny(msg, "\r\n") {
		return msg
	}
	msg = strings.ReplaceAll(msg, "\r", "\\r")
	return strings.ReplaceAll(msg, "\n", "\\n")
}

func limitMessage(msg string) string {
	total := utf8.RuneCountInString(msg)
	if total <= maxMessageRunes {
		return msg
	}
	count := 0
	for idx := range msg {
		if count == maxMessageRunes {
			return msg[:idx] + fmt.Sprintf("(truncated, total %d runes)", total)
		}
		count++
	}
	return msg
}

func normalizeLevel(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// callSite returns "internal/.../file.go:line" for the first caller outside
// package logging (similar to Python logging's %(pathname)s:%(lineno)d).
func callSite() string {
	for skip := 2; skip < 16; skip++ {
		_, file, line, ok := runtime.Caller(skip)
		if !ok || file == "" {
			break
		}
		if strings.Contains(file, string(filepath.Separator)+"logging"+string(filepath.Separator)) {
			continue
		}
		if i := strings.LastIndex(file, string(filepath.Separator)+"internal"+string(filepath.Separator)); i >= 0 {
			file = file[i+1:]
		} else {
			file = filepath.Base(file)
		}
		return fmt.Sprintf("%s:%d", file, line)
	}
	return ""
}
