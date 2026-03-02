package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
)

const (
	maxIndexEntries = 500
	pruneMaxAge     = 7 * 24 * time.Hour // 7 days
	maxNoteVersions = 5                  // soft versioning: keep last N notes per connector

	// SchemaNote identifies connector-level notes written by harbor_remember.
	// Notes accumulate up to maxNoteVersions; oldest are pruned on each write.
	SchemaNote = "harbor.note.v1"
)

// Store manages memory objects on the local filesystem.
type Store struct {
	dir       string
	objDir    string
	mu        sync.Mutex
	CloudPush func(key, content string) // nil = no-op; injected by CLI layer to avoid import cycle
}

// NewStore creates a memory store at HARBOR_HOME/memory (or ~/.harbor/memory).
func NewStore() (*Store, error) {
	return NewStoreAt(harborhome.Path("memory"))
}

// NewStoreAt creates a memory store at the given directory.
// Useful for testing with isolated temp directories.
func NewStoreAt(dir string) (*Store, error) {
	objDir := filepath.Join(dir, "objects")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating memory dir: %w", err)
	}
	s := &Store{dir: dir, objDir: objDir}
	s.maybeRebuildIndex()
	return s, nil
}

// maybeRebuildIndex checks whether the index is missing entries for existing
// objects (e.g. objects saved by an older binary that didn't maintain the index)
// and triggers a full rebuild if needed. Called once at store creation.
func (s *Store) maybeRebuildIndex() {
	entries, err := os.ReadDir(s.objDir)
	if err != nil {
		return
	}
	var objCount int
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			objCount++
		}
	}
	if objCount == 0 {
		return
	}
	idx := loadIndex(s.dir)
	if len(idx.Entries) < objCount {
		rebuildIndex(s.dir)
	}
}

// Save persists a memory object and updates the index. Returns the object ID.
func (s *Store) Save(obj *Object) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID
	obj.ID = generateID(obj.Connector, obj.Resource, obj.Params)
	obj.CreatedAt = time.Now().UTC()

	// Compute content hash
	hash := sha256.Sum256(obj.Layers.Normalized)
	obj.Meta.ContentHash = hex.EncodeToString(hash[:16])

	// Compute byte sizes
	obj.Meta.BytesRaw = len(obj.Layers.Raw)
	obj.Meta.BytesCompact = len(obj.Layers.Compact)

	// Write object file
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling memory object: %w", err)
	}

	objPath := filepath.Join(s.objDir, obj.ID+".json")
	if err := os.WriteFile(objPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing memory object: %w", err)
	}

	// Update index
	idx := loadIndex(s.dir)

	// Notes (harbor.note.v1) accumulate — each call creates a new entry.
	// All other schemas deduplicate: only the latest fetch per connector+resource+params is kept.
	if obj.Schema != SchemaNote {
		key := canonicalKey(obj.Connector, obj.Resource, obj.Params)
		filtered := idx.Entries[:0]
		for _, e := range idx.Entries {
			if canonicalKey(e.Connector, e.Resource, e.Params) != key {
				filtered = append(filtered, e)
			} else {
				os.Remove(filepath.Join(s.objDir, e.ID+".json"))
			}
		}
		idx.Entries = filtered
	}

	// Add new entry
	idx.Entries = append(idx.Entries, IndexEntry{
		ID:           obj.ID,
		Connector:    obj.Connector,
		Resource:     obj.Resource,
		Schema:       obj.Schema,
		Params:       obj.Params,
		CreatedAt:    obj.CreatedAt,
		TTLSeconds:   obj.TTLSeconds,
		BytesRaw:     obj.Meta.BytesRaw,
		BytesCompact: obj.Meta.BytesCompact,
		Summary:      obj.Layers.Summary,
	})

	// Note soft versioning: keep only the most recent maxNoteVersions per connector.
	if obj.Schema == SchemaNote {
		var noteEntries []IndexEntry
		for _, e := range idx.Entries {
			if e.Connector == obj.Connector && e.Resource == obj.Resource && e.Schema == SchemaNote {
				noteEntries = append(noteEntries, e)
			}
		}
		if len(noteEntries) > maxNoteVersions {
			sort.Slice(noteEntries, func(i, j int) bool {
				return noteEntries[i].CreatedAt.Before(noteEntries[j].CreatedAt)
			})
			pruneSet := make(map[string]bool)
			for _, e := range noteEntries[:len(noteEntries)-maxNoteVersions] {
				pruneSet[e.ID] = true
				os.Remove(filepath.Join(s.objDir, e.ID+".json"))
			}
			fresh := idx.Entries[:0]
			for _, e := range idx.Entries {
				if !pruneSet[e.ID] {
					fresh = append(fresh, e)
				}
			}
			idx.Entries = fresh
		}
	}

	// Lazy pruning: if index exceeds max size, prune old entries
	if len(idx.Entries) > maxIndexEntries {
		s.pruneOld(idx, pruneMaxAge)
	}

	if err := saveIndex(s.dir, idx); err != nil {
		return "", fmt.Errorf("saving index: %w", err)
	}

	// Best-effort async cloud push — non-blocking, never fails the local save.
	if s.CloudPush != nil && obj.Layers.Summary != "" {
		key := obj.Connector + "." + obj.Resource
		summary := obj.Layers.Summary
		go s.CloudPush(key, summary)
	}

	return obj.ID, nil
}

