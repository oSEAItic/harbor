package recall

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/memory"
)

// ToolDefinition returns the MCP tool definition for harbor_recall.
func ToolDefinition() mcp.Tool {
	return mcp.NewToolWithRawSchema(
		"harbor_recall",
		"Search and retrieve data from Harbor's memory. "+
			"Call with no arguments to browse recent memories. "+
			"Use 'query' to search by keyword. "+
			"Use 'id' to retrieve the full content of a specific memory.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "string",
					"description": "Memory ID to retrieve full content. Omit to browse/search."
				},
				"query": {
					"type": "string",
					"description": "Keyword search across connector names, resources, params, and summaries"
				},
				"layer": {
					"type": "string",
					"enum": ["raw", "normalized", "compact", "summary"],
					"description": "Which data layer to return when retrieving by ID (default: compact)"
				},
				"connector": {
					"type": "string",
					"description": "Filter results to a specific connector"
				},
				"since": {
					"type": "string",
					"description": "Only show memories newer than this duration (e.g. '1h', '30m')"
				}
			}
		}`),
	)
}

// MakeHandler returns an MCP tool handler for harbor_recall.
func MakeHandler(store *memory.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if store == nil {
			return mcp.NewToolResultError("memory store not available"), nil
		}

		id := req.GetString("id", "")

		// Retrieve mode: return a specific memory object
		if id != "" {
			return handleRetrieve(store, id, req.GetString("layer", "compact"))
		}

		// Browse/search mode
		return handleBrowse(store, req)
	}
}

// handleRetrieve loads a memory object and returns the requested layer.
func handleRetrieve(store *memory.Store, id, layer string) (*mcp.CallToolResult, error) {
	obj, err := store.Get(id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("memory %q not found: %v", id, err)), nil
	}

	var content string
	switch layer {
	case "raw":
		content = string(obj.Layers.Raw)
	case "normalized":
		content = string(obj.Layers.Normalized)
	case "summary":
		content = obj.Layers.Summary
	default: // "compact" or unrecognized
		content = string(obj.Layers.Compact)
	}

	// Fall back through layers if the requested one is empty
	if content == "" {
		content = string(obj.Layers.Compact)
	}
	if content == "" {
		content = string(obj.Layers.Raw)
	}
	if content == "" {
		content = obj.Layers.Summary
	}
	if content == "" {
		return mcp.NewToolResultError(fmt.Sprintf("memory %q has no content", id)), nil
	}

	return mcp.NewToolResultText(content), nil
}

// handleBrowse searches memories and returns a compact listing.
func handleBrowse(store *memory.Store, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	connector := req.GetString("connector", "")
	since := req.GetString("since", "")

	opts := memory.QueryOptions{
		Connector: connector,
		Limit:     30,
	}
	if since != "" {
		if d, err := time.ParseDuration(since); err == nil {
			opts.Since = d
		}
	}

	results := store.Search(query, opts)

	if len(results) == 0 {
		msg := "No memories found"
		if query != "" {
			msg += fmt.Sprintf(" matching %q", query)
		}
		msg += "."
		return mcp.NewToolResultText(msg), nil
	}

	// Build compact listing
	var lines []string
	for _, r := range results {
		line := fmt.Sprintf("- %s | %s.%s | %s | %s",
			r.ID,
			r.Connector,
			r.Resource,
			formatAge(r.Age),
			freshLabel(r.Fresh),
		)

		if len(r.Params) > 0 {
			line += " | " + formatParams(r.Params)
		}

		if r.Summary != "" {
			line += "\n  " + r.Summary
		}

		lines = append(lines, line)
	}

	header := fmt.Sprintf("%d memories found", len(results))
	if query != "" {
		header += fmt.Sprintf(" matching %q", query)
	}
	header += ". Call harbor_recall with id=<ID> to retrieve full content.\n"

	return mcp.NewToolResultText(header + "\n" + strings.Join(lines, "\n")), nil
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm ago", h, m)
		}
		return fmt.Sprintf("%dh ago", h)
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func freshLabel(fresh bool) string {
	if fresh {
		return "fresh"
	}
	return "stale"
}

func formatParams(params map[string]string) string {
	parts := make([]string, 0, len(params))
	for k, v := range params {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}
