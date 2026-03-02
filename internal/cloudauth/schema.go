package cloudauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// CloudSchema is a learned compression schema stored in Harbor Cloud.
type CloudSchema struct {
	ToolName        string    `json:"tool_name"`
	SummaryFields   []string  `json:"summary_fields"`
	SummaryTemplate string    `json:"summary_template"`
	LearnedAt       time.Time `json:"learned_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PushSchema upserts a learned schema to Harbor Cloud.
// Called asynchronously from Store.Save; errors are silently dropped.
func PushSchema(toolName string, fields []string, template string, learnedAt time.Time, cfg *Config) error {
	body, err := json.Marshal(map[string]any{
		"summary_fields":   fields,
		"summary_template": template,
		"learned_at":       learnedAt,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut,
		cfg.Endpoint+"/api/schemas/"+url.PathEscape(toolName),
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushing schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// PullSchemas downloads all learned schemas for the authenticated user.
func PullSchemas(cfg *Config) ([]CloudSchema, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/schemas", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pulling schemas: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var schemas []CloudSchema
	if err := json.NewDecoder(resp.Body).Decode(&schemas); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return schemas, nil
}
