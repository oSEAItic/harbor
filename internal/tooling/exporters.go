package tooling

import "github.com/oseaitic/harbor/internal/protocol"

// FromOpenAISchemas converts OpenAI function-calling tool schemas into ToolIR.
func FromOpenAISchemas(schemas []protocol.ToolSchema) []ToolIR {
	out := make([]ToolIR, 0, len(schemas))
	for _, s := range schemas {
		out = append(out, ToolIR{
			Name:            s.Function.Name,
			Description:     s.Function.Description,
			Parameters:      s.Function.Parameters,
			SummaryFields:   append([]string(nil), s.Function.SummaryFields...),
			SummaryTemplate: s.Function.SummaryTemplate,
		})
	}
	return out
}

// ToOpenAISchemas converts ToolIR into OpenAI function-calling tool schemas.
func ToOpenAISchemas(ir []ToolIR) []protocol.ToolSchema {
	out := make([]protocol.ToolSchema, 0, len(ir))
	for _, t := range ir {
		out = append(out, protocol.ToolSchema{
			Type: "function",
			Function: protocol.ToolFunction{
				Name:            t.Name,
				Description:     t.Description,
				Parameters:      t.Parameters,
				SummaryFields:   append([]string(nil), t.SummaryFields...),
				SummaryTemplate: t.SummaryTemplate,
			},
		})
	}
	return out
}

// ToMCPSchemas converts ToolIR into JSON-serializable MCP tool definitions.
func ToMCPSchemas(ir []ToolIR) []MCPTool {
	out := make([]MCPTool, 0, len(ir))
	for _, t := range ir {
		out = append(out, MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	return out
}
