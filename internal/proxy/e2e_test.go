package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/schema"
)

// Realistic GitHub API response data (similar to what GitHub MCP server returns)
var githubIssuesResponse = `[
	{
		"id": 2847291,
		"number": 142,
		"title": "Add dark mode support",
		"state": "open",
		"body": "We should add dark mode support to the application. This has been requested by multiple users in the community forum. The implementation should follow the system preference by default but allow manual override.\n\nAcceptance criteria:\n- Detect system preference\n- Toggle in settings\n- Persist preference\n- All components themed",
		"user": {"login": "alice", "id": 12345, "avatar_url": "https://avatars.githubusercontent.com/u/12345", "type": "User"},
		"labels": [{"id": 1, "name": "enhancement", "color": "a2eeef"}, {"id": 2, "name": "frontend", "color": "d4c5f9"}],
		"assignee": {"login": "bob", "id": 67890, "avatar_url": "https://avatars.githubusercontent.com/u/67890"},
		"milestone": {"id": 1, "title": "v2.0", "state": "open", "due_on": "2024-06-01T00:00:00Z"},
		"comments": 8,
		"created_at": "2024-01-15T10:30:00Z",
		"updated_at": "2024-02-20T14:22:00Z",
		"closed_at": null,
		"html_url": "https://github.com/org/repo/issues/142",
		"repository_url": "https://api.github.com/repos/org/repo",
		"events_url": "https://api.github.com/repos/org/repo/issues/142/events",
		"labels_url": "https://api.github.com/repos/org/repo/issues/142/labels{/name}",
		"comments_url": "https://api.github.com/repos/org/repo/issues/142/comments",
		"pull_request": null,
		"reactions": {"url": "https://api.github.com/repos/org/repo/issues/142/reactions", "total_count": 12, "+1": 10, "-1": 0, "laugh": 1, "hooray": 1, "confused": 0, "heart": 0, "rocket": 0, "eyes": 0}
	},
	{
		"id": 2847350,
		"number": 143,
		"title": "Fix memory leak in WebSocket handler",
		"state": "open",
		"body": "There's a memory leak when WebSocket connections are not properly closed. The goroutine count keeps increasing over time. Reproduced with 100 concurrent connections, memory grows from 50MB to 2GB over 24 hours.",
		"user": {"login": "charlie", "id": 11111, "avatar_url": "https://avatars.githubusercontent.com/u/11111", "type": "User"},
		"labels": [{"id": 3, "name": "bug", "color": "d73a4a"}, {"id": 4, "name": "critical", "color": "b60205"}],
		"assignee": null,
		"milestone": {"id": 2, "title": "v1.9.1", "state": "open", "due_on": "2024-02-28T00:00:00Z"},
		"comments": 3,
		"created_at": "2024-02-18T09:15:00Z",
		"updated_at": "2024-02-19T16:45:00Z",
		"closed_at": null,
		"html_url": "https://github.com/org/repo/issues/143",
		"repository_url": "https://api.github.com/repos/org/repo",
		"events_url": "https://api.github.com/repos/org/repo/issues/143/events",
		"labels_url": "https://api.github.com/repos/org/repo/issues/143/labels{/name}",
		"comments_url": "https://api.github.com/repos/org/repo/issues/143/comments",
		"pull_request": null,
		"reactions": {"url": "https://api.github.com/repos/org/repo/issues/143/reactions", "total_count": 5, "+1": 4, "-1": 0, "laugh": 0, "hooray": 0, "confused": 1, "heart": 0, "rocket": 0, "eyes": 0}
	},
	{
		"id": 2847400,
		"number": 144,
		"title": "Update dependencies to latest versions",
		"state": "closed",
		"body": "Routine dependency update. Several packages have security patches available.",
		"user": {"login": "dependabot[bot]", "id": 99999, "avatar_url": "https://avatars.githubusercontent.com/u/99999", "type": "Bot"},
		"labels": [{"id": 5, "name": "dependencies", "color": "0366d6"}],
		"assignee": {"login": "alice", "id": 12345, "avatar_url": "https://avatars.githubusercontent.com/u/12345"},
		"milestone": null,
		"comments": 1,
		"created_at": "2024-02-20T08:00:00Z",
		"updated_at": "2024-02-20T10:30:00Z",
		"closed_at": "2024-02-20T10:30:00Z",
		"html_url": "https://github.com/org/repo/issues/144",
		"repository_url": "https://api.github.com/repos/org/repo",
		"events_url": "https://api.github.com/repos/org/repo/issues/144/events",
		"labels_url": "https://api.github.com/repos/org/repo/issues/144/labels{/name}",
		"comments_url": "https://api.github.com/repos/org/repo/issues/144/comments",
		"pull_request": null,
		"reactions": {"url": "https://api.github.com/repos/org/repo/issues/144/reactions", "total_count": 0, "+1": 0, "-1": 0, "laugh": 0, "hooray": 0, "confused": 0, "heart": 0, "rocket": 0, "eyes": 0}
	}
]`

