// Package store abstracts persistence under the plugin data directory.
//
// Default implementation is a JSON file store (FileStore). Business code should
// depend on the Store interface so alternate backends can be injected later
// (DIP / polymorphism).
//
// The command engine injects store.New(paths.DataDir()) into middleware.Context.
// Handlers typically use ctx.Store:
//
//	type prefs struct{ Theme string `json:"theme"` }
//
//	func Run(ctx *middleware.Context, in input.Input) (any, error) {
//	    var p prefs
//	    if err := ctx.Store.ReadJSON("prefs.json", &p); err != nil && !os.IsNotExist(err) {
//	        return nil, err
//	    }
//	    p.Theme = in.Str("theme")
//	    if err := ctx.Store.WriteJSON("prefs.json", p); err != nil {
//	        return nil, err
//	    }
//	    return p, nil
//	}
//
// Custom backend example:
//
//	type MemoryStore struct{ data map[string][]byte }
//	func (m *MemoryStore) ReadJSON(name string, v any) error { ... }
//	func (m *MemoryStore) WriteJSON(name string, v any) error { ... }
//	// then assign ctx.Store = m in tests or a custom middleware
package store

import (
	"encoding/json"
	"fmt"
	"os"

	"gmssh-cli-plugin/internal/util"
)

// Store is the persistence port (Interface Segregation / DIP).
// name is a relative path under the store root (e.g. "prefs.json").
type Store interface {
	ReadJSON(name string, v any) error
	WriteJSON(name string, v any) error
}

// FileStore is the default JSON-on-disk adapter rooted at Dir.
type FileStore struct {
	Dir string
}

// New constructs the default file-backed Store rooted at dir.
//
//	s := store.New(paths.DataDir())
func New(dir string) Store {
	return &FileStore{Dir: dir}
}

// Path resolves a relative name under Dir and rejects ".." traversal.
func (s *FileStore) Path(name string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("store: nil store")
	}
	full, err := util.ResolveUnderRoot(s.Dir, name)
	if err != nil {
		return "", fmt.Errorf("store: %w", err)
	}
	return full, nil
}

// ReadJSON unmarshals the named JSON document into v.
func (s *FileStore) ReadJSON(name string, v any) error {
	path, err := s.Path(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// WriteJSON marshals v as indented JSON (mode 0600) under Dir.
func (s *FileStore) WriteJSON(name string, v any) error {
	path, err := s.Path(name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return util.WriteFile(path, data, 0o600)
}
