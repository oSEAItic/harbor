package auth

// Harbor Keychain — a portable, encrypted credential store at ~/.harbor/keychain.json.
//
// Design:
//   - Works in any environment: macOS, Linux, Docker, CI, sandboxed agents.
//   - Not dependent on OS keychain services (dbus, macOS Keychain, etc.).
//   - Each credential is encrypted with AES-256-GCM; the key is derived from
//     the user's Harbor API key via PBKDF2. Without the API key the file is
//     opaque.  Both files live in ~/.harbor/ (0600), so the combined security
//     posture is equivalent to OS-keychain-backed storage for local threats.
//   - Used as a fallback by Retrieve() when the OS keychain is unavailable.
//   - Written by 'harbor login' (auto-sync from cloud) and 'harbor auth sync'.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/oseaitic/harbor/internal/harborhome"
	"golang.org/x/crypto/pbkdf2"
)

const (
	keychainKDFIter = 100_000
	keychainKeyLen  = 32 // AES-256
	keychainSaltLen = 32
)

type keychainEntry struct {
	Connector string `json:"connector"`
	Blob      string `json:"blob"` // base64(salt||nonce||ciphertext)
}

func keychainFilePath() string {
	return harborhome.Path("keychain.json")
}

// keychainEncrypt encrypts plaintext with a key derived from apiKey via PBKDF2.
// Returns a self-contained blob: salt(32) || nonce(12) || ciphertext.
func keychainEncrypt(plaintext, apiKey string) ([]byte, error) {
	salt := make([]byte, keychainSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := pbkdf2.Key([]byte(apiKey), salt, keychainKDFIter, keychainKeyLen, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	blob := make([]byte, 0, keychainSaltLen+len(nonce)+len(ct))
	blob = append(blob, salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ct...)
	return blob, nil
}

// keychainDecrypt reverses keychainEncrypt.
func keychainDecrypt(blob []byte, apiKey string) (string, error) {
	if len(blob) < keychainSaltLen+12+1 {
		return "", fmt.Errorf("blob too short")
	}
	salt := blob[:keychainSaltLen]
	rest := blob[keychainSaltLen:]
	key := pbkdf2.Key([]byte(apiKey), salt, keychainKDFIter, keychainKeyLen, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(rest) < ns+1 {
		return "", fmt.Errorf("blob too short after salt")
	}
	pt, err := gcm.Open(nil, rest[:ns], rest[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong API key?): %w", err)
	}
	return string(pt), nil
}

func loadKeychain() []keychainEntry {
	data, err := os.ReadFile(keychainFilePath())
	if err != nil {
		return nil
	}
	var entries []keychainEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
}

func saveKeychain(entries []keychainEntry) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(harborhome.RootDir(), 0o755)
	os.WriteFile(keychainFilePath(), data, 0o600)
}

// LoadFromKeychain decrypts and returns the credential for a connector.
// apiKey is used to derive the decryption key.
func LoadFromKeychain(connector, apiKey string) (string, error) {
	for _, e := range loadKeychain() {
		if e.Connector == connector {
			blob, err := base64.StdEncoding.DecodeString(e.Blob)
			if err != nil {
				return "", fmt.Errorf("malformed keychain entry: %w", err)
			}
			return keychainDecrypt(blob, apiKey)
		}
	}
	return "", fmt.Errorf("connector %q not in Harbor Keychain", connector)
}

// SaveToKeychain encrypts and writes a credential to the Harbor Keychain.
// Existing entries for the same connector are replaced.
func SaveToKeychain(connector, secret, apiKey string) error {
	blob, err := keychainEncrypt(secret, apiKey)
	if err != nil {
		return fmt.Errorf("encrypting credential: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(blob)

	entries := loadKeychain()
	for i, e := range entries {
		if e.Connector == connector {
			entries[i].Blob = encoded
			saveKeychain(entries)
			return nil
		}
	}
	entries = append(entries, keychainEntry{Connector: connector, Blob: encoded})
	saveKeychain(entries)
	return nil
}

// RemoveFromKeychain removes a connector's entry from the Harbor Keychain.
func RemoveFromKeychain(connector string) {
	entries := loadKeychain()
	var filtered []keychainEntry
	for _, e := range entries {
		if e.Connector != connector {
			filtered = append(filtered, e)
		}
	}
	saveKeychain(filtered)
}
