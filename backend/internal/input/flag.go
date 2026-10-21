package input

import (
	"strconv"
	"strings"

	"gmssh-cli-plugin/internal/util"
)

// SplitFlag parses "-n=v" / "--name=value" into name, optional value, and hasValue.
//
//	SplitFlag("--lang=zh-CN") → ("lang", "zh-CN", true)
//	SplitFlag("--json")       → ("json", "", false)
func SplitFlag(item string) (name, value string, hasValue bool) {
	body := strings.TrimLeft(item, "-")
	if idx := strings.Index(body, "="); idx >= 0 {
		return body[:idx], body[idx+1:], true
	}
	return body, "", false
}

// BoolLiteralValue maps common truthy/falsey tokens to "true"/"false".
func BoolLiteralValue(token string) (string, bool) {
	return util.BoolLiteral(token)
}

// IsNumberLiteral reports whether token looks like a negative numeric literal
// (so PeekFlagValue does not treat it as another flag).
func IsNumberLiteral(token string) bool {
	if len(token) < 2 || token[0] != '-' {
		return false
	}
	_, err := strconv.ParseFloat(token, 64)
	return err == nil
}

// UnknownFlagLabel formats a token for error messages (-x vs --name).
func UnknownFlagLabel(token string) string {
	name, _, _ := SplitFlag(token)
	if name == "" {
		return "--"
	}
	if strings.HasPrefix(token, "-") && !strings.HasPrefix(token, "--") {
		return "-" + name
	}
	return "--" + name
}

// PeekFlagValue returns argv[i+1] when it is a valid flag value token.
func PeekFlagValue(argv []string, i int) (string, bool) {
	if i+1 >= len(argv) {
		return "", false
	}
	next := argv[i+1]
	if strings.HasPrefix(next, "-") && next != "-" && !IsNumberLiteral(next) {
		return "", false
	}
	return next, true
}

// PeekBoolLiteral returns a normalized bool literal from argv[i+1], if present.
func PeekBoolLiteral(argv []string, i int) (string, bool) {
	if i+1 >= len(argv) {
		return "", false
	}
	return BoolLiteralValue(argv[i+1])
}
