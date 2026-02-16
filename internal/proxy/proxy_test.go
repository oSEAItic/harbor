package proxy

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/schema"
)

// sampleFiles is test data that looks like a filesystem listing.
var sampleFiles = `[{"name":"readme.md","size":1024,"type":"file","permissions":"rw-r--r--","modified":"2024-01-15"},{"name":"src","size":4096,"type":"directory","permissions":"rwxr-xr-x","modified":"2024-01-20"},{"name":"go.mod","size":256,"type":"file","permissions":"rw-r--r--","modified":"2024-01-10"}]`

func setupMockUpstream(t *testing.T) (*client.Client, *server.MCPServer) {
	t.Helper()

	s := server.NewMCPServer("test-upstream", "1.0.0")

	tool := mcp.NewToolWithRawSchema(
		"list_files",
		"List files in a directory",
		json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(sampleFiles), nil
	})

	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1.0.0"}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatal(err)
	}

	return c, s
}

func TestProxyHandlerRawPassthroughWithHint(t *testing.T) {
	upstreamClient, _ := setupMockUpstream(t)
	defer upstreamClient.Close()

	schemaStore, err := schema.NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	memStore, err := memory.NewStoreAt(filepath.Join(t.TempDir(), "memory"))
	if err != nil {
		t.Fatal(err)
	}

	h := &proxyHandler{
		upstream:    upstreamClient,
		toolName:    "list_files",
		schemaStore: schemaStore,
		memStore:    memStore,
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "list_files"
	req.Params.Arguments = map[string]interface{}{"path": "/tmp"}

	result, err := h.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	text := extractTextContent(result)

	// Should contain the raw output
	if !strings.Contains(text, `"readme.md"`) {
		t.Error("missing raw output content")
	}

	// Should contain the learning hint
	if !strings.Contains(text, "harbor_learn_schema") {
		t.Error("missing harbor_learn_schema hint")
	}
	if !strings.Contains(text, "list_files") {
		t.Error("hint missing tool name")
	}
}

func TestLearnHandlerAndCompression(t *testing.T) {
	upstreamClient, _ := setupMockUpstream(t)
	defer upstreamClient.Close()

	schemaStore, err := schema.NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	memStore, err := memory.NewStoreAt(filepath.Join(t.TempDir(), "memory"))
	if err != nil {
		t.Fatal(err)
	}

	// Step 1: Agent calls harbor_learn_schema to teach Harbor
	learnHandler := makeLearnHandler(schemaStore)

	learnReq := mcp.CallToolRequest{}
	learnReq.Params.Name = "harbor_learn_schema"
	learnReq.Params.Arguments = map[string]interface{}{
		"tool_name":        "list_files",
		"summary_fields":   []interface{}{"name", "size", "type"},
		"summary_template": "{name} ({type}, {size} bytes)",
	}

	learnResult, err := learnHandler(context.Background(), learnReq)
	if err != nil {
		t.Fatalf("learn handler: %v", err)
	}

	learnText := extractTextContent(learnResult)
	if !strings.Contains(learnText, "Schema learned") {
		t.Errorf("unexpected learn result: %s", learnText)
	}

	// Verify schema was persisted
	if !schemaStore.Has("list_files") {
		t.Fatal("schema not saved after learn")
	}

	ls := schemaStore.Get("list_files")
	if ls.LLMModel != "agent-taught" {
		t.Errorf("LLMModel = %q, want 'agent-taught'", ls.LLMModel)
	}

	// Step 2: Next proxy call should compress
	h := &proxyHandler{
		upstream:    upstreamClient,
		toolName:    "list_files",
		schemaStore: schemaStore,
		memStore:    memStore,
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "list_files"
	req.Params.Arguments = map[string]interface{}{"path": "/tmp"}

	result, err := h.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("handle after learn: %v", err)
	}

	text := extractTextContent(result)

	// Should NOT contain the hint anymore
	if strings.Contains(text, "harbor_learn_schema") {
		t.Error("compressed result still contains learning hint")
	}

	// Compressed output should be shorter than raw
	if len(text) >= len(sampleFiles) {
		t.Errorf("compressed (%d bytes) should be shorter than raw (%d bytes): %s",
			len(text), len(sampleFiles), text)
	}

	// Verify only summary fields present
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("compressed not valid JSON: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("items len = %d, want 3", len(items))
	}

	for i, item := range items {
		if _, ok := item["permissions"]; ok {
			t.Errorf("item[%d] should not have 'permissions'", i)
		}
		if _, ok := item["modified"]; ok {
			t.Errorf("item[%d] should not have 'modified'", i)
		}
		if _, ok := item["name"]; !ok {
			t.Errorf("item[%d] missing 'name'", i)
		}
	}
}

func TestLearnHandlerValidation(t *testing.T) {
	schemaStore, err := schema.NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	handler := makeLearnHandler(schemaStore)

	tests := []struct {
		name string
		args map[string]interface{}
		want string
	}{
		{
			name: "missing tool_name",
			args: map[string]interface{}{
				"summary_fields":   []interface{}{"a"},
				"summary_template": "{a}",
			},
			want: "tool_name is required",
		},
		{
			name: "missing summary_fields",
			args: map[string]interface{}{
				"tool_name":        "test",
				"summary_template": "{a}",
			},
			want: "summary_fields is required",
		},
		{
			name: "empty summary_fields",
			args: map[string]interface{}{
				"tool_name":        "test",
				"summary_fields":   []interface{}{},
				"summary_template": "{a}",
			},
			want: "summary_fields must not be empty",
		},
		{
			name: "missing summary_template",
			args: map[string]interface{}{
				"tool_name":      "test",
				"summary_fields": []interface{}{"a"},
			},
			want: "summary_template is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "harbor_learn_schema"
			req.Params.Arguments = tt.args

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if !result.IsError {
				t.Error("expected error result")
			}
			text := extractTextContent(result)
			if !strings.Contains(text, tt.want) {
				t.Errorf("error = %q, want to contain %q", text, tt.want)
			}
		})
	}
}

