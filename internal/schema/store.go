package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

// Store manages learned schemas on the local filesystem.
// Each tool gets one JSON file at ~/.harbor/schemas/<sanitized_name>.json.
type Store struct {
	dir string
	mu  sync.RWMutex
}

// NewStore creates a schema store at ~/.harbor/schemas/.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("finding home dir: %w", err)
	}
	return NewStoreAt(filepath.Join(home, ".harbor", "schemas"))
}

// NewStoreAt creates a schema store at the given directory. Useful for testing.
func NewStoreAt(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating schema dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Get loads a learned schema by tool name. Returns nil if not found or corrupted.
func (s *Store) Get(toolName string) *LearnedSchema {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.path(toolName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var ls LearnedSchema
	if err := json.Unmarshal(data, &ls); err != nil {
		return nil
	}

	return &ls
}

// Save persists a learned schema to disk.
func (s *Store) Save(ls *LearnedSchema) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(ls, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling schema: %w", err)
	}

	path := s.path(ls.ToolName)

	// Atomic write via temp + rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing schema: %w", err)
	}

	return os.Rename(tmp, path)
}

// Has returns true if a schema exists for the given tool name.
func (s *Store) Has(toolName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := os.Stat(s.path(toolName))
	return err == nil
}

// path returns the file path for a tool's schema.
func (s *Store) path(toolName string) string {
	return filepath.Join(s.dir, sanitizeName(toolName)+".json")
}

// sanitizeName replaces non-alphanumeric chars (except _ and -) with _.
func sanitizeName(name string) string {
	return sanitizeRe.ReplaceAllString(name, "_")
}
