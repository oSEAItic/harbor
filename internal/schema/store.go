package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
)

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

// Store manages learned schemas on the local filesystem.
// Each tool gets one JSON file at HARBOR_HOME/schemas/<sanitized_name>.json.
type Store struct {
	dir string
	mu  sync.RWMutex
}

// NewStore creates a schema store at HARBOR_HOME/schemas (or ~/.harbor/schemas).
func NewStore() (*Store, error) {
	return NewStoreAt(harborhome.Path("schemas"))
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

	return s.getNoLock(toolName)
}

// Save persists a learned schema to disk.
func (s *Store) Save(ls *LearnedSchema) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.saveNoLock(ls)
	return err
}

// History returns all known versions for a tool, sorted by version ascending.
func (s *Store) History(toolName string) ([]LearnedSchema, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.historyNoLock(toolName)
}

// Rollback sets the active schema to a previous version and saves it as a new
// latest version. If targetVersion is 0, the immediate previous version is used.
func (s *Store) Rollback(toolName string, targetVersion int) (*LearnedSchema, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.getNoLock(toolName)
	if current == nil {
		return nil, fmt.Errorf("schema %q not found", toolName)
	}

	history, err := s.historyNoLock(toolName)
	if err != nil {
		return nil, err
	}
	if len(history) < 2 {
		return nil, fmt.Errorf("no previous schema version available for %q", toolName)
	}

	var candidate *LearnedSchema
	if targetVersion > 0 {
		for i := range history {
			if history[i].Version == targetVersion {
				cp := history[i]
				candidate = &cp
				break
			}
		}
		if candidate == nil {
			return nil, fmt.Errorf("schema %q version %d not found", toolName, targetVersion)
		}
	} else {
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Version < current.Version {
				cp := history[i]
				candidate = &cp
				break
			}
		}
		if candidate == nil {
			return nil, fmt.Errorf("no previous schema version available for %q", toolName)
		}
	}

	candidate.ToolName = toolName
	candidate.LearnedAt = time.Now()
	if candidate.LLMModel == "" {
		candidate.LLMModel = "rollback"
	} else {
		candidate.LLMModel = candidate.LLMModel + "+rollback"
	}

	return s.saveNoLock(candidate)
}

func (s *Store) getNoLock(toolName string) *LearnedSchema {
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

func (s *Store) saveNoLock(ls *LearnedSchema) (*LearnedSchema, error) {
	if ls == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if strings.TrimSpace(ls.ToolName) == "" {
		return nil, fmt.Errorf("tool_name is required")
	}
	if ls.LearnedAt.IsZero() {
		ls.LearnedAt = time.Now()
	}

	current := s.getNoLock(ls.ToolName)
	nextVersion := 1
	if current != nil && current.Version >= nextVersion {
		nextVersion = current.Version + 1
	}
	if ls.Version > nextVersion {
		nextVersion = ls.Version
	}

	saved := *ls
	saved.Version = nextVersion

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling schema: %w", err)
	}

	path := s.path(ls.ToolName)

	// Atomic write via temp + rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing schema: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}

	versionPath := s.versionPath(saved.ToolName, saved.Version)
	versionTmp := versionPath + ".tmp"
	if err := os.WriteFile(versionTmp, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing versioned schema: %w", err)
	}
	if err := os.Rename(versionTmp, versionPath); err != nil {
		return nil, err
	}

	return &saved, nil
}

func (s *Store) historyNoLock(toolName string) ([]LearnedSchema, error) {
	sanitized := sanitizeName(toolName)
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("reading schema dir: %w", err)
	}

	var out []LearnedSchema
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, sanitized+".v") || !strings.HasSuffix(name, ".json") {
			continue
		}
		verStr := strings.TrimSuffix(strings.TrimPrefix(name, sanitized+".v"), ".json")
		ver, err := strconv.Atoi(verStr)
		if err != nil {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		var ls LearnedSchema
		if err := json.Unmarshal(data, &ls); err != nil {
			continue
		}
		ls.Version = ver
		out = append(out, ls)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	if len(out) == 0 {
		if current := s.getNoLock(toolName); current != nil {
			out = append(out, *current)
		}
	}
	return out, nil
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

func (s *Store) versionPath(toolName string, version int) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s.v%d.json", sanitizeName(toolName), version))
}

// sanitizeName replaces non-alphanumeric chars (except _ and -) with _.
func sanitizeName(name string) string {
	return sanitizeRe.ReplaceAllString(name, "_")
}
