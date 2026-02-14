package connector

import (
	"os"
	"path/filepath"
)

// ConnectorsDir returns the path to the connectors directory.
func ConnectorsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".harbor", "connectors")
	}
	return filepath.Join(home, ".harbor", "connectors")
}

// ConnectorPath returns the full path to a connector binary.
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
