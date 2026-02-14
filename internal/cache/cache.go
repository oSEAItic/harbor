package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry is a cached response with TTL.
type Entry struct {
	Response json.RawMessage `json:"response"`
	CachedAt time.Time       `json:"cached_at"`
	TTL      time.Duration   `json:"ttl"`
}

// Cache provides a local filesystem cache for connector responses.
type Cache struct {
	dir string
	ttl time.Duration
}

// New creates a cache with the given TTL.
func New(ttl time.Duration) *Cache {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".harbor", "cache")
	os.MkdirAll(dir, 0o755)
	return &Cache{dir: dir, ttl: ttl}
}

// Key builds a cache key from connector, resource, and params.
func Key(connector, resource string, params map[string]string) string {
	// Sort params for deterministic hashing
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}

	raw := fmt.Sprintf("%s.%s?%s", connector, resource, strings.Join(parts, "&"))
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response, or nil if not found / expired.
func (c *Cache) Get(key string) json.RawMessage {
	path := filepath.Join(c.dir, key+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}

	if time.Since(entry.CachedAt) > c.ttl {
		os.Remove(path)
		return nil
	}

	return entry.Response
}

// Set stores a response in the cache.
func (c *Cache) Set(key string, response json.RawMessage) error {
	entry := Entry{
		Response: response,
		CachedAt: time.Now(),
		TTL:      c.ttl,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	path := filepath.Join(c.dir, key+".json")
	return os.WriteFile(path, data, 0o644)
}

// Clear removes all cached entries.
func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		os.Remove(filepath.Join(c.dir, e.Name()))
	}
	return nil
}