// SaveNote persists a connector-level analysis note written by an agent via harbor_remember.
// Notes use Resource="_context" and SchemaNote to distinguish them from data fetch objects.
// Multiple notes for the same connector accumulate (no deduplication).
func (s *Store) SaveNote(connector, note string) (string, error) {
	obj := &Object{
		Connector:  connector,
		Resource:   "_context",
		Schema:     SchemaNote,
		TTLSeconds: 0, // notes never expire
		Layers:     Layers{Summary: note},
	}
	return s.Save(obj)
}

// Get loads a full memory object by ID.
func (s *Store) Get(id string) (*Object, error) {
	path := filepath.Join(s.objDir, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading memory object %q: %w", id, err)
	}

	var obj Object
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parsing memory object %q: %w", id, err)
	}

	return &obj, nil
}

// Latest finds the most recent index entry matching connector, resource, and params.
func (s *Store) Latest(connector, resource string, params map[string]string) *IndexEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := loadIndex(s.dir)
	key := canonicalKey(connector, resource, params)

	for i := len(idx.Entries) - 1; i >= 0; i-- {
		e := &idx.Entries[i]
		if canonicalKey(e.Connector, e.Resource, e.Params) == key {
			return e
		}
	}

	return nil
}

// Query returns index entries matching the given options.
func (s *Store) Query(opts QueryOptions) []IndexEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := loadIndex(s.dir)
	var results []IndexEntry

	for _, e := range idx.Entries {
		if opts.Connector != "" && e.Connector != opts.Connector {
			continue
		}
		if opts.Resource != "" && e.Resource != opts.Resource {
			continue
		}
		if opts.Since > 0 && time.Since(e.CreatedAt) > opts.Since {
			continue
		}
		if opts.Params != nil && !paramsMatch(e.Params, opts.Params) {
			continue
		}
		results = append(results, e)
	}

	// Sort by creation time, newest first
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results
}

// IsFresh checks if an index entry is still within its TTL.
func (s *Store) IsFresh(entry *IndexEntry) bool {
	if entry == nil {
		return false
	}
	ttl := time.Duration(entry.TTLSeconds) * time.Second
	return time.Since(entry.CreatedAt) <= ttl
}

// Prune removes entries older than maxAge. Returns the count of pruned entries.
func (s *Store) Prune(maxAge time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := loadIndex(s.dir)
	count := s.pruneOld(idx, maxAge)

	if err := saveIndex(s.dir, idx); err != nil {
		return 0, err
	}

	return count, nil
}

