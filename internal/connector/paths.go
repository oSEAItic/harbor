package connector

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/oseaitic/harbor/internal/harborhome"
)

var connectorNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// ValidateConnectorName ensures connector identifiers are safe and canonical.
func ValidateConnectorName(name string) error {
	if !connectorNamePattern.MatchString(name) {
		return fmt.Errorf("invalid connector name %q: use [a-z0-9][a-z0-9_-]{0,63}", name)
	}
	return nil
}

// ConnectorsDir returns the path to the connectors directory.
func ConnectorsDir() string {
	return harborhome.Path("connectors")
}

// SafeConnectorPath returns a validated connector path.
func SafeConnectorPath(name string) (string, error) {
	if err := ValidateConnectorName(name); err != nil {
		return "", err
	}
	return filepath.Join(ConnectorsDir(), name), nil
}

// ConnectorPath returns the full path to a connector binary.
//
// Callers that handle user input should prefer SafeConnectorPath.
func ConnectorPath(name string) string {
	return filepath.Join(ConnectorsDir(), name)
}

// ListInstalled returns the names of all installed connectors.
func ListInstalled() ([]string, error) {
	dir := ConnectorsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
