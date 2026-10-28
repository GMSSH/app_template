// Package i18n provides localized message lookup for the CLI.
//
// Single responsibility: load en/zh-CN message catalogs and format keys with
// positional "{}" placeholders. Language selection is owned by the command
// engine (flag --lang / GMCLI_LANG); this package only stores and formats.
//
// Layout constraints (do not move):
//   - this file: internal/i18n/i18n.go
//   - catalogs:  repo-root i18n/*.ini (embedded via i18n/embed.go)
//
// When GMCLI_I18N_DIR points at a directory that contains the same ini names,
// those files override the embedded catalogs (useful for hot-fixing copy).
// Missing keys return the key string unchanged. Default language is en.
//
// Usage:
//
//	i18n.SetLanguage("zh-CN")
//	msg := i18n.T("cmd.hello.message", "world") // 你好，world
package i18n

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	locales "gmssh-cli-plugin/i18n"
	"gmssh-cli-plugin/internal/consts"
	"gmssh-cli-plugin/internal/util"
)

const (
	DefaultLanguage = consts.DefaultLang
	EnvLang         = consts.EnvLang
	EnvI18nDir      = consts.EnvI18nDir
)

var (
	mu      sync.RWMutex
	current = DefaultLanguage
	loaded  = map[string]map[string]string{}
)

func embedded(lang string) []byte {
	name := "zh-CN.ini"
	if Normalize(lang) == "en" {
		name = "en.ini"
	}
	data, err := locales.Files.ReadFile(name)
	if err != nil {
		return nil
	}
	return data
}

// Normalize maps common aliases onto the supported catalog names.
func Normalize(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "en", "en-us", "en_us", "enus", "english":
		return "en"
	case "zh", "zh-cn", "zh_cn", "zhcn", "cn", "chinese", "zh-hans":
		return "zh-CN"
	default:
		return lang
	}
}

// Language returns the active catalog name.
func Language() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SetLanguage stores the normalized language and returns it.
func SetLanguage(lang string) string {
	normalized := Normalize(lang)
	mu.Lock()
	defer mu.Unlock()
	current = normalized
	return current
}

// Available lists catalog names shipped with the binary.
func Available() []string {
	return []string{"en", "zh-CN"}
}

// Supported reports whether lang normalizes to a shipped catalog.
func Supported(lang string) bool {
	normalized := Normalize(lang)
	for _, item := range Available() {
		if item == normalized {
			return true
		}
	}
	return false
}

func messages(lang string) map[string]string {
	lang = Normalize(lang)

	mu.RLock()
	cached, ok := loaded[lang]
	mu.RUnlock()
	if ok {
		return cached
	}

	data := embedded(lang)
	if dir := strings.TrimSpace(os.Getenv(EnvI18nDir)); dir != "" {
		if file, err := os.ReadFile(filepath.Join(dir, lang+".ini")); err == nil {
			data = file
		}
	}
	parsed := parseIni(data)

	mu.Lock()
	loaded[lang] = parsed
	mu.Unlock()
	return parsed
}

// T looks up key in the active language and replaces "{}" left-to-right with args.
// T looks up key in the active catalog and replaces "{}" placeholders in order.
//
//	i18n.T("cli.missing_required", "text")
func T(key string, args ...any) string {
	text, ok := messages(Language())[key]
	if !ok {
		text = key
	}
	for _, arg := range args {
		replaced := strings.Replace(text, "{}", util.ToString(arg), 1)
		if replaced == text {
			break
		}
		text = replaced
	}
	return text
}

func parseIni(data []byte) map[string]string {
	out := make(map[string]string)
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key == "" {
			continue
		}
		out[key] = parseValue(strings.TrimSpace(line[idx+1:]))
	}
	return out
}

func parseValue(val string) string {
	if strings.HasPrefix(val, "\"") {
		if end := strings.LastIndex(val[1:], "\""); end >= 0 {
			return val[1 : end+1]
		}
		return strings.TrimPrefix(val, "\"")
	}
	if idx := strings.IndexAny(val, ";#"); idx >= 0 {
		val = val[:idx]
	}
	return strings.TrimSpace(val)
}
