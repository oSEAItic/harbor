package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
)

// MetricsLogger appends per-call proxy compression metrics to a JSONL file.
type MetricsLogger struct {
	mu   sync.Mutex
	path string
}

// CompressionMetric captures one proxied tool call outcome.
type CompressionMetric struct {
	Timestamp        time.Time `json:"ts"`
	ToolName         string    `json:"tool_name"`
	FromMemory       bool      `json:"from_memory"`
	SchemaApplied    bool      `json:"schema_applied"`
	DriftDetected    bool      `json:"drift_detected"`
	RawBytes         int       `json:"raw_bytes"`
	CompactBytes     int       `json:"compact_bytes"`
	CompressionRatio float64   `json:"compression_ratio"`
	FieldHitRate     float64   `json:"field_hit_rate"`
	Items            int       `json:"items"`
}

// DefaultMetricsPath returns the default metrics JSONL path.
func DefaultMetricsPath() (string, error) {
	dir := harborhome.Path("metrics")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating metrics dir: %w", err)
	}
	return filepath.Join(dir, "proxy_compression.jsonl"), nil
}

// NewMetricsLogger creates a logger at HARBOR_HOME/metrics/proxy_compression.jsonl.
func NewMetricsLogger() (*MetricsLogger, error) {
	path, err := DefaultMetricsPath()
	if err != nil {
		return nil, err
	}
	return &MetricsLogger{path: path}, nil
}

// Path returns the JSONL target path.
func (l *MetricsLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Log appends an event as one JSON line.
func (l *MetricsLogger) Log(m CompressionMetric) {
	if l == nil {
		return
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}

	b, err := json.Marshal(m)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(append(b, '\n'))
}
