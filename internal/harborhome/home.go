package harborhome

import (
	"os"
	"path/filepath"
	"strings"
)

// RootDir returns Harbor's data root directory.
//
// Resolution order:
// 1. HARBOR_HOME (if non-empty)
// 2. ~/.harbor
// 3. ./.harbor (fallback when home directory is unavailable)
func RootDir() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_HOME")); v != "" {
		return v
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".harbor")
	}

	return filepath.Join(home, ".harbor")
}

// Path joins Harbor's root with one or more path elements.
func Path(elem ...string) string {
	parts := make([]string, 0, len(elem)+1)
	parts = append(parts, RootDir())
	parts = append(parts, elem...)
	return filepath.Join(parts...)
}
