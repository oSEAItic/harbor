package context

import (
	"encoding/json"
	"sort"

	"github.com/oseaitic/harbor/internal/protocol"
)

// CompileOptions controls how the context compiler transforms raw connector output.
type CompileOptions struct {
	Mode      string   // "overview" (default), "full", "expand"
	Budget    int      // target token count (0 = use defaults)
	Limit     int      // max items in data[] (0 = auto from budget)
	Fields    []string // manual field filter (overrides summary_fields)
	ExpandIdx int      // item index to expand (-1 = none)
	PreviewN  int      // number of preview items (default 3)
}

// DefaultOptions returns CompileOptions with sensible defaults.
func DefaultOptions() CompileOptions {
	return CompileOptions{
		Mode:      "overview",
		Budget:    0,
		Limit:     0,
		Fields:    nil,
		ExpandIdx: -1,
		PreviewN:  3,
	}
}

// estimateTokens approximates the token count for a JSON byte slice.
// Uses the ~4 chars per token heuristic for English JSON.
func estimateTokens(data []byte) int {
	return len(data) / 4
}

// Compile transforms a raw connector response into agent-ready context.
// summaryFields are the fields to keep in overview mode (from connector schema).
// schema is the meta.schema string used for template-based summaries.
func Compile(resp *protocol.Response, summaryFields []string, schema string, opts CompileOptions) *protocol.Response {
	if opts.Mode == "full" {
		return resp
	}

	// Parse data as array of objects
	var items []map[string]interface{}
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		// Data isn't an array of objects — return as-is
		return resp
	}

	if len(items) == 0 {
		return resp
	}

	// Build overview: field list from first item
	fields := extractFields(items[0])

	overview := &protocol.ContextOverview{
		TotalItems: len(items),
		Fields:     fields,
	}

	// Determine which fields to keep
	activeFields := summaryFields
	if len(opts.Fields) > 0 {
		activeFields = opts.Fields
	}

	switch opts.Mode {
	case "expand":
		return compileExpand(resp, items, overview, opts)
	default:
		// overview mode (also handles budget)
		return compileOverview(resp, items, overview, activeFields, schema, opts)
	}
}

// compileExpand returns full detail for a single item.
func compileExpand(resp *protocol.Response, items []map[string]interface{}, overview *protocol.ContextOverview, opts CompileOptions) *protocol.Response {
	idx := opts.ExpandIdx
	if idx < 0 || idx >= len(items) {
		idx = 0
	}

	expandedData, _ := json.Marshal([]map[string]interface{}{items[idx]})

	return &protocol.Response{
		Data:     expandedData,
		Meta:     resp.Meta,
		Errors:   resp.Errors,
		Overview: overview,
		Summary:  "", // no summary needed for single-item expand
	}
}

// compileOverview produces an overview with optional budget-based item fitting.
func compileOverview(resp *protocol.Response, items []map[string]interface{}, overview *protocol.ContextOverview, summaryFields []string, schema string, opts CompileOptions) *protocol.Response {
	previewN := opts.PreviewN
	if previewN <= 0 {
		previewN = 3
	}

	// Determine max items to include
	maxItems := previewN
	if opts.Limit > 0 {
		maxItems = opts.Limit
	}

	// Filter items to summary fields
	var previewItems []map[string]interface{}
	for i := 0; i < len(items) && i < maxItems; i++ {
		if len(summaryFields) > 0 {
			previewItems = append(previewItems, filterFields(items[i], summaryFields))
		} else {
			previewItems = append(previewItems, items[i])
		}
	}

	// If budget mode, try to fit more items
	if opts.Budget > 0 {
		previewItems = fitToBudget(items, summaryFields, opts.Budget, overview, schema, resp.Meta)
	}

	// Generate summary
	summary := GenerateSummary(items, summaryFields, schema)

	// Marshal preview data
	previewData, _ := json.Marshal(previewItems)

	// Calculate budget used
	result := &protocol.Response{
		Data:     previewData,
		Meta:     resp.Meta,
		Errors:   resp.Errors,
		Overview: overview,
		Summary:  summary,
	}

	// Add budget_used to meta if budget was specified
	if opts.Budget > 0 {
		resultBytes, _ := json.Marshal(result)
		_ = estimateTokens(resultBytes) // could be surfaced in meta later
	}

	return result
}

// fitToBudget adds items progressively until the token budget is reached.
func fitToBudget(items []map[string]interface{}, summaryFields []string, budget int, overview *protocol.ContextOverview, schema string, meta protocol.Meta) []map[string]interface{} {
	// Reserve tokens for overhead (overview, summary, meta)
	summary := GenerateSummary(items, summaryFields, schema)
	overviewBytes, _ := json.Marshal(overview)
	metaBytes, _ := json.Marshal(meta)
	overhead := estimateTokens([]byte(summary)) + estimateTokens(overviewBytes) + estimateTokens(metaBytes) + 50 // envelope overhead

	available := budget - overhead
	if available < 50 {
		available = 50
	}

	var result []map[string]interface{}
	totalSize := 0

	for _, item := range items {
		filtered := item
		if len(summaryFields) > 0 {
			filtered = filterFields(item, summaryFields)
		}

		itemBytes, _ := json.Marshal(filtered)
		itemTokens := estimateTokens(itemBytes)

		if totalSize+itemTokens > available && len(result) > 0 {
			break
		}

		result = append(result, filtered)
		totalSize += itemTokens
	}

	return result
}

// extractFields returns sorted field names from a map.
func extractFields(item map[string]interface{}) []string {
	fields := make([]string, 0, len(item))
	for k := range item {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	return fields
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
