package cloudauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Register creates a new account using an invite code and returns the user ID.
func Register(endpoint, email, password, inviteCode string) (userID string, err error) {
	body, _ := json.Marshal(map[string]string{
		"email":       email,
		"password":    password,
		"invite_code": inviteCode,
	})
	resp, err := httpClient.Post(endpoint+"/api/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("contacting harbor cloud: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusCreated {
		var out struct {
			UserID string `json:"user_id"`
		}
		json.Unmarshal(raw, &out)
		return out.UserID, nil
	}
	var errResp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(raw, &errResp)
	if errResp.Error != "" {
		return "", fmt.Errorf("%s", errResp.Error)
	}
	return "", fmt.Errorf("registration failed (%d)", resp.StatusCode)
}

// LoginWithPassword authenticates with email/password and returns a session token
// and an API key (only present on first login when the server auto-creates one).
func LoginWithPassword(endpoint, email, password string) (sessionToken, apiKey string, err error) {
	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	resp, err := httpClient.Post(endpoint+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("contacting harbor cloud: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(raw, &errResp)
		if errResp.Error != "" {
			return "", "", fmt.Errorf("%s", errResp.Error)
		}
		return "", "", fmt.Errorf("login failed (%d)", resp.StatusCode)
	}

	var out struct {
		SessionToken string `json:"session_token"`
		APIKey       string `json:"api_key"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", "", fmt.Errorf("parsing login response: %w", err)
	}
	return out.SessionToken, out.APIKey, nil
}

// FetchEncKey calls GET /api/me and returns the stable per-user encryption key.
// Called during 'harbor login' to populate Config.EncKey.
func FetchEncKey(cfg *Config) (string, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/me", nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching enc_key: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var out struct {
		EncKey string `json:"enc_key"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.EncKey == "" {
		return "", fmt.Errorf("invalid enc_key response")
	}
	return out.EncKey, nil
}

// CreateAPIKey creates a new named API key using a session token.
func CreateAPIKey(endpoint, sessionToken, name string) (apiKey string, err error) {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, _ := http.NewRequest(http.MethodPost, endpoint+"/api/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("creating API key: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("creating API key failed (%d): %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("parsing key response: %w", err)
	}
	return out.Key, nil
}
