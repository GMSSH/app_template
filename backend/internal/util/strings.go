package util

import (
	"fmt"
	"strings"
)

// ContainsString reports whether value is present in list.
func ContainsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// SplitCSV splits a comma-separated list and trims empty segments.
//
//	util.SplitCSV("a, b, c") // []string{"a", "b", "c"}
func SplitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// ToString converts v for message formatting; newlines become spaces.
func ToString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case error:
		return val.Error()
	default:
		return strings.ReplaceAll(fmt.Sprint(val), "\n", " ")
	}
}
