package remember

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/memory"
)

// ToolDefinition returns the MCP tool definition for harbor_remember.
func ToolDefinition() mcp.Tool {
	return mcp.NewToolWithRawSchema(
		"harbor_remember",
		"Persist your analysis conclusions about a connector for future sessions. "+
			"Call this after completing meaningful analysis of a data source. "+
			"Your note will appear as 'context' at the start of every future session with this connector — "+
			"so you or any other agent won't need to re-derive the same insights. "+
			"Before calling, compose a comprehensive summary covering: "+
			"(1) what you analyzed and why, "+
			"(2) patterns or anomalies found, "+
			"(3) conclusions reached, "+
			"(4) recommendations made.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"connector": {
					"type": "string",
					"description": "The connector name (e.g. 'kuse-hive', 'coingecko')"
				},
				"note": {
					"type": "string",
					"description": "Your comprehensive analysis summary for this connector's data source"
				}
			},
			"required": ["connector", "note"]
		}`),
	)
}

// MakeHandler returns an MCP tool handler for harbor_remember.
func MakeHandler(store *memory.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if store == nil {
			return mcp.NewToolResultError("memory store not available"), nil
		}

		connector := req.GetString("connector", "")
		note := req.GetString("note", "")

		if connector == "" {
			return mcp.NewToolResultError("connector is required"), nil
		}
		if note == "" {
			return mcp.NewToolResultError("note is required"), nil
		}

		id, err := store.SaveNote(connector, note)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save note: %v", err)), nil
		}

		// Best-effort cloud push — sync note to cloud for cross-device recall.
		go func() {
			if cfg, err := cloudauth.Load(); err == nil {
				key := connector + "._context"
				_ = cloudauth.PushMemory(key, note, cfg)
			}
		}()

		return mcp.NewToolResultText(fmt.Sprintf(
			"Saved note for %q (%s). This will appear as context in future sessions with this connector.",
			connector, id,
		)), nil
	}
}
