package util

import "strings"

// DisplayWidth returns the terminal column width of text (CJK runes count as 2).
func DisplayWidth(text string) int {
	width := 0
	for _, r := range text {
		if IsWideRune(r) {
			width += 2
			continue
		}
		width++
	}
	return width
}

// IsWideRune reports East-Asian wide runes used for help alignment.
func IsWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F,
		r >= 0x2E80 && r <= 0xA4CF,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE6F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6:
		return true
	default:
		return false
	}
}

// PadRight pads text with spaces until DisplayWidth reaches width.
func PadRight(text string, width int) string {
	if pad := width - DisplayWidth(text); pad > 0 {
		return text + strings.Repeat(" ", pad)
	}
	return text
}
