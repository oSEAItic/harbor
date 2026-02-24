package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oseaitic/harbor/internal/harborhome"
)

func verifyAndPinChecksum(name string, data []byte, expected string) (string, error) {
	actual := sha256Hex(data)

	expectedNorm, err := normalizeSHA256(expected)
	if err != nil {
		return "", fmt.Errorf("invalid --sha256 value: %w", err)
	}

	pinPath := checksumPinPath(name)

	if expectedNorm != "" {
		if actual != expectedNorm {
			return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedNorm, actual)
		}
		if err := writeChecksumPin(pinPath, actual); err != nil {
			return "", err
		}
		return actual, nil
	}

	pinned, err := readChecksumPin(pinPath)
	if err == nil {
		if actual != pinned {
			return "", fmt.Errorf(
				"checksum mismatch with pinned value: pinned %s, got %s (re-run install with --sha256 %s if this new bundle is trusted)",
				pinned, actual, actual,
			)
		}
		return actual, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("reading pinned checksum: %w", err)
	}

	if err := writeChecksumPin(pinPath, actual); err != nil {
		return "", err
	}
	return actual, nil
}

func checksumPinPath(name string) string {
	return harborhome.Path("checksums", "connectors", name+".sha256")
}

func writeChecksumPin(path, checksum string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating checksum dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(checksum+"\n"), 0o600); err != nil {
		return fmt.Errorf("writing checksum pin: %w", err)
	}
	return nil
}

func readChecksumPin(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum, err := normalizeSHA256(string(data))
	if err != nil {
		return "", fmt.Errorf("invalid pinned checksum in %s: %w", path, err)
	}
	return sum, nil
}

func normalizeSHA256(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return "", nil
	}
	s = strings.TrimPrefix(s, "sha256:")
	if len(s) != 64 {
		return "", fmt.Errorf("must be 64 hex chars")
	}
	if _, err := hex.DecodeString(s); err != nil {
		return "", fmt.Errorf("must be hex: %w", err)
	}
	return s, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
