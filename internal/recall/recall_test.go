package recall

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/oseaitic/harbor/internal/memory"
)

func testStore(t *testing.T) *memory.Store {
	t.Helper()
	s, err := memory.NewStoreAt(t.TempDir())
	if err != nil {
		t.Fatalf("NewStoreAt: %v", err)
	}
	return s
}

func saveTestMemory(t *testing.T, store *memory.Store, connector, resource, summary string, params map[string]string) string {
	t.Helper()
	obj := &memory.Object{
		Connector:  connector,
		Resource:   resource,
		Params:     params,
		TTLSeconds: 600,
		Layers: memory.Layers{
			Raw:        json.RawMessage(`{"raw":true}`),
			Normalized: json.RawMessage(`[{"id":"1"}]`),
			Compact:    json.RawMessage(`[{"id":"1"}]`),
			Summary:    summary,
		},
		Meta: memory.ObjectMeta{
			Source: "test",
		},
	}
	id, err := store.Save(obj)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	return id
}

func callTool(t *testing.T, store *memory.Store, args map[string]interface{}) *mcp.CallToolResult {
	t.Helper()
	handler := MakeHandler(store)

	argsJSON, _ := json.Marshal(args)
	req := mcp.CallToolRequest{}
	req.Params.Name = "harbor_recall"
	req.Params.Arguments = make(map[string]interface{})
	json.Unmarshal(argsJSON, &req.Params.Arguments)

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return result
}

func extractText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestToolDefinition(t *testing.T) {
	tool := ToolDefinition()
	if tool.Name != "harbor_recall" {
		t.Errorf("Name = %q, want %q", tool.Name, "harbor_recall")
	}
	if tool.Description == "" {
		t.Error("Description is empty")
	}
}

func TestHandlerNilStore(t *testing.T) {
	handler := MakeHandler(nil)
	req := mcp.CallToolRequest{}
	req.Params.Name = "harbor_recall"

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for nil store")
	}
}

func TestBrowseEmpty(t *testing.T) {
	store := testStore(t)
	result := callTool(t, store, map[string]interface{}{})
	text := extractText(result)
	if text != "No memories found." {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestBrowseWithMemories(t *testing.T) {
	store := testStore(t)
	saveTestMemory(t, store, "coingecko", "prices", "Bitcoin at $50k", map[string]string{"ids": "bitcoin"})
	saveTestMemory(t, store, "yahoo", "quote", "Apple at $175", map[string]string{"symbols": "AAPL"})

	result := callTool(t, store, map[string]interface{}{})
	text := extractText(result)

	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}

	// Should contain both memories
	if !contains(text, "coingecko") || !contains(text, "yahoo") {
		t.Errorf("expected both connectors in output, got: %s", text)
	}
	if !contains(text, "2 memories found") {
		t.Errorf("expected '2 memories found' in output, got: %s", text)
	}
}

func TestSearchByQuery(t *testing.T) {
	store := testStore(t)
	saveTestMemory(t, store, "coingecko", "prices", "Bitcoin at $50k", map[string]string{"ids": "bitcoin"})
	saveTestMemory(t, store, "yahoo", "quote", "Apple at $175", map[string]string{"symbols": "AAPL"})

	// Search for "bitcoin"
	result := callTool(t, store, map[string]interface{}{"query": "bitcoin"})
	text := extractText(result)

	if !contains(text, "1 memories found") {
		t.Errorf("expected 1 memory found for 'bitcoin', got: %s", text)
	}
	if !contains(text, "coingecko") {
		t.Errorf("expected coingecko in results, got: %s", text)
	}
}

func TestSearchNoResults(t *testing.T) {
	store := testStore(t)
	saveTestMemory(t, store, "coingecko", "prices", "Bitcoin at $50k", nil)

	result := callTool(t, store, map[string]interface{}{"query": "nonexistent"})
	text := extractText(result)

	if !contains(text, "No memories found") {
		t.Errorf("expected no results message, got: %s", text)
	}
}

func TestRetrieveByID(t *testing.T) {
	store := testStore(t)
	id := saveTestMemory(t, store, "coingecko", "prices", "Bitcoin at $50k", nil)

	result := callTool(t, store, map[string]interface{}{"id": id})
	text := extractText(result)

	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}

	// Default layer is compact
	if !contains(text, "id") {
		t.Errorf("expected compact layer content, got: %s", text)
	}
}

func TestRetrieveByIDWithLayer(t *testing.T) {
	store := testStore(t)
	id := saveTestMemory(t, store, "coingecko", "prices", "Bitcoin at $50k", nil)

	// Request summary layer
	result := callTool(t, store, map[string]interface{}{"id": id, "layer": "summary"})
	text := extractText(result)

	if text != "Bitcoin at $50k" {
		t.Errorf("expected summary text, got: %q", text)
	}

	// Request raw layer
	result = callTool(t, store, map[string]interface{}{"id": id, "layer": "raw"})
	text = extractText(result)

	if !contains(text, "raw") {
		t.Errorf("expected raw layer content, got: %s", text)
	}
}

func TestRetrieveNonExistent(t *testing.T) {
	store := testStore(t)

	result := callTool(t, store, map[string]interface{}{"id": "mem_nonexist"})
	if !result.IsError {
		t.Error("expected error for non-existent memory")
	}
}

func TestBrowseWithConnectorFilter(t *testing.T) {
	store := testStore(t)
	saveTestMemory(t, store, "coingecko", "prices", "Bitcoin", nil)
	saveTestMemory(t, store, "yahoo", "quote", "Apple", nil)

	result := callTool(t, store, map[string]interface{}{"connector": "yahoo"})
	text := extractText(result)

	if !contains(text, "1 memories found") {
		t.Errorf("expected 1 memory with connector filter, got: %s", text)
	}
	if !contains(text, "yahoo") {
		t.Errorf("expected yahoo in results, got: %s", text)
	}
}

func TestBrowseWithSinceFilter(t *testing.T) {
	store := testStore(t)
	saveTestMemory(t, store, "coingecko", "prices", "Bitcoin", nil)

	// "since" with very short duration — just-created memory should match
	result := callTool(t, store, map[string]interface{}{"since": "1h"})
	text := extractText(result)

	if !contains(text, "1 memories found") {
		t.Errorf("expected 1 memory within 1h, got: %s", text)
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s ago"},
		{5 * time.Minute, "5m ago"},
		{2 * time.Hour, "2h ago"},
		{2*time.Hour + 30*time.Minute, "2h30m ago"},
		{48 * time.Hour, "2d ago"},
	}

	for _, tt := range tests {
		got := formatAge(tt.d)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
