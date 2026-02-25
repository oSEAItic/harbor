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

		inventory := buildFieldInventory(rawText, ls.SummaryFields)
		return mcp.NewToolResultText(compressed + inventory), nil
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

	// Build a hint with auto-suggested fields if the output is JSON
	hint := buildLearnHint(h.toolName, rawText)

	return mcp.NewToolResultText(rawText + hint), nil
}

// makeLearnHandler returns an MCP handler for the harbor_learn_schema tool.
// The agent calls this to teach Harbor how to compress a specific tool's output.
// If memStore is non-nil and cached raw data exists, the response includes
// re-compressed data so the agent doesn't need an extra tool call.
func makeLearnHandler(store *schema.Store, memStore *memory.Store) server.ToolHandlerFunc {
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

		// Try to re-compress cached raw data with the new schema
		recompressed := recompressCached(memStore, toolName, ls)
		if recompressed != "" {
			fmt.Fprintf(os.Stderr, "harbor-proxy: re-compressed cached data for %q (%d bytes)\n", toolName, len(recompressed))
			return mcp.NewToolResultText(fmt.Sprintf(
				"Schema learned for %q (version %d, fields: %v). Re-compressed cached data:\n%s",
				toolName, version, fields, recompressed,
			)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Schema learned for %q (version %d). Future calls will be compressed to fields: %v",
			toolName, version, fields,
		)), nil
	}
}

// recompressCached finds the most recent cached raw data for a tool and
// re-compresses it with the given schema. Returns empty string if no cache.
func recompressCached(memStore *memory.Store, toolName string, ls *schema.LearnedSchema) string {
	if memStore == nil {
		return ""
	}

	// Find the most recent entry for this tool regardless of params
	entries := memStore.Query(memory.QueryOptions{
		Connector: proxyConnectorName,
		Resource:  toolName,
		Limit:     1,
	})
	if len(entries) == 0 {
		return ""
	}

	obj, err := memStore.Get(entries[0].ID)
	if err != nil || len(obj.Layers.Raw) == 0 {
		return ""
	}

	compressed, summary, _, err := compressWithSchema(string(obj.Layers.Raw), ls)
	if err != nil {
		return ""
	}

	// Update the cached object with new compression
	obj.Layers.Compact = json.RawMessage(compressed)
	obj.Layers.Summary = summary
	if _, err := memStore.Save(obj); err != nil {
		fmt.Fprintf(os.Stderr, "harbor-proxy: failed to update cache: %v\n", err)
	}

	return compressed
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
// Nested objects are flattened to a display value (e.g., user → user.login).
func filterFields(item map[string]interface{}, fields []string) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		v, ok := item[f]
		if !ok {
			continue
		}
		// Flatten nested objects to their display value
		if nested, isMap := v.(map[string]interface{}); isMap {
			result[f] = flattenNestedObject(nested)
		} else {
			result[f] = v
		}
	}
	return result
}

// flattenNestedObject extracts a display value from a nested object.
// Tries common display fields (login, name, title, label, id) in priority order.
func flattenNestedObject(obj map[string]interface{}) interface{} {
	for _, key := range []string{"login", "name", "title", "label", "display_name", "full_name", "email", "slug"} {
		if v, ok := obj[key]; ok {
			if s, isStr := v.(string); isStr && s != "" {
				return s
			}
		}
	}
	// Fallback: if there's an "id" field, use it
	if id, ok := obj["id"]; ok {
		return fmt.Sprintf("%v", id)
	}
	// Last resort: return JSON
	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("%v", obj)
	}
	return string(b)
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
			result = strings.ReplaceAll(result, placeholder, formatValue(item[k]))
		}
	}
	return result
}

// formatValue converts a value to a display string.
// Nested maps are flattened; primitives use fmt.Sprintf.
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		flat := flattenNestedObject(val)
		return fmt.Sprintf("%v", flat)
	case []interface{}:
		if len(val) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, formatValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildLearnHint generates a directive hint with auto-suggested fields when
