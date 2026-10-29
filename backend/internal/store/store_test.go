package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadJSON(t *testing.T) {
	s := New(t.TempDir())
	in := map[string]any{"n": 1.0, "s": "ok"}
	if err := s.WriteJSON("a.json", in); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := s.ReadJSON("a.json", &out); err != nil {
		t.Fatal(err)
	}
	if out["s"] != "ok" {
		t.Fatalf("got %+v", out)
	}
}

func TestRejectsPathEscape(t *testing.T) {
	s := &FileStore{Dir: t.TempDir()}
	if _, err := s.Path("../x.json"); err == nil {
		t.Fatal("expected path escape to be rejected")
	}
}

func TestNestedFile(t *testing.T) {
	dir := t.TempDir()
	s := &FileStore{Dir: dir}
	if err := s.WriteJSON("cache/item.json", map[string]string{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache", "item.json")); err != nil {
		t.Fatal(err)
	}
}
