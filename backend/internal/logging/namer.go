package logging

import (
	"regexp"
	"strings"
	"time"
)

const defaultLayout = "2006-01-02.log"

// FileNamer builds daily log filenames from a Go time layout and matches old
// files with a regexp (for retention pruning).
//
//	n, _ := logging.NewFileNamer("runtime-2006-01-02.log", "")
//	name := n.Name(time.Now()) // e.g. runtime-2026-10-12.log
type FileNamer struct {
	layout string
	re     *regexp.Regexp
}

// NewFileNamer compiles layout/pattern. An empty pattern is derived from layout.
// If the provided pattern does not match a sample layout value, a derived
// pattern is substituted so pruning still works after layout changes.
func NewFileNamer(layout, pattern string) (*FileNamer, error) {
	layout = strings.TrimSpace(layout)
	if layout == "" {
		layout = defaultLayout
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = layoutToPattern(layout)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	sample := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC).Format(layout)
	if !re.MatchString(sample) {
		re = regexp.MustCompile(layoutToPattern(layout))
	}
	return &FileNamer{layout: layout, re: re}, nil
}

func (n *FileNamer) Layout() string {
	if n == nil {
		return defaultLayout
	}
	return n.layout
}

func (n *FileNamer) Name(t time.Time) string {
	if n == nil {
		return t.Format(defaultLayout)
	}
	return t.Format(n.layout)
}

func (n *FileNamer) Match(name string) bool {
	return n != nil && n.re != nil && n.re.MatchString(name)
}

func (n *FileNamer) Day(name string, loc *time.Location) (time.Time, bool) {
	if n == nil {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}
	day, err := time.ParseInLocation(n.layout, name, loc)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc), true
}

func layoutToPattern(layout string) string {
	quoted := regexp.QuoteMeta(layout)
	repls := []struct{ from, to string }{
		{"2006", "\\d{4}"},
		{"15", "\\d{2}"},
		{"04", "\\d{2}"},
		{"05", "\\d{2}"},
		{"01", "\\d{2}"},
		{"02", "\\d{2}"},
		{"03", "\\d{2}"},
		{"06", "\\d{2}"},
		{"07", "\\d{2}"},
	}
	for _, token := range repls {
		quoted = strings.ReplaceAll(quoted, token.from, token.to)
	}
	return "^" + quoted + "$"
}
