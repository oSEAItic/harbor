package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/schema"
)

const (
	proxyConnectorName = "proxy"
	defaultTTLSeconds  = 600 // 10 minutes
)

// proxyHandler handles tool calls by forwarding to the upstream MCP server,
// compressing output with learned schemas, and caching to memory.
type proxyHandler struct {
	upstream    *client.Client
	toolName    string
	schemaStore *schema.Store
	memStore    *memory.Store
}

// handle is the MCP tool handler function that proxies calls to the upstream server.
func (h *proxyHandler) handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := extractParams(req)

	// Check memory for cached result
	if h.memStore != nil {
		entry := h.memStore.Latest(proxyConnectorName, h.toolName, params)
		if h.memStore.IsFresh(entry) {
			obj, err := h.memStore.Get(entry.ID)
			if err == nil {
				fmt.Fprintf(os.Stderr, "harbor-proxy: memory hit for %s\n", h.toolName)
				text := obj.Layers.Summary
				if text == "" {
					text = string(obj.Layers.Compact)
				}
				if text == "" {
					text = string(obj.Layers.Raw)
				}
				return mcp.NewToolResultText(text), nil
			}
		}
	}

	// Forward call to upstream
	result, err := h.upstream.CallTool(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("upstream error: %v", err)), nil
	}

	if result.IsError {
		return result, nil
	}

	rawText := extractTextContent(result)
	if rawText == "" {
		return result, nil
	}

	// Compress if we have a learned schema
	ls := h.schemaStore.Get(h.toolName)
	if ls != nil {
		compressed, summary, err := compressWithSchema(rawText, ls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "harbor-proxy: compression error for %s: %v\n", h.toolName, err)
			h.saveToMemory(rawText, rawText, "", params)
			return result, nil
		}

		fmt.Fprintf(os.Stderr, "harbor-proxy: compressed %s (%d → %d bytes)\n",
			h.toolName, len(rawText), len(compressed))

		h.saveToMemory(rawText, compressed, summary, params)
		return mcp.NewToolResultText(compressed), nil
	}

	// No schema — return raw with a hint for the agent to teach us
	h.saveToMemory(rawText, rawText, "", params)

	hint := fmt.Sprintf(
		"\n\n[Harbor: No compression schema for %q. To enable compression, "+
			"call harbor_learn_schema with tool_name=%q, summary_fields (the most important field names), "+
			"and summary_template (a template with {field} placeholders).]",
		h.toolName, h.toolName,
	)

	return mcp.NewToolResultText(rawText + hint), nil
}

// makeLearnHandler returns an MCP handler for the harbor_learn_schema tool.
// The agent calls this to teach Harbor how to compress a specific tool's output.
func makeLearnHandler(store *schema.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toolName, err := req.RequireString("tool_name")
		if err != nil {
			return mcp.NewToolResultError("tool_name is required"), nil
		}

		// Parse summary_fields from the arguments
		args := req.GetArguments()
		fieldsRaw, ok := args["summary_fields"]
		if !ok {
			return mcp.NewToolResultError("summary_fields is required"), nil
		}

		var fields []string
		switch v := fieldsRaw.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					fields = append(fields, s)
				}
			}
		case []string:
			fields = v
		default:
			return mcp.NewToolResultError("summary_fields must be an array of strings"), nil
		}

		if len(fields) == 0 {
			return mcp.NewToolResultError("summary_fields must not be empty"), nil
		}

		tmpl := req.GetString("summary_template", "")
		if tmpl == "" {
			return mcp.NewToolResultError("summary_template is required"), nil
		}

		ls := &schema.LearnedSchema{
			ToolName:        toolName,
			SummaryFields:   fields,
			SummaryTemplate: tmpl,
			LearnedAt:       time.Now(),
			LLMModel:        "agent-taught",
			Version:         1,
		}

		if err := store.Save(ls); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save schema: %v", err)), nil
		}

		fmt.Fprintf(os.Stderr, "harbor-proxy: agent taught schema for %q (fields: %v)\n", toolName, fields)

		return mcp.NewToolResultText(fmt.Sprintf(
			"Schema learned for %q. Future calls will be compressed to fields: %v",
			toolName, fields,
		)), nil
	}
}

// saveToMemory persists a tool result to the 4-layer memory store.
func (h *proxyHandler) saveToMemory(raw, compact, summary string, params map[string]string) {
	if h.memStore == nil {
		return
	}

	obj := &memory.Object{
		Connector:  proxyConnectorName,
		Resource:   h.toolName,
		Params:     params,
		TTLSeconds: defaultTTLSeconds,
		Layers: memory.Layers{
			Raw:        json.RawMessage(raw),
			Normalized: json.RawMessage(raw),
			Compact:    json.RawMessage(compact),
			Summary:    summary,
		},
		Meta: memory.ObjectMeta{
			Source: "harbor-proxy",
		},
	}

	if _, err := h.memStore.Save(obj); err != nil {
		fmt.Fprintf(os.Stderr, "harbor-proxy: memory save error: %v\n", err)
	}
}

// extractTextContent extracts the concatenated text from a CallToolResult.
func extractTextContent(result *mcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// extractParams extracts string parameters from a CallToolRequest for memory keying.
func extractParams(req mcp.CallToolRequest) map[string]string {
	params := make(map[string]string)
	args := req.GetArguments()
	if args == nil {
		return params
	}

	for k, v := range args {
		switch val := v.(type) {
		case string:
			params[k] = val
		default:
			b, _ := json.Marshal(val)
			params[k] = string(b)
		}
	}
	return params
}

// compressWithSchema applies a learned schema to compress raw JSON output.
func compressWithSchema(rawJSON string, ls *schema.LearnedSchema) (compressed string, summary string, err error) {
	trimmed := strings.TrimSpace(rawJSON)

	var items []map[string]interface{}

	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
			return rawJSON, "", fmt.Errorf("parsing array: %w", err)
		}
	} else if strings.HasPrefix(trimmed, "{") {
		var single map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
			return rawJSON, "", fmt.Errorf("parsing object: %w", err)
		}
		items = []map[string]interface{}{single}
	} else {
		return rawJSON, "", nil
	}

	filtered := make([]map[string]interface{}, len(items))
	var summaries []string

	for i, item := range items {
		filtered[i] = filterFields(item, ls.SummaryFields)

		if ls.SummaryTemplate != "" {
			summaries = append(summaries, applyTemplate(ls.SummaryTemplate, item))
		}
	}

	compactBytes, err := json.Marshal(filtered)
	if err != nil {
		return rawJSON, "", fmt.Errorf("marshaling compact: %w", err)
	}

	summaryText := strings.Join(summaries, "; ")

	return string(compactBytes), summaryText, nil
}

// filterFields returns a new map containing only the specified fields.
func filterFields(item map[string]interface{}, fields []string) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		if v, ok := item[f]; ok {
			result[f] = v
		}
	}
	return result
}

// applyTemplate replaces {field} placeholders in the template with values from the item.
func applyTemplate(tmpl string, item map[string]interface{}) string {
	result := tmpl

	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		placeholder := "{" + k + "}"
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", item[k]))
		}
	}
	return result
}
