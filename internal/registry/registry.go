package registry

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/connector"
)

type InstallOptions struct {
	ExpectedSHA256 string
}

// Install looks up a connector in the embedded catalog, checks prerequisites,
// downloads the bundle, and installs it to HARBOR_HOME/connectors/{name}
// (or ~/.harbor/connectors/{name}).
func Install(name string, opts InstallOptions) error {
	if err := connector.ValidateConnectorName(name); err != nil {
		return err
	}

	entry := LookupCatalog(name)
	if entry == nil {
		// Not in catalog — try the user's cloud registry if logged in.
		cfg, err := cloudauth.Load()
		if err == nil {
			fmt.Printf("  %q not in catalog, checking your cloud registry...\n", name)
			if cloudErr := InstallFromCloud(name, cfg); cloudErr == nil {
				fmt.Printf("Connector %s installed from cloud.\n", name)
				return nil
			}
		}
		available := ListCatalog()
		ids := make([]string, len(available))
		for i, e := range available {
			ids[i] = e.ID
		}
		hint := ""
		if err != nil {
			hint = "\n(tip: run 'harbor login' to enable cloud connectors)"
		}
		return fmt.Errorf("connector %q not found in catalog. Available: %v%s", name, ids, hint)
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

	expected := opts.ExpectedSHA256
	if expected == "" {
		expected = entry.SHA256
	}

	actual, err := verifyAndPinChecksum(name, body, expected)
	if err != nil {
		return fmt.Errorf("verifying connector checksum: %w", err)
	}
	fmt.Printf("  checksum: %s\n", actual)

	return writeConnector(name, body)
}

// InstallFromLocal copies a local bundle file into the connectors directory.
// This is used for local development via `harbor install --from <path>`.
func InstallFromLocal(name, path string, opts InstallOptions) error {
	if err := connector.ValidateConnectorName(name); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading local bundle %q: %w", path, err)
	}

	actual, err := verifyAndPinChecksum(name, data, opts.ExpectedSHA256)
	if err != nil {
		return fmt.Errorf("verifying connector checksum: %w", err)
	}
	fmt.Printf("  checksum: %s\n", actual)

	return writeConnector(name, data)
}

// writeConnector writes bundle bytes to the connectors dir and makes it executable.
func writeConnector(name string, data []byte) error {
	dir := connector.ConnectorsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating connectors dir: %w", err)
	}

	destPath, err := connector.SafeConnectorPath(name)
	if err != nil {
		return err
	}
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
	path, err := connector.SafeConnectorPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("connector %q is not installed", name)
	}
	return os.Remove(path)
}
