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
	metrics     *MetricsLogger
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
				h.metrics.Log(CompressionMetric{
					ToolName:      h.toolName,
					FromMemory:    true,
					SchemaApplied: false,
				})
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
		compressed, summary, stats, err := compressWithSchema(rawText, ls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "harbor-proxy: compression error for %s: %v\n", h.toolName, err)
			h.saveToMemory(rawText, rawText, "", params)
			h.metrics.Log(CompressionMetric{
				ToolName:         h.toolName,
				SchemaApplied:    true,
				RawBytes:         len(rawText),
				CompactBytes:     len(rawText),
				CompressionRatio: 1.0,
			})
			return result, nil
		}

		if drift, reason := detectSchemaDrift(stats); drift {
			fmt.Fprintf(os.Stderr, "harbor-proxy: schema drift for %s: %s\n", h.toolName, reason)
			if _, rbErr := h.schemaStore.Rollback(h.toolName, 0); rbErr == nil {
				fmt.Fprintf(os.Stderr, "harbor-proxy: rolled back schema for %s to previous version\n", h.toolName)
			}

			h.saveToMemory(rawText, rawText, "", params)
			h.metrics.Log(CompressionMetric{
				ToolName:         h.toolName,
				SchemaApplied:    true,
				DriftDetected:    true,
				RawBytes:         stats.RawBytes,
				CompactBytes:     stats.RawBytes,
				CompressionRatio: 1.0,
				FieldHitRate:     stats.FieldHitRate(),
				Items:            stats.Items,
			})

			hint := fmt.Sprintf(
				"\n\n[Harbor: Schema drift detected for %q (%s). Returning raw output. "+
					"Please call harbor_learn_schema again to refresh compression.]",
				h.toolName, reason,
			)
			return mcp.NewToolResultText(rawText + hint), nil
		}

		fmt.Fprintf(os.Stderr, "harbor-proxy: compressed %s (%d → %d bytes)\n",
			h.toolName, len(rawText), len(compressed))

		h.saveToMemory(rawText, compressed, summary, params)
		h.metrics.Log(CompressionMetric{
			ToolName:         h.toolName,
			SchemaApplied:    true,
			RawBytes:         stats.RawBytes,
			CompactBytes:     stats.CompactBytes,
			CompressionRatio: stats.CompressionRatio(),
			FieldHitRate:     stats.FieldHitRate(),
			Items:            stats.Items,
		})
		return mcp.NewToolResultText(compressed), nil
	}

	// No schema — return raw with a hint for the agent to teach us
	h.saveToMemory(rawText, rawText, "", params)
	h.metrics.Log(CompressionMetric{
		ToolName:         h.toolName,
		SchemaApplied:    false,
		RawBytes:         len(rawText),
		CompactBytes:     len(rawText),
		CompressionRatio: 1.0,
	})

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
		if len(fields) > 16 {
			return mcp.NewToolResultError("summary_fields must contain at most 16 fields"), nil
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
		}

		if err := store.Save(ls); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save schema: %v", err)), nil
		}

		current := store.Get(toolName)
		version := 0
		if current != nil {
			version = current.Version
		}

		fmt.Fprintf(os.Stderr, "harbor-proxy: agent taught schema for %q (version=%d, fields=%v)\n", toolName, version, fields)

		return mcp.NewToolResultText(fmt.Sprintf(
			"Schema learned for %q (version %d). Future calls will be compressed to fields: %v",
			toolName, version, fields,
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
type compressionStats struct {
	Items           int
	FieldsRequested int
	FieldsMatched   int
	RawBytes        int
	CompactBytes    int
}

func (s compressionStats) FieldHitRate() float64 {
	den := s.Items * s.FieldsRequested
	if den <= 0 {
		return 1.0
	}
	return float64(s.FieldsMatched) / float64(den)
}

func (s compressionStats) CompressionRatio() float64 {
	if s.RawBytes <= 0 {
		return 1.0
	}
	return float64(s.CompactBytes) / float64(s.RawBytes)
}

func detectSchemaDrift(stats compressionStats) (bool, string) {
	if stats.Items == 0 || stats.FieldsRequested == 0 {
		return false, ""
	}
	hitRate := stats.FieldHitRate()
	if hitRate < 0.2 {
		return true, fmt.Sprintf("field hit rate %.2f below threshold", hitRate)
	}
	return false, ""
}

func compressWithSchema(rawJSON string, ls *schema.LearnedSchema) (compressed string, summary string, stats compressionStats, err error) {
	trimmed := strings.TrimSpace(rawJSON)
	stats.RawBytes = len(rawJSON)
	stats.FieldsRequested = len(ls.SummaryFields)

	var items []map[string]interface{}

	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
			return rawJSON, "", stats, fmt.Errorf("parsing array: %w", err)
		}
	} else if strings.HasPrefix(trimmed, "{") {
		var single map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
			return rawJSON, "", stats, fmt.Errorf("parsing object: %w", err)
		}
		items = []map[string]interface{}{single}
	} else {
		stats.CompactBytes = len(rawJSON)
		return rawJSON, "", stats, nil
	}
	stats.Items = len(items)

	filtered := make([]map[string]interface{}, len(items))
	var summaries []string

	for i, item := range items {
		matched := 0
		for _, f := range ls.SummaryFields {
			if _, ok := item[f]; ok {
				matched++
			}
		}
		stats.FieldsMatched += matched

		filtered[i] = filterFields(item, ls.SummaryFields)

		if ls.SummaryTemplate != "" {
			summaries = append(summaries, applyTemplate(ls.SummaryTemplate, item))
		}
	}

	compactBytes, err := json.Marshal(filtered)
	if err != nil {
		return rawJSON, "", stats, fmt.Errorf("marshaling compact: %w", err)
	}
	stats.CompactBytes = len(compactBytes)

	summaryText := strings.Join(summaries, "; ")

	return string(compactBytes), summaryText, stats, nil
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