func TestProxyHandlerMemoryHit(t *testing.T) {
	upstreamClient, _ := setupMockUpstream(t)
	defer upstreamClient.Close()

	schemaStore, err := schema.NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	memStore, err := memory.NewStoreAt(filepath.Join(t.TempDir(), "memory"))
	if err != nil {
		t.Fatal(err)
	}

	h := &proxyHandler{
		upstream:    upstreamClient,
		toolName:    "list_files",
		schemaStore: schemaStore,
		memStore:    memStore,
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "list_files"
	req.Params.Arguments = map[string]interface{}{"path": "/tmp"}

	// First call: goes to upstream, saves to memory
	_, err = h.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("first handle: %v", err)
	}

	// Second call: should hit memory
	result2, err := h.handle(context.Background(), req)
	if err != nil {
		t.Fatalf("second handle: %v", err)
	}

	text2 := extractTextContent(result2)
	if text2 == "" {
		t.Error("memory hit returned empty text")
	}

	entry := memStore.Latest(proxyConnectorName, "list_files", map[string]string{"path": "/tmp"})
	if entry == nil {
		t.Fatal("no memory entry found")
	}

	if !memStore.IsFresh(entry) {
		t.Error("memory entry should be fresh")
	}
}

func TestCompressWithSchema(t *testing.T) {
	ls := &schema.LearnedSchema{
		ToolName:        "test_tool",
		SummaryFields:   []string{"name", "size"},
		SummaryTemplate: "{name}: {size} bytes",
	}

	input := `[{"name":"file.txt","size":1024,"type":"file","perms":"rw-"},{"name":"dir","size":4096,"type":"directory","perms":"rwx"}]`

	compressed, summary, err := compressWithSchema(input, ls)
	if err != nil {
		t.Fatalf("compressWithSchema: %v", err)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(compressed), &items); err != nil {
		t.Fatalf("compressed not valid JSON: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("items len = %d, want 2", len(items))
	}

	for i, item := range items {
		if _, ok := item["type"]; ok {
			t.Errorf("item[%d] should not have 'type' field", i)
		}
		if _, ok := item["perms"]; ok {
			t.Errorf("item[%d] should not have 'perms' field", i)
		}
		if _, ok := item["name"]; !ok {
			t.Errorf("item[%d] missing 'name' field", i)
		}
		if _, ok := item["size"]; !ok {
			t.Errorf("item[%d] missing 'size' field", i)
		}
	}

	if summary == "" {
		t.Error("summary is empty")
	}
	if !strings.Contains(summary, "file.txt") || !strings.Contains(summary, "dir") {
		t.Errorf("summary missing expected content: %s", summary)
	}
}

func TestCompressWithSchemaSingleObject(t *testing.T) {
	ls := &schema.LearnedSchema{
		ToolName:        "test_tool",
		SummaryFields:   []string{"name", "status"},
		SummaryTemplate: "{name}: {status}",
	}

	input := `{"name":"my-project","status":"active","id":42,"created":"2024-01-01"}`

	compressed, summary, err := compressWithSchema(input, ls)
	if err != nil {
		t.Fatalf("compressWithSchema: %v", err)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(compressed), &items); err != nil {
		t.Fatalf("compressed not valid JSON: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("items len = %d, want 1", len(items))
	}

	if _, ok := items[0]["id"]; ok {
		t.Error("should not have 'id' field")
	}

	if summary != "my-project: active" {
		t.Errorf("summary = %q, want %q", summary, "my-project: active")
	}
}
