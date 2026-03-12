package cloudauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ShareResult struct {
	Token     string `json:"token"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

type SharedMemory struct {
	Content   string `json:"content"`
	Author    string `json:"author"`
	Connector string `json:"connector"`
	ExpiresIn string `json:"expires_in"`
}

type ShareInfo struct {
	ID          string `json:"id"`
	MemoryKey   string `json:"memory_key"`
	Layer       string `json:"layer"`
	ExpiresAt   string `json:"expires_at"`
	CreatedAt   string `json:"created_at"`
	AccessCount int    `json:"access_count"`
}

// CreateShare calls POST /api/shares to create a temporary share link.
func CreateShare(memoryKey, layer, ttl string, cfg *Config) (*ShareResult, error) {
	payload := map[string]string{
		"memory_key": memoryKey,
		"ttl":        ttl,
		"layer":      layer,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, cfg.Endpoint+"/api/shares", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("creating share: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var result ShareResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

// ResolveShare calls GET /api/s/{token} to fetch a shared memory (public, no auth).
func ResolveShare(token string, endpoint string) (*SharedMemory, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint+"/api/s/"+token, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resolving share: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	// Response: {"data": "content", "meta": {"shared_by": "...", "memory_key": "connector.resource.mem_id", "expires_in": "..."}}
	var envelope struct {
		Data string `json:"data"`
		Meta struct {
			SharedBy  string `json:"shared_by"`
			MemoryKey string `json:"memory_key"`
			ExpiresIn string `json:"expires_in"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Extract connector from memory_key (format: connector.resource.mem_id)
	connector := ""
	if parts := splitKey(envelope.Meta.MemoryKey); len(parts) > 0 {
		connector = parts[0]
	}

	return &SharedMemory{
		Content:   envelope.Data,
		Author:    envelope.Meta.SharedBy,
		Connector: connector,
		ExpiresIn: envelope.Meta.ExpiresIn,
	}, nil
}

func splitKey(key string) []string {
	return strings.Split(key, ".")
}

// ListShares calls GET /api/shares to list active shares.
func ListShares(cfg *Config) ([]ShareInfo, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/shares", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing shares: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var shares []ShareInfo
	if err := json.NewDecoder(resp.Body).Decode(&shares); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return shares, nil
}

// RevokeShare calls DELETE /api/shares/{id} to revoke a share.
func RevokeShare(id string, cfg *Config) error {
	req, err := http.NewRequest(http.MethodDelete, cfg.Endpoint+"/api/shares/"+id, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("revoking share: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
