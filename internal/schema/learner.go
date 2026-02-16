package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	maxSampleChars     = 4000
	maxArrayItems      = 3
	schemaVersion      = 1
)

const systemPrompt = `You are a data compression analyst. Given a sample of JSON output from a tool, determine:
1. Which fields are most important for a summary (summary_fields)
2. A template string for generating a one-line natural language summary per item

Return ONLY valid JSON with this exact structure, no markdown, no explanation:
{"summary_fields": ["field1", "field2"], "summary_template": "{field1}: {field2}"}

Rules:
- summary_fields: Pick 3-6 most informative fields. Prefer names, identifiers, status, and key metrics.
- summary_template: Use {field_name} placeholders. Keep it concise (under 100 chars).
- If the data is a single object (not an array), still pick the most useful fields.
- Only reference fields that actually exist in the data.`

// LearnResult holds the parsed LLM response for schema learning.
type LearnResult struct {
	SummaryFields   []string `json:"summary_fields"`
	SummaryTemplate string   `json:"summary_template"`
}

// Learn sends a sample of tool output to the LLM and parses the compression schema.
func Learn(ctx context.Context, cfg *LLMConfig, toolName, sampleOutput string) (*LearnResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM config is nil")
	}

	truncated := truncateSample(sampleOutput, maxSampleChars)
	userMsg := buildPrompt(toolName, truncated)

	raw, err := chatCompletion(ctx, cfg, systemPrompt, userMsg)
	if err != nil {
		return nil, fmt.Errorf("LLM call: %w", err)
	}

	// Strip markdown code fences if present
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result LearnResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w (raw: %s)", err, raw)
	}

	if len(result.SummaryFields) == 0 {
		return nil, fmt.Errorf("LLM returned no summary_fields")
	}

	return &result, nil
}

// LearnAndSave learns a schema and persists it to the store.
func LearnAndSave(ctx context.Context, cfg *LLMConfig, store *Store, toolName, sampleOutput string) error {
	result, err := Learn(ctx, cfg, toolName, sampleOutput)
	if err != nil {
		return err
	}

	ls := &LearnedSchema{
		ToolName:        toolName,
		SummaryFields:   result.SummaryFields,
		SummaryTemplate: result.SummaryTemplate,
		LearnedAt:       time.Now(),
		LLMModel:        cfg.Model,
		Version:         schemaVersion,
	}

	return store.Save(ls)
}

// truncateSample truncates the raw output for sending to the LLM.
// If the output is a JSON array, it keeps only the first maxArrayItems items.
// Otherwise, it truncates to maxChars.
func truncateSample(raw string, maxChars int) string {
	trimmed := strings.TrimSpace(raw)

	// Try to parse as JSON array and truncate items
	if strings.HasPrefix(trimmed, "[") {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(trimmed), &arr) == nil && len(arr) > maxArrayItems {
			arr = arr[:maxArrayItems]
			if truncated, err := json.Marshal(arr); err == nil {
				return string(truncated)
			}
		}
	}

	// Fallback: character truncation
	if len(trimmed) > maxChars {
		return trimmed[:maxChars] + "..."
	}
	return trimmed
}

// buildPrompt creates the user message for the LLM.
func buildPrompt(toolName, sample string) string {
	return fmt.Sprintf("Tool name: %s\n\nSample output:\n%s", toolName, sample)
}