// pruneOld removes entries older than maxAge from the index and deletes their files.
// Must be called with s.mu held.
func (s *Store) pruneOld(idx *Index, maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	count := 0
	filtered := idx.Entries[:0]

	for _, e := range idx.Entries {
		if e.CreatedAt.Before(cutoff) {
			os.Remove(filepath.Join(s.objDir, e.ID+".json"))
			count++
		} else {
			filtered = append(filtered, e)
		}
	}

	idx.Entries = filtered
	return count
}

// generateID creates a memory ID from connector, resource, params, and timestamp.
// Format: "mem_" + SHA256(connector.resource?params@timestamp)[:8]
func generateID(connector, resource string, params map[string]string) string {
	raw := fmt.Sprintf("%s.%s?%s@%d",
		connector, resource,
		sortedParams(params),
		time.Now().UnixNano(),
	)
	hash := sha256.Sum256([]byte(raw))
	return "mem_" + hex.EncodeToString(hash[:])[:8]
}

// canonicalKey builds a deterministic lookup key for connector+resource+params.
func canonicalKey(connector, resource string, params map[string]string) string {
	return fmt.Sprintf("%s.%s?%s", connector, resource, sortedParams(params))
}

// sortedParams returns a deterministic string representation of params.
func sortedParams(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	return strings.Join(parts, "&")
}

// paramsMatch returns true if all query params are present and equal in the entry params.
func paramsMatch(entryParams, queryParams map[string]string) bool {
	for k, v := range queryParams {
		if entryParams[k] != v {
			return false
		}
	}
	return true
}

// Search finds memory entries matching a keyword query. Tokens are matched
// against connector, resource, param values, summary, and schema. Results
// are scored by match count and sorted by score then recency.
func (s *Store) Search(query string, opts QueryOptions) []SearchResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := loadIndex(s.dir)
	tokens := tokenize(query)

	var results []SearchResult

	for i := range idx.Entries {
		e := &idx.Entries[i]

		// Apply structural filters
		if opts.Connector != "" && e.Connector != opts.Connector {
			continue
		}
		if opts.Resource != "" && e.Resource != opts.Resource {
			continue
		}
		if opts.Since > 0 && time.Since(e.CreatedAt) > opts.Since {
			continue
		}
		if opts.Params != nil && !paramsMatch(e.Params, opts.Params) {
			continue
		}

		// Score by keyword match
		score := 0
		if len(tokens) == 0 {
			score = 1 // no query = match everything
		} else {
			searchable := strings.ToLower(strings.Join([]string{
				e.Connector, e.Resource, e.Schema, e.Summary,
				paramValuesString(e.Params),
			}, " "))

			for _, token := range tokens {
				if strings.Contains(searchable, token) {
					score++
				}
			}
		}

		if score > 0 {
			results = append(results, SearchResult{
				IndexEntry: *e,
				Score:      score,
				Age:        time.Since(e.CreatedAt),
				Fresh:      s.IsFresh(e),
			})
		}
	}

	// Sort by score desc, then recency desc
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results
}

// tokenize splits a query into lowercase, deduplicated tokens.
func tokenize(query string) []string {
	words := strings.Fields(strings.ToLower(query))
	seen := make(map[string]bool, len(words))
	var tokens []string
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// paramValuesString joins all param values into a single string for search.
func paramValuesString(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	vals := make([]string, 0, len(params))
	for _, v := range params {
		vals = append(vals, v)
	}
	return strings.Join(vals, " ")
}

// DefaultTTL returns the default TTL for a connector+resource combination.
func DefaultTTL(connector, resource string) time.Duration {
	key := connector + "." + resource

	// Pattern-based TTLs
	patterns := map[string]time.Duration{
		"prices":   5 * time.Minute,
		"quote":    5 * time.Minute,
		"trending": 15 * time.Minute,
		"coin":     30 * time.Minute,
		"summary":  30 * time.Minute,
		"search":   30 * time.Minute,
	}

	// Check exact match first
	for pattern, ttl := range patterns {
		if strings.HasSuffix(key, "."+pattern) {
			return ttl
		}
	}

	// Fallback
	return 10 * time.Minute
}
