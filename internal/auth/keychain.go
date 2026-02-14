package auth

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "harbor"

// Store saves a credential for a connector in the OS keychain.
func Store(connector string, secret string) error {
	if connector == "" {
		return fmt.Errorf("connector name is required")
	}
	if secret == "" {
		return fmt.Errorf("secret is required")
	}
	return keyring.Set(serviceName, connector, secret)
}

// Retrieve loads a stored credential for a connector.
func Retrieve(connector string) (string, error) {
	if connector == "" {
		return "", fmt.Errorf("connector name is required")
	}
	return keyring.Get(serviceName, connector)
}

// Delete removes a stored credential.
func Delete(connector string) error {
	return keyring.Delete(serviceName, connector)
}
