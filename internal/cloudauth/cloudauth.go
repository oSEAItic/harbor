// Package cloudauth manages Harbor Cloud credentials.
//
// Credentials are stored as a JSON file at ~/.harbor/cloud.json (0600).
// This is separate from connector auth (internal/auth) which stores
// per-connector API keys in the OS keychain.
package cloudauth

import (
	"encoding/json"
	"fmt"
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
