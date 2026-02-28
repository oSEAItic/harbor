package cloudauth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	kdfIterations = 100_000
	kdfKeyLen     = 32 // AES-256
	saltLen       = 32
)

// CredentialEntry is a list entry returned by the server (no blob).
type CredentialEntry struct {
	Connector string    `json:"connector"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// encryptCredential encrypts plaintext using AES-256-GCM with a key derived from password.
// Returns a self-contained blob: salt(32) || nonce(12) || ciphertext.
func encryptCredential(plaintext, password string) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}

	key := pbkdf2.Key([]byte(password), salt, kdfIterations, kdfKeyLen, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	blob := make([]byte, 0, saltLen+len(nonce)+len(ciphertext))
	blob = append(blob, salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)
	return blob, nil
}

// DecryptCredential decrypts a blob produced by encryptCredential using password.
// Returns the original plaintext credential.
func DecryptCredential(blob []byte, password string) (string, error) {
	if len(blob) < saltLen+12+1 {
		return "", fmt.Errorf("blob too short")
	}

	salt := blob[:saltLen]
	rest := blob[saltLen:]

	// Derive key (same parameters as encryption)
	key := pbkdf2.Key([]byte(password), salt, kdfIterations, kdfKeyLen, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(rest) < nonceSize+1 {
		return "", fmt.Errorf("blob too short after salt")
	}
	nonce := rest[:nonceSize]
	ciphertext := rest[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong password?): %w", err)
	}
	return string(plaintext), nil
}

// StoreCloudCredential encrypts the connector credential client-side and uploads
// the ciphertext blob to harbor-cloud. The server never sees the plaintext.
func StoreCloudCredential(connector, credential, password string, cfg *Config) error {
	blob, err := encryptCredential(credential, password)
	if err != nil {
		return fmt.Errorf("encrypting credential: %w", err)
	}

	body, _ := json.Marshal(map[string]string{
		"blob": base64.StdEncoding.EncodeToString(blob),
	})

	req, err := http.NewRequest(http.MethodPut,
		cfg.Endpoint+"/api/credentials/"+connector,
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("uploading credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// FetchCloudCredentialBlob downloads the encrypted blob for a connector.
// The returned blob must be decrypted client-side with DecryptCredential.
func FetchCloudCredentialBlob(connector string, cfg *Config) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet,
		cfg.Endpoint+"/api/credentials/"+connector, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no cloud credential stored for %q", connector)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var out struct {
		Blob string `json:"blob"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	blob, err := base64.StdEncoding.DecodeString(out.Blob)
	if err != nil {
		return nil, fmt.Errorf("decoding blob: %w", err)
	}
	return blob, nil
}

// ListCloudCredentials returns the list of connector names with stored cloud credentials.
func ListCloudCredentials(cfg *Config) ([]CredentialEntry, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.Endpoint+"/api/credentials", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var entries []CredentialEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return entries, nil
}

// DeleteCloudCredential removes a stored cloud credential.
func DeleteCloudCredential(connector string, cfg *Config) error {
	req, err := http.NewRequest(http.MethodDelete,
		cfg.Endpoint+"/api/credentials/"+connector, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("deleting credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no cloud credential found for %q", connector)
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
