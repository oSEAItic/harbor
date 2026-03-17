// Package cloudauth manages Harbor Cloud credentials.
//
// Credentials are stored as a JSON file at ~/.harbor/cloud.json (0600).
// This is separate from connector auth (internal/auth) which stores
// per-connector API keys in the OS keychain.
package cloudauth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
)

// Config holds Harbor Cloud connection credentials.
type Config struct {
	Endpoint  string    `json:"endpoint"`
	APIKey    string    `json:"api_key"`
	EncKey    string    `json:"enc_key,omitempty"` // stable per-user encryption key (independent of API key rotation)
	CreatedAt time.Time `json:"created_at"`
}

// EncryptionKey returns the stable per-user key used for credential encryption.
// Falls back to APIKey for existing installs that predate enc_key support
// (those installs will get enc_key on next 'harbor login').
func (c *Config) EncryptionKey() string {
	if c.EncKey != "" {
		return c.EncKey
	}
	return c.APIKey
}

func configPath() string {
	return harborhome.Path("cloud.json")
}

// Save writes cloud credentials to disk.
func Save(cfg Config) error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("api key is required")
	}
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now().UTC()
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cloud config: %w", err)
	}

	if err := os.WriteFile(configPath(), data, 0o600); err != nil {
		return fmt.Errorf("writing cloud config: %w", err)
	}
	return nil
}

// Load reads cloud credentials from disk.
// Returns an error if the file does not exist or is invalid.
func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing cloud config: %w", err)
	}

	if cfg.Endpoint == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("cloud config is incomplete (missing endpoint or api_key)")
	}

	return &cfg, nil
}

// DefaultEndpoint is the production Harbor Cloud URL.
const DefaultEndpoint = "https://harbor-cloud.oseaitic.com"

// AutoProvision creates an anonymous cloud account using a device fingerprint.
// Returns the API key on success. The config is saved to disk automatically.
func AutoProvision() (string, error) {
	fp := deviceFingerprint()

	payload, _ := json.Marshal(map[string]string{"device_fingerprint": fp})
	resp, err := http.Post(DefaultEndpoint+"/api/auth/auto-provision", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("auto-provision request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("auto-provision returned %d", resp.StatusCode)
	}

	var result struct {
		APIKey string `json:"api_key"`
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	cfg := Config{
		Endpoint:  DefaultEndpoint,
		APIKey:    result.APIKey,
		CreatedAt: time.Now().UTC(),
	}
	if err := Save(cfg); err != nil {
		return "", fmt.Errorf("saving config: %w", err)
	}

	return result.APIKey, nil
}

// OptOutPath returns the path to the cloud opt-out marker file.
func OptOutPath() string {
	return harborhome.Path("cloud-optout")
}

// IsOptedOut returns true if the user has declined cloud sync.
func IsOptedOut() bool {
	_, err := os.Stat(OptOutPath())
	return err == nil
}

// OptOut creates the opt-out marker file so we never ask again.
func OptOut() error {
	return os.WriteFile(OptOutPath(), []byte("opted out of cloud sync\n"), 0o600)
}

// ClearOptOut removes the opt-out marker file.
func ClearOptOut() error {
	err := os.Remove(OptOutPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// deviceFingerprint generates a stable hash from hostname + OS info.
func deviceFingerprint() string {
	hostname, _ := os.Hostname()
	raw := fmt.Sprintf("harbor:%s:%s", hostname, os.Getenv("USER"))
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Delete removes cloud credentials from disk.
func Delete() error {
	err := os.Remove(configPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsLoggedIn returns true if valid cloud credentials exist.
func IsLoggedIn() bool {
	_, err := Load()
	return err == nil
}
