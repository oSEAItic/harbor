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

// TestCrossSessionMemory verifies that data saved by one Harbor proxy session
// is fully accessible from a completely independent session via harbor_recall.
// This is the core value proposition: session 1 queries GitHub issues through
// Harbor proxy, session 2 starts fresh and retrieves the data without any
// upstream API call.
func TestCrossSessionMemory(t *testing.T) {
	// Use a shared directory that both "sessions" will point to
	sharedDir := t.TempDir()

	// ── Session 1: Proxy saves GitHub issues to memory ──
	t.Log("Session 1: Simulating proxy saving GitHub issues")

	session1Store, err := memory.NewStoreAt(sharedDir)
	if err != nil {
		t.Fatal(err)
	}

	// Save realistic data mimicking what proxyHandler.saveToMemory() does
	rawGitHub := json.RawMessage(`[
		{"number":201,"title":"Memory leak in WebSocket pool","state":"open","user":{"login":"charlie"},"labels":[{"name":"bug"},{"name":"critical"}],"assignee":{"login":"frank"},"body":"Connection pool grows unbounded...","comments":15,"created_at":"2026-02-10T10:00:00Z"},
		{"number":202,"title":"Add dark mode support","state":"open","user":{"login":"alice"},"labels":[{"name":"enhancement"},{"name":"frontend"}],"assignee":null,"body":"Users want dark mode...","comments":22,"created_at":"2026-01-05T10:00:00Z"},
		{"number":211,"title":"Rate limiting not working for /api/auth","state":"open","user":{"login":"eve"},"labels":[{"name":"bug"},{"name":"critical"},{"name":"security"}],"assignee":null,"body":"Auth endpoints not rate limited...","comments":11,"created_at":"2026-02-20T10:00:00Z"}
	]`)

	compactGitHub := json.RawMessage(`[
		{"number":201,"title":"Memory leak in WebSocket pool","state":"open","user":"charlie","assignee":"frank"},
		{"number":202,"title":"Add dark mode support","state":"open","user":"alice","assignee":null},
		{"number":211,"title":"Rate limiting not working for /api/auth","state":"open","user":"eve","assignee":null}
	]`)

	obj := &memory.Object{
		Connector:  "proxy",
		Resource:   "list_issues",
		Schema:     "proxy.list_issues.v2",
		Params:     map[string]string{"state": "open", "repo": "org/project"},
		TTLSeconds: 600,
		Layers: memory.Layers{
			Raw:        rawGitHub,
			Normalized: rawGitHub,
			Compact:    compactGitHub,
			Summary:    "3 open issues: #201 critical WebSocket leak (assigned: frank), #202 dark mode request, #211 critical auth rate limit not working",
		},
		Meta: memory.ObjectMeta{
			Source:           "harbor-proxy",
			ConnectorVersion: "1.0.0",
		},
	}

	savedID, err := session1Store.Save(obj)
	if err != nil {
		t.Fatalf("Session 1 save: %v", err)
	}
	t.Logf("Session 1: Saved memory %s (%d bytes raw, %d bytes compact)",
		savedID, len(rawGitHub), len(compactGitHub))

	// Session 1 store goes out of scope — simulates process exit
	session1Store = nil

	// ── Session 2: Fresh start, only reads from disk ──
	t.Log("Session 2: New process reads from same directory")

	session2Store, err := memory.NewStoreAt(sharedDir)
	if err != nil {
		t.Fatal(err)
	}

	handler := MakeHandler(session2Store)

	// Test 1: Browse all memories
	t.Log("  Test 1: Browse all memories")
	browseResult := callToolWith(t, handler, map[string]interface{}{})
	browseText := extractText(browseResult)
	if !contains(browseText, "1 memories found") {
		t.Errorf("browse should find 1 memory, got: %s", browseText)
	}
	if !contains(browseText, "proxy.list_issues") {
		t.Errorf("browse should show proxy.list_issues, got: %s", browseText)
	}
	if !contains(browseText, savedID) {
		t.Errorf("browse should show saved ID %s, got: %s", savedID, browseText)
	}
	t.Logf("  Browse output:\n%s", browseText)

	// Test 2: Search by keyword
	t.Log("  Test 2: Search by keyword 'issues'")
	searchResult := callToolWith(t, handler, map[string]interface{}{"query": "issues"})
	searchText := extractText(searchResult)
	if !contains(searchText, "1 memories found") {
		t.Errorf("search should find 1 memory matching 'issues', got: %s", searchText)
	}
	t.Logf("  Search output:\n%s", searchText)

	// Test 3: Retrieve compact layer (default)
	t.Log("  Test 3: Retrieve compact layer by ID")
	compactResult := callToolWith(t, handler, map[string]interface{}{"id": savedID})
	compactText := extractText(compactResult)
	if compactResult.IsError {
		t.Fatalf("retrieve compact failed: %s", compactText)
	}
	if !contains(compactText, "201") || !contains(compactText, "WebSocket") {
		t.Errorf("compact layer should contain issue #201, got: %s", compactText)
	}
	if !contains(compactText, `"frank"`) {
		t.Errorf("compact layer should contain assignee frank, got: %s", compactText)
	}
	t.Logf("  Compact layer (%d bytes):\n%s", len(compactText), compactText)

	// Test 4: Retrieve summary layer
	t.Log("  Test 4: Retrieve summary layer")
	summaryResult := callToolWith(t, handler, map[string]interface{}{"id": savedID, "layer": "summary"})
	summaryText := extractText(summaryResult)
	if !contains(summaryText, "critical") {
		t.Errorf("summary should mention 'critical', got: %s", summaryText)
	}
	t.Logf("  Summary: %s", summaryText)

	// Test 5: Retrieve raw layer (full original API data)
	t.Log("  Test 5: Retrieve raw layer")
	rawResult := callToolWith(t, handler, map[string]interface{}{"id": savedID, "layer": "raw"})
	rawText := extractText(rawResult)
	if !contains(rawText, "login") {
		t.Errorf("raw layer should contain user.login, got: %s", rawText[:100])
	}
	if !contains(rawText, "assignee") {
		t.Errorf("raw layer should contain assignee field, got: %s", rawText[:100])
	}
	t.Logf("  Raw layer (%d bytes)", len(rawText))

	// Test 6: Filter by connector
	t.Log("  Test 6: Filter by connector=proxy")
	connResult := callToolWith(t, handler, map[string]interface{}{"connector": "proxy"})
	connText := extractText(connResult)
	if !contains(connText, "1 memories found") {
		t.Errorf("connector filter should find 1 memory, got: %s", connText)
	}

	// Test 7: Filter by non-existent connector
	t.Log("  Test 7: Filter by non-existent connector")
	noResult := callToolWith(t, handler, map[string]interface{}{"connector": "nonexistent"})
	noText := extractText(noResult)
	if !contains(noText, "No memories found") {
		t.Errorf("non-existent connector should return no results, got: %s", noText)
	}

	t.Log("Cross-session memory validation: ALL PASS")
}

// callToolWith calls harbor_recall with a pre-created handler.
func callToolWith(t *testing.T, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]interface{}) *mcp.CallToolResult {
	t.Helper()
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
