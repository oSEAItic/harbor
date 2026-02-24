package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetricsLoggerLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	l, err := NewMetricsLogger()
	if err != nil {
		t.Fatalf("NewMetricsLogger: %v", err)
	}

	l.Log(CompressionMetric{
		ToolName:      "list_files",
		SchemaApplied: true,
		RawBytes:      100,
		CompactBytes:  25,
	})

	path := filepath.Join(home, ".harbor", "metrics", "proxy_compression.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metrics file: %v", err)
	}
	if !strings.Contains(string(data), `"tool_name":"list_files"`) {
		t.Fatalf("metrics line missing tool_name: %s", string(data))
	}
}
