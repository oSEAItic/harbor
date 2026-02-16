package schema

import "time"

// LearnedSchema is the compression schema learned by the LLM for a specific tool.
// It tells the proxy which fields to keep and how to generate a natural language summary.
type LearnedSchema struct {
	ToolName        string    `json:"tool_name"`
	SummaryFields   []string  `json:"summary_fields"`
	SummaryTemplate string    `json:"summary_template"`
	LearnedAt       time.Time `json:"learned_at"`
	LLMModel        string    `json:"llm_model,omitempty"`
	Version         int       `json:"version"`
}
