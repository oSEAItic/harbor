package memory

import (
	"encoding/json"
	"time"
)

// LayerName identifies a memory layer.
type LayerName string

const (
	LayerRaw        LayerName = "raw"
	LayerNormalized LayerName = "normalized"
	LayerCompact    LayerName = "compact"
	LayerSummary    LayerName = "summary"
)

// Object is the 4-layer memory unit persisted after every harbor get.
type Object struct {
	ID         string            `json:"id"`
	Connector  string            `json:"connector"`
	Resource   string            `json:"resource"`
	Schema     string            `json:"schema"`
	Params     map[string]string `json:"params,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	TTLSeconds int               `json:"ttl_seconds"`
	Author     string            `json:"author,omitempty"`
	Refs       []string          `json:"refs,omitempty"`
	Layers     Layers            `json:"layers"`
	Meta       ObjectMeta        `json:"meta"`
}

// Layers holds the four tiers of a memory object.
type Layers struct {
	Raw        json.RawMessage `json:"raw,omitempty"`        // Original API JSON
	Normalized json.RawMessage `json:"normalized,omitempty"` // Connector's data[]
	Compact    json.RawMessage `json:"compact,omitempty"`    // Summary fields only
	Summary    string          `json:"summary,omitempty"`    // Natural language
}

// ObjectMeta holds provenance information for a memory object.
type ObjectMeta struct {
	Source           string `json:"source"`
	ConnectorVersion string `json:"connector_version"`
	RequestID        string `json:"request_id"`
	ContentHash      string `json:"content_hash"`
	BytesRaw         int    `json:"bytes_raw"`
	BytesCompact     int    `json:"bytes_compact"`
}

// IndexEntry is a lightweight subset stored in index.json for fast lookup.
type IndexEntry struct {
	ID           string            `json:"id"`
	Connector    string            `json:"connector"`
	Resource     string            `json:"resource"`
	Schema       string            `json:"schema"`
	Params       map[string]string `json:"params,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	TTLSeconds   int               `json:"ttl_seconds"`
	BytesRaw     int               `json:"bytes_raw"`
	BytesCompact int               `json:"bytes_compact"`
	Summary      string            `json:"summary,omitempty"`
	Author       string            `json:"author,omitempty"`
}

// SearchResult is a memory entry with a relevance score for search results.
type SearchResult struct {
	IndexEntry
	Score int
	Age   time.Duration
	Fresh bool
}

// Index is the on-disk index file for fast memory lookups.
type Index struct {
	Version   int          `json:"version"`
	UpdatedAt time.Time    `json:"updated_at"`
	Entries   []IndexEntry `json:"entries"`
}

// QueryOptions filters memory queries.
type QueryOptions struct {
	Connector string
	Resource  string
	Params    map[string]string
	Since     time.Duration
	Limit     int
}
