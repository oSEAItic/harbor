package cloudauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// CloudNote is a memory summary stored in Harbor Cloud.
type CloudNote struct {
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	Author    string    `json:"author,omitempty"`
	Topic     string    `json:"topic,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Connector string    `json:"connector,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeviceID returns a short identifier for the current machine (hostname).
func DeviceID() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "unknown"
}

// PushMemory upserts a memory summary to Harbor Cloud.
// key is "connector.topic.mem_id" (e.g. "kuse-hive.ws-reconnect.mem_abc123").
func PushMemory(key, content, author string, cfg *Config) error {
	return PushMemoryFull(key, content, author, "", "", "", cfg)
}

// PushMemoryFull upserts a memory with topic, session, and connector metadata.
func PushMemoryFull(key, content, author, topic, sessionID, connector string, cfg *Config) error {
	payload := map[string]string{"content": content}
	if author != "" {
		payload["author"] = author + " @ " + DeviceID()
	} else {
		payload["author"] = DeviceID()
	}
	if topic != "" {
		payload["topic"] = topic
	}
	if sessionID != "" {
		payload["session_id"] = sessionID
	}
	if connector != "" {
		payload["connector"] = connector
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut,
		cfg.Endpoint+"/api/memories/"+url.PathEscape(key),
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushing memory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// PullMemories downloads all memory summaries for the authenticated user.
func PullMemories(cfg *Config) ([]CloudNote, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/memories", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pulling memories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var notes []CloudNote
	if err := json.NewDecoder(resp.Body).Decode(&notes); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return notes, nil
}

// GraphEdge is a directed ref edge for cloud sync.
type GraphEdge struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// PushEdges upserts memory graph edges to Harbor Cloud.
func PushEdges(edges []GraphEdge, cfg *Config) error {
	if len(edges) == 0 {
		return nil
	}
	body, err := json.Marshal(edges)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut,
		cfg.Endpoint+"/api/memories/graph",
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushing edges: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// PullEdges downloads memory graph edges from Harbor Cloud.
func PullEdges(cfg *Config) ([]GraphEdge, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/memories/graph", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pulling edges: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var edges []GraphEdge
	if err := json.NewDecoder(resp.Body).Decode(&edges); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return edges, nil
}
