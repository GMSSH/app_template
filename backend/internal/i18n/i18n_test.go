package i18n

import "testing"

func TestDefaultLanguageIsEnglish(t *testing.T) {
	SetLanguage("")
	t.Cleanup(func() { SetLanguage("") })
	if Language() != "en" {
		t.Fatalf("default lang=%q want en", Language())
	}
	if got := T("cli.unknown_command", "abc"); got != "Unknown command: abc" {
		t.Fatalf("default en text=%q", got)
	}
}

func TestSwitchChinese(t *testing.T) {
	SetLanguage("zh-CN")
	t.Cleanup(func() { SetLanguage("") })
	if got := T("cli.unknown_command", "abc"); got != "未知命令: abc" {
		t.Fatalf("zh-CN text=%q", got)
	}
}

func TestMissingKeyReturnsKey(t *testing.T) {
	SetLanguage("en")
	t.Cleanup(func() { SetLanguage("") })
	if got := T("not.a.real.key"); got != "not.a.real.key" {
		t.Fatalf("missing key should return itself, got %q", got)
	}
}
