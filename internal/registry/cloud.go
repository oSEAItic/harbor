package registry

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/connector"
)

// CloudConnectorInfo holds metadata for a connector in the user's cloud registry.
type CloudConnectorInfo struct {
	Name      string    `json:"name"`
	SHA256    string    `json:"sha256"`
	Size      int       `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PublishToCloud reads the installed connector bundle, extracts its schema via
// --describe, and uploads both to the harbor-cloud connector registry.
// Returns the server-confirmed sha256 and bundle size.
func PublishToCloud(name string, cfg *cloudauth.Config) (sha256 string, size int, err error) {
	binPath := connector.ConnectorPath(name)
	if _, statErr := os.Stat(binPath); os.IsNotExist(statErr) {
		return "", 0, fmt.Errorf("connector %q is not installed (expected at %s)\nRun 'harbor build %s --install' first", name, binPath, name)
	}

	// Read the bundle binary
	bundle, err := os.ReadFile(binPath)
	if err != nil {
		return "", 0, fmt.Errorf("reading connector bundle: %w", err)
	}

	// Extract schema by running --describe
	cmd := exec.Command("node", binPath, "--describe")
	schemaBytes, err := cmd.Output()
	if err != nil {
		return "", 0, fmt.Errorf("extracting connector schema (node %s --describe): %w", binPath, err)
	}

	// Validate schema is a non-empty JSON array
	var schemaCheck []json.RawMessage
	if err := json.Unmarshal(schemaBytes, &schemaCheck); err != nil || len(schemaCheck) == 0 {
		return "", 0, fmt.Errorf("connector --describe did not return a valid schema array")
	}

	// Encode schema as base64 for the X-Schema header
	schemaB64 := base64.StdEncoding.EncodeToString(schemaBytes)

	// Upload to harbor-cloud
	url := cfg.Endpoint + "/api/connectors/" + name
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(bundle))
	if err != nil {
		return "", 0, fmt.Errorf("creating upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("X-Schema", schemaB64)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("uploading connector: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", 0, fmt.Errorf("cloud rejected upload (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response for sha256 and size
	var result struct {
		SHA256 string `json:"sha256"`
		Size   int    `json:"size"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil {
		return result.SHA256, result.Size, nil
	}

	// Fall back to header if body parsing fails
	return resp.Header.Get("X-SHA256"), len(bundle), nil
}

// InstallFromCloud downloads the named connector bundle from the user's cloud
// registry and installs it to the local connectors directory.
func InstallFromCloud(name string, cfg *cloudauth.Config) error {
	url := cfg.Endpoint + "/api/connectors/" + name + "/bundle"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating download request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading connector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("connector %q not found in your cloud registry", name)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloud error (%d): %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return fmt.Errorf("reading bundle: %w", err)
	}

	// Reuse the existing writeConnector function (same package)
	return writeConnector(name, data)
}

// ListCloudConnectors returns the list of connectors the user has published to cloud.
func ListCloudConnectors(cfg *cloudauth.Config) ([]CloudConnectorInfo, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/connectors", nil)
	if err != nil {
		return nil, fmt.Errorf("creating list request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing cloud connectors: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cloud error (%d): %s", resp.StatusCode, string(body))
	}

	var infos []CloudConnectorInfo
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return nil, fmt.Errorf("parsing cloud connector list: %w", err)
	}
	return infos, nil
}
