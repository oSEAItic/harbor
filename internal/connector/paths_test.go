package connector

import "testing"

func TestValidateConnectorName(t *testing.T) {
	valid := []string{
		"coingecko",
		"demo_1",
		"demo-1",
		"a",
	}
	for _, name := range valid {
		if err := ValidateConnectorName(name); err != nil {
			t.Fatalf("expected %q to be valid, got error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"../evil",
		"demo.js",
		"UPPER",
		"-bad",
		"_bad",
		"has space",
	}
	for _, name := range invalid {
		if err := ValidateConnectorName(name); err == nil {
			t.Fatalf("expected %q to be invalid", name)
		}
	}
}
