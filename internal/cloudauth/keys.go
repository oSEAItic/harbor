package cloudauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KeyInfo describes an API key returned by the cloud.
type KeyInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	Policy    *KeyPolicy `json:"policy,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// KeyPolicy mirrors the server-side Policy, including AllowedScopes.
type KeyPolicy struct {
	Connectors    map[string][]string `json:"connectors,omitempty"`
	MaxVisibility string              `json:"max_visibility,omitempty"`
	AllowedScopes []string            `json:"allowed_scopes,omitempty"`
}

// ListAPIKeys returns all active API keys for the authenticated user.
func ListAPIKeys(cfg *Config) ([]KeyInfo, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", cfg.Endpoint, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var keys []KeyInfo
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return keys, nil
}

// CreateScopedAPIKey creates a new API key with the given name and optional policy.
// Returns the raw key (shown only once) and its ID.
func CreateScopedAPIKey(name string, policy *KeyPolicy, cfg *Config) (rawKey, id string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"name":   name,
		"policy": policy,
	})
	req, err := http.NewRequest(http.MethodPost, cfg.Endpoint+"/api/keys", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("connecting to %s: %w", cfg.Endpoint, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		var errResp struct{ Error string `json:"error"` }
		json.Unmarshal(raw, &errResp)
		if errResp.Error != "" {
			return "", "", fmt.Errorf("%s", errResp.Error)
		}
		return "", "", fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var out struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", "", fmt.Errorf("parsing response: %w", err)
	}
	return out.Key, out.ID, nil
}

// RevokeAPIKey revokes the API key with the given ID.
func RevokeAPIKey(keyID string, cfg *Config) error {
	req, err := http.NewRequest(http.MethodDelete, cfg.Endpoint+"/api/keys/"+keyID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", cfg.Endpoint, err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("server returned %d", resp.StatusCode)
}
