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
// If the credential exists in the keychain but not in the index,
// it is automatically backfilled (handles pre-index credentials).
func Retrieve(connector string) (string, error) {
	if connector == "" {
		return "", fmt.Errorf("connector name is required")
	}
	secret, err := keyring.Get(serviceName, connector)
	if err == nil {
		indexAdd(connector) // backfill if not already indexed
	}
	return secret, err
}

// Delete removes a stored credential.
func Delete(connector string) error {
	if err := keyring.Delete(serviceName, connector); err != nil {
		return err
	}
	indexRemove(connector)
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
	os.WriteFile(indexPath(), data, 0o644)
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
