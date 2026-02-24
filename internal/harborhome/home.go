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
// 2. ~/.harbor (if writable)
// 3. ./.harbor (fallback when home directory is unavailable or not writable)
func RootDir() string {
	if v := strings.TrimSpace(os.Getenv("HARBOR_HOME")); v != "" {
		return v
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".harbor")
	}

	preferred := filepath.Join(home, ".harbor")
	if isWritableDir(preferred) {
		return preferred
	}

	return filepath.Join(".", ".harbor")
}

// Path joins Harbor's root with one or more path elements.
func Path(elem ...string) string {
	parts := make([]string, 0, len(elem)+1)
	parts = append(parts, RootDir())
	parts = append(parts, elem...)
	return filepath.Join(parts...)
}

func isWritableDir(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}

	f, err := os.CreateTemp(dir, ".harbor-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
