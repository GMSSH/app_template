package hello

import (
	"testing"

	"gmssh-cli-plugin/internal/i18n"
)

func TestGreet(t *testing.T) {
	i18n.SetLanguage("en")
	t.Cleanup(func() { i18n.SetLanguage("") })
	got, err := Greet(" world ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello, world" {
		t.Fatalf("got %q", got.Text)
	}
}

func TestGreetZH(t *testing.T) {
	i18n.SetLanguage("zh-CN")
	t.Cleanup(func() { i18n.SetLanguage("") })
	got, err := Greet("world")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "你好，world" {
		t.Fatalf("got %q", got.Text)
	}
}
