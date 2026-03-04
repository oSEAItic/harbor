package govern

import "strings"

// Policy represents the access control policy for a request.
// It covers all three layers of Harbor's govern model:
//   - Connectors (Layer 1+2): which connectors and resources are accessible
//   - MaxVisibility (Layer 3): field-level visibility ceiling
//
// A nil or zero-value Policy means no restrictions (backward compatible).
type Policy struct {
	// Connectors maps connector names to lists of allowed resource names.
	// Use "*" as a connector key to allow all connectors.
	// Use ["*"] as the resource list to allow all resources within a connector.
	// An empty or nil map means no connector-level restrictions.
	Connectors map[string][]string `json:"connectors,omitempty"`

	// MaxVisibility sets the ceiling for field visibility.
	// Fields declared with a higher visibility level will be filtered out.
	// Empty string means no visibility restrictions.
	MaxVisibility string `json:"max_visibility,omitempty"`

	// FieldOverrides provides per-field visibility overrides from a govern profile.
	// These override the connector's default field_visibility annotations.
	// Keys are field names (scoped to the current connector.resource request).
	FieldOverrides map[string]string `json:"field_overrides,omitempty"`
}

// CanAccess reports whether the policy allows accessing the given connector and resource.
// A nil policy or empty Connectors map permits everything.
func (p *Policy) CanAccess(connector, resource string) bool {
	if p == nil || len(p.Connectors) == 0 {
		return true
	}

	// Check specific connector entry.
	if resources, ok := p.Connectors[connector]; ok {
		return containsAny(resources, "*", resource)
	}

	// Check wildcard connector entry.
	if resources, ok := p.Connectors["*"]; ok {
		return containsAny(resources, "*", resource)
	}

	return false
}

// AllowedConnectorNames returns the list of explicitly allowed connector names.
// Returns nil if all connectors are allowed (wildcard or empty policy).
// This is used to filter the /tools endpoint response.
func (p *Policy) AllowedConnectorNames() []string {
	if p == nil || len(p.Connectors) == 0 {
		return nil
	}

	// If wildcard is present, all connectors are allowed.
	if _, ok := p.Connectors["*"]; ok {
		return nil
	}

	names := make([]string, 0, len(p.Connectors))
	for name := range p.Connectors {
		names = append(names, name)
	}
	return names
}

// containsAny reports whether the slice contains any of the targets.
func containsAny(slice []string, targets ...string) bool {
	for _, s := range slice {
		for _, t := range targets {
			if strings.EqualFold(s, t) {
				return true
			}
		}
	}
	return false
}
