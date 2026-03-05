package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
	"github.com/zalando/go-keyring"
)

const serviceName = "harbor"

// KeyEntry records metadata about a stored credential.
type KeyEntry struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Store saves a credential for a connector in the OS keychain.
func Store(connector string, secret string) error {
	if connector == "" {
		return fmt.Errorf("connector name is required")
	}
	if secret == "" {
		return fmt.Errorf("secret is required")
	}
	if err := keyring.Set(serviceName, connector, secret); err != nil {
		return err
	}
	indexAdd(connector)
	return nil
}

// Retrieve loads a stored credential for a connector.
// Resolution order:
//  1. OS keychain (macOS/Linux/Windows)
//  2. Harbor Keychain (~/.harbor/keychain.json, AES-256-GCM encrypted with API key)
//     Written by 'harbor login' (auto-sync) or 'harbor auth sync'.
//
// If the credential exists in the keychain but not in the index,
// it is automatically backfilled (handles pre-index credentials).
func Retrieve(connector string) (string, error) {
	if connector == "" {
		return "", fmt.Errorf("connector name is required")
	}
	secret, err := keyring.Get(serviceName, connector)
	if err == nil {
		indexAdd(connector) // backfill if not already indexed
		return secret, nil
	}
	// OS keychain unavailable or credential not found — try Harbor Keychain.
	apiKey := loadAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("credential for %q not found (OS keychain failed, not logged in to Harbor Cloud)", connector)
	}
	return LoadFromKeychain(connector, apiKey)
}

// Delete removes a stored credential from OS keychain and Harbor Keychain.
func Delete(connector string) error {
	if err := keyring.Delete(serviceName, connector); err != nil {
		return err
	}
	indexRemove(connector)
	RemoveFromKeychain(connector)
	return nil
}

// List returns all known credential names (from the local index).
// It verifies each entry still exists in the keychain.
func List() []KeyEntry {
	entries := indexLoad()
	var valid []KeyEntry
	changed := false
	for _, e := range entries {
		if _, err := keyring.Get(serviceName, e.Name); err == nil {
			valid = append(valid, e)
		} else {
			changed = true // stale entry, will be pruned
		}
	}
	if changed {
		indexSave(valid)
	}
	return valid
}

// loadAPIKey reads the Harbor API key from ~/.harbor/cloud.json.
// Returns "" if not logged in or the file cannot be read.
// Avoids importing cloudauth to prevent import cycles.
func loadAPIKey() string {
	data, err := os.ReadFile(harborhome.Path("cloud.json"))
	if err != nil {
		return ""
	}
	var cfg struct {
		APIKey string `json:"api_key"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return ""
	}
	return cfg.APIKey
}

// --- index file helpers ---

func indexPath() string {
	return harborhome.Path("auth_keys.json")
}

func indexLoad() []KeyEntry {
	data, err := os.ReadFile(indexPath())
	if err != nil {
		return nil
	}
	var entries []KeyEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
}

func indexSave(entries []KeyEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(harborhome.RootDir(), 0o755)
	os.WriteFile(indexPath(), data, 0o600)
}

func indexAdd(name string) {
	entries := indexLoad()
	for _, e := range entries {
		if e.Name == name {
			return // already indexed
		}
	}
	entries = append(entries, KeyEntry{Name: name, CreatedAt: time.Now()})
	indexSave(entries)
}

func indexRemove(name string) {
	entries := indexLoad()
	var filtered []KeyEntry
	for _, e := range entries {
		if e.Name != name {
			filtered = append(filtered, e)
		}
	}
	indexSave(filtered)
}
