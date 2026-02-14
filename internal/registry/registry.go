package registry

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/oseaitic/harbor/internal/connector"
)

// Install looks up a connector in the embedded catalog, checks prerequisites,
// downloads the bundle, and installs it to ~/.harbor/connectors/{name}.
func Install(name string) error {
	entry := LookupCatalog(name)
	if entry == nil {
		available := ListCatalog()
		ids := make([]string, len(available))
		for i, e := range available {
			ids[i] = e.ID
		}
		return fmt.Errorf("connector %q not found in catalog. Available: %v", name, ids)
	}

	// Check runtime prerequisite
	if entry.Runtime == "node" {
		if _, err := exec.LookPath("node"); err != nil {
			return fmt.Errorf("connector %q requires Node.js but 'node' was not found in PATH", name)
		}
	}

	// Download the bundle
	fmt.Printf("  downloading %s v%s ...\n", entry.Name, entry.Version)
	resp, err := http.Get(entry.DownloadURL)
	if err != nil {
		return fmt.Errorf("downloading connector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, entry.DownloadURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading download: %w", err)
	}

	return writeConnector(name, body)
}

// InstallFromLocal copies a local bundle file into the connectors directory.
// This is used for local development via `harbor install --from <path>`.
func InstallFromLocal(name, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading local bundle %q: %w", path, err)
	}

	return writeConnector(name, data)
}

// writeConnector writes bundle bytes to the connectors dir and makes it executable.
func writeConnector(name string, data []byte) error {
	dir := connector.ConnectorsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating connectors dir: %w", err)
	}

	destPath := filepath.Join(dir, name)
	if err := os.WriteFile(destPath, data, 0o755); err != nil {
		return fmt.Errorf("writing connector file: %w", err)
	}

	entry := LookupCatalog(name)
	if entry != nil {
		fmt.Printf("  version: %s\n", entry.Version)
		fmt.Printf("  schemas: %v\n", entry.Schemas)
	}

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
