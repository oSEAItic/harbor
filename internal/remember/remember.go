package remember

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/memory"
)

// ToolDefinition returns the MCP tool definition for harbor_remember.
func ToolDefinition() mcp.Tool {
	return mcp.NewToolWithRawSchema(
		"harbor_remember",
		"Persist your analysis conclusions organized by topic for future sessions. "+
			"Notes are organized by topic (e.g. 'websocket-reconnect', 'billing-logic', 'market-trends') "+
			"and optionally scoped to a connector. "+
			"IMPORTANT: Use specific, descriptive topics — NOT the connector name. One insight per topic. "+
			"If you have multiple findings from one investigation, call this multiple times with different topics "+
			"and the SAME session_id to group them (e.g. session_id='kuse-debug-20260316'). "+
			"Harbor auto-links notes in the same session so future agents can pull the whole investigation. "+
			"Always pass your name in 'author'. "+
			"Before calling, summarize: (1) what you analyzed, (2) patterns found, (3) conclusions, (4) recommendations.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"topic": {
					"type": "string",
					"description": "Topic label for this note (e.g. 'websocket-reconnect', 'billing-logic', 'market-trends'). Use specific, descriptive names — NOT the connector name."
				},
				"note": {
					"type": "string",
					"description": "Your comprehensive analysis summary"
				},
				"connector": {
					"type": "string",
					"description": "Optional: scope this note to a specific connector (e.g. 'kuse-hive', 'coingecko'). Omit for global notes."
				},
				"author": {
					"type": "string",
					"description": "Your agent/model name (e.g. 'Claude Code', 'Gemini', 'Cursor'). If omitted, Harbor auto-detects from environment."
				},
				"refs": {
					"type": "array",
					"items": { "type": "string" },
					"description": "Memory IDs this note references or builds upon (e.g. ['mem_abc123']). Creates graph edges for dependency tracking."
				},
				"session_id": {
					"type": "string",
					"description": "Optional session ID to group related notes from the same investigation. Pass the same session_id across multiple harbor_remember calls to link them. If omitted, auto-generated from process context."
				}
			},
			"required": ["topic", "note"]
		}`),
	)
}

// MakeHandler returns an MCP tool handler for harbor_remember.
func MakeHandler(store *memory.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if store == nil {
			return mcp.NewToolResultError("memory store not available"), nil
		}

		topic := req.GetString("topic", "")
		note := req.GetString("note", "")
		connector := req.GetString("connector", "")
		author := req.GetString("author", "")
		sessionID := req.GetString("session_id", "")

		if topic == "" {
			return mcp.NewToolResultError("topic is required"), nil
		}
		if note == "" {
			return mcp.NewToolResultError("note is required"), nil
		}

		if author == "" {
			author = detectAgent()
		}

		// Parse optional refs (memory IDs this note references).
		refs := parseRefs(req)

		id, err := store.SaveNoteWithSession(connector, topic, note, author, sessionID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save note: %v", err)), nil
		}

		// Record reference edges in the knowledge graph.
		if len(refs) > 0 {
			memory.AddRefEdges(store.Dir(), id, refs)
		}

		// Best-effort cloud push — sync note to cloud for cross-device recall.
		go func() {
			if cfg, err := cloudauth.Load(); err == nil {
				key := connector + "." + topic + "." + id
				_ = cloudauth.PushMemory(key, note, author, cfg)
			}
		}()

		msg := fmt.Sprintf("Saved note topic=%q (%s).", topic, id)
		if connector != "" {
			msg += fmt.Sprintf(" Connector: %s.", connector)
		}
		if author != "" {
			msg += fmt.Sprintf(" Author: %s.", author)
		}
		if len(refs) > 0 {
			msg += fmt.Sprintf(" References: %s.", strings.Join(refs, ", "))
		}
		msg += " This will appear as context in future sessions."

		return mcp.NewToolResultText(msg), nil
	}
}

// parseRefs extracts the refs array from an MCP request.
// MCP passes JSON arrays as []interface{}, so we convert each element to string.
func parseRefs(req mcp.CallToolRequest) []string {
	args := req.GetArguments()
	if args == nil {
		return nil
	}
	raw, ok := args["refs"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var refs []string
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			refs = append(refs, s)
		}
	}
	return refs
}

// detectAgent attempts to identify the calling agent from environment variables.
func detectAgent() string {
	// Claude Code / Anthropic
	if os.Getenv("CLAUDE_CODE") != "" || strings.Contains(strings.ToLower(os.Getenv("TERM_PROGRAM")), "claude") {
		return "Claude Code"
	}
	// Cursor
	if strings.Contains(strings.ToLower(os.Getenv("TERM_PROGRAM")), "cursor") {
		return "Cursor"
	}
	// Gemini CLI
	if os.Getenv("GEMINI_API_KEY") != "" || strings.Contains(strings.ToLower(os.Getenv("TERM_PROGRAM")), "gemini") {
		return "Gemini"
	}
	// Generic MCP client identifier
	if client := os.Getenv("MCP_CLIENT_NAME"); client != "" {
		return client
	}
	return ""
}
