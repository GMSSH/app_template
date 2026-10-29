package util

import (
	"strconv"
	"strings"
)

// ParseBool accepts common truthy/falsey tokens (true/yes/on/1, false/no/off/0).
// The second return value is false when the token is not recognized.
//
//	on, ok := util.ParseBool("yes") // true, true
func ParseBool(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "t", "true", "yes", "on":
		return true, true
	case "0", "f", "false", "no", "off":
		return false, true
	default:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, false
		}
		return parsed, true
	}
}

// BoolLiteral normalizes a token to "true"/"false" for flag rewriting.
func BoolLiteral(token string) (string, bool) {
	on, ok := ParseBool(token)
	if !ok {
		return "", false
	}
	if on {
		return "true", true
	}
	return "false", true
}