// Realistic filesystem listing (similar to what filesystem MCP server returns)
var filesystemResponse = `[
	{"name": "package.json", "size": 2048, "type": "file", "permissions": "rw-r--r--", "modified": "2024-02-15T10:30:00Z", "created": "2023-06-01T08:00:00Z", "owner": "user", "group": "staff", "inode": 12345678},
	{"name": "src", "size": 4096, "type": "directory", "permissions": "rwxr-xr-x", "modified": "2024-02-20T14:22:00Z", "created": "2023-06-01T08:00:00Z", "owner": "user", "group": "staff", "inode": 12345679},
	{"name": "node_modules", "size": 4096, "type": "directory", "permissions": "rwxr-xr-x", "modified": "2024-02-19T09:00:00Z", "created": "2023-06-01T08:00:00Z", "owner": "user", "group": "staff", "inode": 12345680},
	{"name": "README.md", "size": 8192, "type": "file", "permissions": "rw-r--r--", "modified": "2024-02-10T16:45:00Z", "created": "2023-06-01T08:00:00Z", "owner": "user", "group": "staff", "inode": 12345681},
	{"name": ".gitignore", "size": 256, "type": "file", "permissions": "rw-r--r--", "modified": "2023-06-01T08:00:00Z", "created": "2023-06-01T08:00:00Z", "owner": "user", "group": "staff", "inode": 12345682},
	{"name": "tsconfig.json", "size": 512, "type": "file", "permissions": "rw-r--r--", "modified": "2024-01-20T11:00:00Z", "created": "2023-06-01T08:00:00Z", "owner": "user", "group": "staff", "inode": 12345683},
	{"name": "dist", "size": 4096, "type": "directory", "permissions": "rwxr-xr-x", "modified": "2024-02-20T14:30:00Z", "created": "2023-08-15T12:00:00Z", "owner": "user", "group": "staff", "inode": 12345684},
	{"name": ".env", "size": 128, "type": "file", "permissions": "rw-------", "modified": "2024-02-01T09:00:00Z", "created": "2023-06-01T08:00:00Z", "owner": "user", "group": "staff", "inode": 12345685}
]`

