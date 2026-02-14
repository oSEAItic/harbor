package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/oseaitic/harbor/internal/connector"
)

const defaultRegistryURL = "https://raw.githubusercontent.com/oseaitic/harbor-registry/main/registry.json"

// ConnectorEntry describes a connector in the registry.
type ConnectorEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Binary   string   `json:"binary"`
	Checksum string   `json:"checksum"`
	Schemas  []string `json:"schemas"`
}

// RegistryManifest is the top-level registry file.
type RegistryManifest struct {
	Connectors []ConnectorEntry `json:"connectors"`
}

// ListAvailable fetches the registry and returns available connectors.
func ListAvailable() ([]ConnectorEntry, error) {
	resp, err := http.Get(defaultRegistryURL)
	if err != nil {
		return nil, fmt.Errorf("fetching registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	var manifest RegistryManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("parsing registry: %w", err)
	}

	return manifest.Connectors, nil
}

// Install downloads and installs a connector from the registry.
func Install(name string) error {
	entries, err := ListAvailable()
	if err != nil {
		return err
	}

	var entry *ConnectorEntry
	for _, e := range entries {
		if e.ID == name {
			entry = &e
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("connector %q not found in registry", name)
	}

	// Ensure connectors directory exists
	dir := connector.ConnectorsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating connectors dir: %w", err)
	}

	destPath := filepath.Join(dir, entry.ID)

	// TODO: Download binary from registry CDN, verify checksum
	// For now, create a placeholder
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating connector file: %w", err)
	}
	f.Close()

	if err := os.Chmod(destPath, 0o755); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	fmt.Printf("  version: %s\n", entry.Version)
	fmt.Printf("  schemas: %v\n", entry.Schemas)

	return nil
}

// Uninstall removes an installed connector.
func Uninstall(name string) error {
	path := connector.ConnectorPath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("connector %q is not installed", name)
	}
	return os.Remove(path)
}