// the raw output is JSON. If the output is not JSON, returns a generic hint.
func buildLearnHint(toolName, rawText string) string {
	trimmed := strings.TrimSpace(rawText)

	// Try to extract field names from JSON to suggest concrete fields
	fields := detectTopFields(trimmed)
	if len(fields) > 0 {
		// Pick up to 5 fields, build a suggested call
		if len(fields) > 5 {
			fields = fields[:5]
		}
		fieldsJSON, _ := json.Marshal(fields)
		tmplParts := make([]string, len(fields))
		for i, f := range fields {
			tmplParts[i] = "{" + f + "}"
		}
		tmpl := strings.Join(tmplParts, " | ")

		return fmt.Sprintf(
			"\n\n[Harbor: call harbor_learn_schema now to compress future responses. "+
				"Suggested: tool_name=%q, summary_fields=%s, summary_template=%q. "+
				"Adjust the fields and template based on what matters for your task.]",
			toolName, string(fieldsJSON), tmpl,
		)
	}

	// Non-JSON output — generic hint
	return fmt.Sprintf(
		"\n\n[Harbor: call harbor_learn_schema with tool_name=%q to compress future responses. "+
			"Pick the most important field names as summary_fields and write a summary_template with {field} placeholders.]",
		toolName,
	)
}

// buildFieldInventory returns a hint listing fields that were omitted during
// compression. This lets the agent know what additional data is available
// so it can update the schema if needed, rather than giving confidently
// wrong answers due to missing fields.
func buildFieldInventory(rawText string, schemaFields []string) string {
	allFields := detectAllFields(rawText)
	if len(allFields) == 0 {
		return ""
	}

	included := make(map[string]bool, len(schemaFields))
	for _, f := range schemaFields {
		included[f] = true
	}

	var omitted []string
	for _, f := range allFields {
		if !included[f] {
			omitted = append(omitted, f)
		}
	}
	if len(omitted) == 0 {
		return ""
	}

	shown := omitted
	more := ""
	if len(shown) > 10 {
		shown = shown[:10]
		more = fmt.Sprintf(", +%d more", len(omitted)-10)
	}

	return fmt.Sprintf(
		"\n\n[Harbor: %d/%d fields shown. Omitted: %s%s. "+
			"To include omitted fields, call harbor_learn_schema with updated summary_fields.]",
		len(schemaFields), len(allFields), strings.Join(shown, ", "), more,
	)
}

// detectAllFields extracts all top-level field names from a JSON object or
// the first item of a JSON array. Returns sorted field names.
func detectAllFields(text string) []string {
	trimmed := strings.TrimSpace(text)
	var item map[string]interface{}

	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil || len(arr) == 0 {
			return nil
		}
		item = arr[0]
	} else if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &item); err != nil {
			return nil
		}
	} else {
		return nil
	}

	fields := make([]string, 0, len(item))
	for k := range item {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	return fields
}

// detectTopFields extracts top-level field names from a JSON object or the
// first item of a JSON array. It returns short, likely-useful field names
// (skipping URLs and deeply nested objects), sorted alphabetically.
func detectTopFields(text string) []string {
	var item map[string]interface{}

	if strings.HasPrefix(text, "[") {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(text), &arr); err != nil || len(arr) == 0 {
			return nil
		}
		item = arr[0]
	} else if strings.HasPrefix(text, "{") {
		if err := json.Unmarshal([]byte(text), &item); err != nil {
			return nil
		}
	} else {
		return nil
	}

	var fields []string
	for k, v := range item {
		// Skip fields that look like noise (URLs, internal IDs, nested objects)
		if strings.HasSuffix(k, "_url") || strings.HasSuffix(k, "_urls") {
			continue
		}
		// Skip deeply nested objects (likely metadata)
		switch v.(type) {
		case map[string]interface{}:
			// Keep simple nested objects like "user", "labels" but skip if key suggests noise
			if strings.Contains(k, "url") || strings.Contains(k, "reaction") {
				continue
			}
		}
		fields = append(fields, k)
	}

	sort.Strings(fields)
	return fields
}