func setupRealisticUpstream(t *testing.T, toolName, data string) *client.Client {
	t.Helper()

	s := server.NewMCPServer("test-upstream", "1.0.0")

	tool := mcp.NewToolWithRawSchema(
		toolName,
		fmt.Sprintf("Realistic %s tool", toolName),
		json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"state":{"type":"string"}}}`),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(data), nil
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
	initReq.Params.ClientInfo = mcp.Implementation{Name: "e2e-test", Version: "1.0.0"}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatal(err)
	}

	return c
}

// TestE2EFullProxyFlow tests the complete proxy flow:
// 1. First call → raw output + hint
// 2. Agent teaches schema
// 3. Second call → compressed output
// 4. Third call → memory hit
// 5. Measures token efficiency
func TestE2EFullProxyFlow(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		data           string
		summaryFields  []interface{}
		summaryTmpl    string
		expectedFields []string // fields that should be in compressed output
		removedFields  []string // fields that should NOT be in compressed output
	}{
		{
			name:           "github_issues",
			toolName:       "list_issues",
			data:           githubIssuesResponse,
			summaryFields:  []interface{}{"number", "title", "state", "user", "labels", "comments"},
			summaryTmpl:    "#{number} {title} [{state}] by {user} ({comments} comments)",
			expectedFields: []string{"number", "title", "state"},
			removedFields:  []string{"events_url", "labels_url", "comments_url", "repository_url", "reactions", "html_url"},
		},
		{
			name:           "filesystem_listing",
			toolName:       "list_directory",
			data:           filesystemResponse,
			summaryFields:  []interface{}{"name", "size", "type", "modified"},
			summaryTmpl:    "{name} ({type}, {size} bytes, modified {modified})",
			expectedFields: []string{"name", "size", "type", "modified"},
			removedFields:  []string{"permissions", "created", "owner", "group", "inode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamClient := setupRealisticUpstream(t, tt.toolName, tt.data)
			defer upstreamClient.Close()

			schemaStore, err := schema.NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
			if err != nil {
				t.Fatal(err)
			}
			memStore, err := memory.NewStoreAt(filepath.Join(t.TempDir(), "memory"))
			if err != nil {
				t.Fatal(err)
			}
			metricsLogger := &MetricsLogger{path: filepath.Join(t.TempDir(), "metrics", "proxy_compression.jsonl")}

			h := &proxyHandler{
				upstream:    upstreamClient,
				toolName:    tt.toolName,
				schemaStore: schemaStore,
				memStore:    memStore,
				metrics:     metricsLogger,
			}

			ctx := context.Background()
			req := mcp.CallToolRequest{}
			req.Params.Name = tt.toolName
			req.Params.Arguments = map[string]interface{}{"path": "/test"}

			// ── Step 1: First call → raw output + hint ──
			t.Log("Step 1: First call (no schema) → expect raw + hint")
			result1, err := h.handle(ctx, req)
			if err != nil {
				t.Fatalf("step 1: %v", err)
			}
			rawText := extractTextContent(result1)
			rawBytes := len(rawText)

			if !strings.Contains(rawText, "harbor_learn_schema") {
				t.Error("step 1: missing learning hint in raw output")
			}
			t.Logf("  Raw output: %d bytes", rawBytes)

			// ── Step 2: Agent teaches schema ──
			t.Log("Step 2: Agent calls harbor_learn_schema")
			learnHandler := makeLearnHandler(schemaStore, memStore)

			learnReq := mcp.CallToolRequest{}
			learnReq.Params.Name = "harbor_learn_schema"
			learnReq.Params.Arguments = map[string]interface{}{
				"tool_name":        tt.toolName,
				"summary_fields":   tt.summaryFields,
				"summary_template": tt.summaryTmpl,
			}

			learnResult, err := learnHandler(ctx, learnReq)
			if err != nil {
				t.Fatalf("step 2: %v", err)
			}
			learnText := extractTextContent(learnResult)
			if !strings.Contains(learnText, "Schema learned") {
				t.Errorf("step 2: unexpected learn result: %s", learnText)
			}
			t.Logf("  Schema learned: %s", learnText)

			// Clear memory so step 3 goes to upstream
			memStore2, _ := memory.NewStoreAt(filepath.Join(t.TempDir(), "memory2"))
			h.memStore = memStore2

			// ── Step 3: Second call → compressed output ──
			t.Log("Step 3: Second call (with schema) → expect compressed")
			result3, err := h.handle(ctx, req)
			if err != nil {
				t.Fatalf("step 3: %v", err)
			}
			compressedText := extractTextContent(result3)

			// Separate JSON from inventory hint
			compressedJSON := compressedText
			if idx := strings.Index(compressedText, "\n\n[Harbor:"); idx >= 0 {
				compressedJSON = compressedText[:idx]
			}
			compressedBytes := len(compressedJSON)

			// Should NOT contain learning hint (the first-time learn prompt)
			if strings.Contains(compressedText, "call harbor_learn_schema now") {
				t.Error("step 3: compressed result still contains learning hint")
			}

			// Should be significantly shorter
			compressionRatio := float64(compressedBytes) / float64(rawBytes)
			tokenReduction := (1 - compressionRatio) * 100

			t.Logf("  Compressed output: %d bytes", compressedBytes)
			t.Logf("  Compression ratio: %.2f", compressionRatio)
			t.Logf("  Token reduction: %.1f%%", tokenReduction)

			if compressionRatio >= 1.0 {
				t.Errorf("step 3: compressed (%d) should be smaller than raw (%d)", compressedBytes, rawBytes)
			}

			// Verify correct fields
			var items []map[string]interface{}
			if err := json.Unmarshal([]byte(compressedJSON), &items); err != nil {
				t.Fatalf("step 3: compressed not valid JSON: %v\nContent: %s", err, compressedJSON)
			}

			for i, item := range items {
				for _, expected := range tt.expectedFields {
					if _, ok := item[expected]; !ok {
						t.Errorf("step 3: item[%d] missing expected field %q", i, expected)
					}
				}
				for _, removed := range tt.removedFields {
					if _, ok := item[removed]; ok {
						t.Errorf("step 3: item[%d] should not have removed field %q", i, removed)
					}
				}
			}

			// ── Step 4: Third call → memory hit ──
			t.Log("Step 4: Third call → expect memory hit (no upstream call)")
			start := time.Now()
			result4, err := h.handle(ctx, req)
			if err != nil {
				t.Fatalf("step 4: %v", err)
			}
			memoryDuration := time.Since(start)
			memoryText := extractTextContent(result4)

			if memoryText == "" {
				t.Error("step 4: memory hit returned empty text")
			}
			t.Logf("  Memory hit: %d bytes in %v", len(memoryText), memoryDuration)

			// ── Summary ──
			t.Log("")
			t.Log("═══════════════════════════════════")
			t.Logf("  RESULTS for %s", tt.name)
			t.Log("═══════════════════════════════════")
			t.Logf("  Raw bytes:        %d", rawBytes)
			t.Logf("  Compressed bytes: %d", compressedBytes)
			t.Logf("  Compression:      %.1f%% reduction", tokenReduction)
			t.Logf("  Fields kept:      %d / removed noise fields", len(tt.expectedFields))
			t.Logf("  Memory latency:   %v", memoryDuration)
			t.Log("═══════════════════════════════════")
		})
	}
}

// TestE2EProvenanceTracking tests that Harbor adds provenance metadata
// that allows Agent to attribute data to specific sources.
func TestE2EProvenanceTracking(t *testing.T) {
	upstreamClient := setupRealisticUpstream(t, "list_issues", githubIssuesResponse)
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
		toolName:    "list_issues",
		schemaStore: schemaStore,
		memStore:    memStore,
		metrics:     &MetricsLogger{},
	}

	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "list_issues"
	req.Params.Arguments = map[string]interface{}{"state": "open"}

	// Make a call to populate memory
	_, err = h.handle(ctx, req)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	// Check memory has provenance
	entry := memStore.Latest(proxyConnectorName, "list_issues", map[string]string{"state": "open"})
	if entry == nil {
		t.Fatal("no memory entry found")
	}

	obj, err := memStore.Get(entry.ID)
	if err != nil {
		t.Fatalf("get object: %v", err)
	}

	// Verify provenance fields
	if obj.Connector != proxyConnectorName {
		t.Errorf("Connector = %q, want %q", obj.Connector, proxyConnectorName)
	}
	if obj.Resource != "list_issues" {
		t.Errorf("Resource = %q, want %q", obj.Resource, "list_issues")
	}
	if obj.Meta.Source != "harbor-proxy" {
		t.Errorf("Meta.Source = %q, want %q", obj.Meta.Source, "harbor-proxy")
	}
	if obj.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero (no timestamp)")
	}
	if obj.Params["state"] != "open" {
		t.Errorf("Params[state] = %q, want %q", obj.Params["state"], "open")
	}

	t.Logf("Provenance verified:")
	t.Logf("  Connector: %s", obj.Connector)
	t.Logf("  Resource:  %s", obj.Resource)
	t.Logf("  Source:    %s", obj.Meta.Source)
	t.Logf("  Timestamp: %s", obj.CreatedAt.Format(time.RFC3339))
	t.Logf("  Params:    %v", obj.Params)
	t.Logf("  TTL:       %d seconds", obj.TTLSeconds)
	t.Logf("  ID:        %s", obj.ID)
}

// TestE2EDataFreshness tests that Harbor correctly identifies stale data.
func TestE2EDataFreshness(t *testing.T) {
	upstreamClient := setupRealisticUpstream(t, "get_price", `{"symbol":"BTC","price":50000,"currency":"USD"}`)
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
		toolName:    "get_price",
		schemaStore: schemaStore,
		memStore:    memStore,
		metrics:     &MetricsLogger{},
	}

	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "get_price"
	req.Params.Arguments = map[string]interface{}{"symbol": "BTC"}

	// Make a call to populate memory
	_, err = h.handle(ctx, req)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	// Check freshness of recent data
	entry := memStore.Latest(proxyConnectorName, "get_price", map[string]string{"symbol": "BTC"})
	if entry == nil {
		t.Fatal("no memory entry found")
	}

	// Should be fresh (just created)
	if !memStore.IsFresh(entry) {
		t.Error("freshly created entry should be fresh")
	}

	age := time.Since(entry.CreatedAt)
	t.Logf("Data age: %v (fresh: %v, TTL: %ds)", age, memStore.IsFresh(entry), entry.TTLSeconds)

	// Verify the memory object has correct TTL
	obj, err := memStore.Get(entry.ID)
	if err != nil {
		t.Fatalf("get object: %v", err)
	}

	if obj.TTLSeconds != defaultTTLSeconds {
		t.Errorf("TTLSeconds = %d, want %d", obj.TTLSeconds, defaultTTLSeconds)
	}

	t.Logf("Freshness check passed:")
	t.Logf("  Created: %s", entry.CreatedAt.Format(time.RFC3339))
	t.Logf("  TTL: %d seconds", entry.TTLSeconds)
	t.Logf("  Expires: %s", entry.CreatedAt.Add(time.Duration(entry.TTLSeconds)*time.Second).Format(time.RFC3339))
	t.Logf("  Is fresh: %v", memStore.IsFresh(entry))
}

// TestE2EMultiSourceRecall tests that Harbor can track and recall data from
// multiple sources, enabling Agent to attribute data correctly.
func TestE2EMultiSourceRecall(t *testing.T) {
	// Source 1: GitHub issues
	ghClient := setupRealisticUpstream(t, "list_issues", githubIssuesResponse)
	defer ghClient.Close()

	// Source 2: Filesystem
	fsClient := setupRealisticUpstream(t, "list_directory", filesystemResponse)
	defer fsClient.Close()

	schemaStore, err := schema.NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}
	memStore, err := memory.NewStoreAt(filepath.Join(t.TempDir(), "memory"))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Call GitHub tool
	ghHandler := &proxyHandler{
		upstream:    ghClient,
		toolName:    "list_issues",
		schemaStore: schemaStore,
		memStore:    memStore,
		metrics:     &MetricsLogger{},
	}
	ghReq := mcp.CallToolRequest{}
	ghReq.Params.Name = "list_issues"
	ghReq.Params.Arguments = map[string]interface{}{"state": "open"}
	if _, err := ghHandler.handle(ctx, ghReq); err != nil {
		t.Fatalf("github handle: %v", err)
	}

	// Call filesystem tool
	fsHandler := &proxyHandler{
		upstream:    fsClient,
		toolName:    "list_directory",
		schemaStore: schemaStore,
		memStore:    memStore,
		metrics:     &MetricsLogger{},
	}
	fsReq := mcp.CallToolRequest{}
	fsReq.Params.Name = "list_directory"
	fsReq.Params.Arguments = map[string]interface{}{"path": "/project"}
	if _, err := fsHandler.handle(ctx, fsReq); err != nil {
		t.Fatalf("filesystem handle: %v", err)
	}

	// Now check recall can find both sources
	ghEntry := memStore.Latest(proxyConnectorName, "list_issues", map[string]string{"state": "open"})
	fsEntry := memStore.Latest(proxyConnectorName, "list_directory", map[string]string{"path": "/project"})

	if ghEntry == nil {
		t.Fatal("GitHub memory entry not found")
	}
	if fsEntry == nil {
		t.Fatal("Filesystem memory entry not found")
	}

	// Verify they are distinct entries with distinct resources
	if ghEntry.ID == fsEntry.ID {
		t.Error("GitHub and Filesystem entries should have different IDs")
	}
	if ghEntry.Resource == fsEntry.Resource {
		t.Error("Entries should have different Resource names")
	}

	t.Logf("Multi-source recall verified:")
	t.Logf("  Source 1: %s/%s (ID: %s)", ghEntry.Connector, ghEntry.Resource, ghEntry.ID[:8])
	t.Logf("  Source 2: %s/%s (ID: %s)", fsEntry.Connector, fsEntry.Resource, fsEntry.ID[:8])

	// Verify each can be retrieved independently
	ghObj, err := memStore.Get(ghEntry.ID)
	if err != nil {
		t.Fatalf("get github object: %v", err)
	}
	fsObj, err := memStore.Get(fsEntry.ID)
	if err != nil {
		t.Fatalf("get filesystem object: %v", err)
	}

	// Raw data should be different
	if string(ghObj.Layers.Raw) == string(fsObj.Layers.Raw) {
		t.Error("Raw data from different sources should be different")
	}

	t.Logf("  GitHub raw: %d bytes", len(ghObj.Layers.Raw))
	t.Logf("  Filesystem raw: %d bytes", len(fsObj.Layers.Raw))
}

// TestE2EFieldInventory tests that compressed output includes a field
// inventory hint listing omitted fields.
func TestE2EFieldInventory(t *testing.T) {
	upstreamClient := setupRealisticUpstream(t, "list_issues", githubIssuesResponse)
	defer upstreamClient.Close()

	schemaStore, err := schema.NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}
	memStore, err := memory.NewStoreAt(filepath.Join(t.TempDir(), "memory"))
	if err != nil {
		t.Fatal(err)
	}

	// Teach a schema with only 3 fields
	learnHandler := makeLearnHandler(schemaStore, memStore)
	learnReq := mcp.CallToolRequest{}
	learnReq.Params.Name = "harbor_learn_schema"
	learnReq.Params.Arguments = map[string]interface{}{
		"tool_name":        "list_issues",
		"summary_fields":   []interface{}{"number", "title", "state"},
		"summary_template": "#{number} {title} [{state}]",
	}
	if _, err := learnHandler(context.Background(), learnReq); err != nil {
		t.Fatal(err)
	}

	// Call the tool — should get compressed output with inventory
	h := &proxyHandler{
		upstream:    upstreamClient,
		toolName:    "list_issues",
		schemaStore: schemaStore,
		memStore:    memStore,
		metrics:     &MetricsLogger{path: filepath.Join(t.TempDir(), "metrics", "m.jsonl")},
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "list_issues"
	req.Params.Arguments = map[string]interface{}{"state": "open"}

	result, err := h.handle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := extractTextContent(result)

	// The JSON portion (before the inventory hint) should not contain noise fields
	jsonPart := text
	if idx := strings.Index(text, "\n\n[Harbor:"); idx >= 0 {
		jsonPart = text[:idx]
	}
	if strings.Contains(jsonPart, "events_url") {
		t.Error("compressed JSON should not contain events_url")
	}

	// Should contain field inventory
	if !strings.Contains(text, "[Harbor:") {
		t.Error("missing field inventory hint")
	}
	if !strings.Contains(text, "fields shown") {
		t.Error("inventory hint should mention fields shown")
	}

	// Should list some omitted fields
	if !strings.Contains(text, "Omitted:") {
		t.Error("inventory hint should list omitted fields")
	}

	// body and assignee should be in the omitted list
	if !strings.Contains(text, "body") {
		t.Error("omitted list should include 'body'")
	}

	t.Logf("Compressed output with inventory:\n%s", text)
}

// TestE2ERecompressOnLearn tests that harbor_learn_schema returns
// re-compressed cached data when raw data is available in memory.
func TestE2ERecompressOnLearn(t *testing.T) {
	upstreamClient := setupRealisticUpstream(t, "list_issues", githubIssuesResponse)
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
		toolName:    "list_issues",
		schemaStore: schemaStore,
		memStore:    memStore,
		metrics:     &MetricsLogger{path: filepath.Join(t.TempDir(), "metrics", "m.jsonl")},
	}

	ctx := context.Background()

	// Step 1: Call tool to populate memory with raw data (no schema yet)
	req := mcp.CallToolRequest{}
	req.Params.Name = "list_issues"
	req.Params.Arguments = map[string]interface{}{"state": "open"}
	_, err = h.handle(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	// Step 2: Teach schema — should return re-compressed cached data
	learnHandler := makeLearnHandler(schemaStore, memStore)
	learnReq := mcp.CallToolRequest{}
	learnReq.Params.Name = "harbor_learn_schema"
	learnReq.Params.Arguments = map[string]interface{}{
		"tool_name":        "list_issues",
		"summary_fields":   []interface{}{"number", "title", "state"},
		"summary_template": "#{number} {title} [{state}]",
	}
	learnResult, err := learnHandler(ctx, learnReq)
	if err != nil {
		t.Fatal(err)
	}
	learnText := extractTextContent(learnResult)

	// Should contain "Re-compressed"
	if !strings.Contains(learnText, "Re-compressed") {
		t.Errorf("learn response should contain re-compressed data, got: %s", learnText[:min(200, len(learnText))])
	}

	// Should contain actual compressed JSON
	if !strings.Contains(learnText, `"number"`) || !strings.Contains(learnText, `"title"`) {
		t.Error("re-compressed data should contain schema fields")
	}

	// Should NOT contain noise fields
	if strings.Contains(learnText, "events_url") || strings.Contains(learnText, "repository_url") {
		t.Error("re-compressed data should not contain noise fields")
	}

	t.Logf("Learn response with re-compressed data:\n%s", learnText[:min(500, len(learnText))])

	// Step 3: Teach again with MORE fields — re-compressed should include the new field
	learnReq2 := mcp.CallToolRequest{}
	learnReq2.Params.Name = "harbor_learn_schema"
	learnReq2.Params.Arguments = map[string]interface{}{
		"tool_name":        "list_issues",
		"summary_fields":   []interface{}{"number", "title", "state", "user", "assignee"},
		"summary_template": "#{number} {title} [{state}] by {user} assigned to {assignee}",
	}
	learnResult2, err := learnHandler(ctx, learnReq2)
	if err != nil {
		t.Fatal(err)
	}
	learnText2 := extractTextContent(learnResult2)

	// Should now include the user and assignee fields
	if !strings.Contains(learnText2, "alice") && !strings.Contains(learnText2, "charlie") && !strings.Contains(learnText2, "bob") {
		t.Error("v2 re-compressed data should include flattened user names")
	}

	t.Logf("Learn v2 response:\n%s", learnText2[:min(500, len(learnText2))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
