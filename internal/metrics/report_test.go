package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oseaitic/harbor/internal/proxy"
)

func TestBuildSummary(t *testing.T) {
	now := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	events := []proxy.CompressionMetric{
		{
			Timestamp:        now.Add(-10 * time.Minute),
			ToolName:         "list_files",
			SchemaApplied:    true,
			RawBytes:         400,
			CompactBytes:     100,
			CompressionRatio: 0.25,
		},
		{
			Timestamp:     now.Add(-9 * time.Minute),
			ToolName:      "list_files",
			FromMemory:    true,
			SchemaApplied: false,
		},
		{
			Timestamp:     now.Add(-8 * time.Minute),
			ToolName:      "search_docs",
			SchemaApplied: true,
			RawBytes:      200,
			CompactBytes:  120,
			DriftDetected: true,
		},
	}

	got := BuildSummary("/tmp/proxy.jsonl", events, Query{
		Since: 1 * time.Hour,
		Now:   now,
	})

	if got.TotalCalls != 3 {
		t.Fatalf("TotalCalls = %d, want 3", got.TotalCalls)
	}
	if got.ApproxTokensSavedTotal <= 0 {
		t.Fatalf("ApproxTokensSavedTotal = %d, want > 0", got.ApproxTokensSavedTotal)
	}
	if got.MemoryHitRate <= 0 {
		t.Fatalf("MemoryHitRate = %f, want > 0", got.MemoryHitRate)
	}
	if len(got.Tools) != 2 {
		t.Fatalf("Tools len = %d, want 2", len(got.Tools))
	}
	if got.Tools[0].ToolName != "list_files" {
		t.Fatalf("top tool = %q, want list_files", got.Tools[0].ToolName)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxy.jsonl")

	content := `{"ts":"2026-02-24T10:00:00Z","tool_name":"a","from_memory":false,"schema_applied":true,"raw_bytes":100,"compact_bytes":50}
{"ts":"2026-02-24T10:01:00Z","tool_name":"b","from_memory":true,"schema_applied":false}
not-json
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
}
