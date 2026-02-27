package tooling

import "encoding/json"

// ToolIR is Harbor's canonical, framework-agnostic tool schema.
// Exporters transform this into OpenAI/MCP/Anthropic/etc. formats.
type ToolIR struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Parameters      json.RawMessage   `json:"parameters"`
	SummaryFields   []string          `json:"summary_fields,omitempty"`
	SummaryTemplate string            `json:"summary_template,omitempty"`
	FieldVisibility map[string]string `json:"field_visibility,omitempty"`
}

// MCPTool is a JSON-serializable MCP-compatible tool description.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}
