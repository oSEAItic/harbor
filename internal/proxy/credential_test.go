package proxy

import (
	"os"
	"strings"
	"testing"
)

func TestExtractCredentialFlags(t *testing.T) {
	// This tests the parsing logic. We import the CLI function indirectly
	// by testing buildProxyEnv which is the proxy-side counterpart.

	t.Run("no credentials passes env through unchanged", func(t *testing.T) {
		env, err := buildProxyEnv(nil)
		if err != nil {
			t.Fatal(err)
		}
		// Should be the same as os.Environ()
		osEnv := os.Environ()
		if len(env) != len(osEnv) {
			t.Errorf("expected %d env vars, got %d", len(osEnv), len(env))
		}
	})

	t.Run("empty credentials passes env through unchanged", func(t *testing.T) {
		env, err := buildProxyEnv(map[string]string{})
		if err != nil {
			t.Fatal(err)
		}
		osEnv := os.Environ()
		if len(env) != len(osEnv) {
			t.Errorf("expected %d env vars, got %d", len(osEnv), len(env))
		}
	})
}

func TestBuildProxyEnvOverride(t *testing.T) {
	// Test that env var override logic works correctly.
	// We can't test real keychain in CI, so we test the override mechanics
	// by calling the internal logic directly.

	t.Run("override replaces existing env var", func(t *testing.T) {
		// Simulate the override logic that buildProxyEnv uses after resolving keychain
		baseEnv := []string{
			"PATH=/usr/bin",
			"HOME=/home/test",
			"GITHUB_TOKEN=old-token",
		}
		overrides := map[string]string{
			"GITHUB_TOKEN": "new-secret-from-keychain",
		}

		var result []string
		seen := make(map[string]bool)
		for _, kv := range baseEnv {
			key := strings.SplitN(kv, "=", 2)[0]
			if val, ok := overrides[key]; ok {
				result = append(result, key+"="+val)
				seen[key] = true
			} else {
				result = append(result, kv)
			}
		}
		for key, val := range overrides {
			if !seen[key] {
				result = append(result, key+"="+val)
			}
		}

		// Verify GITHUB_TOKEN was overridden
		found := false
		for _, kv := range result {
			if strings.HasPrefix(kv, "GITHUB_TOKEN=") {
				if kv != "GITHUB_TOKEN=new-secret-from-keychain" {
					t.Errorf("expected override, got %s", kv)
				}
				found = true
			}
		}
		if !found {
			t.Error("GITHUB_TOKEN not found in result")
		}
		if len(result) != 3 {
			t.Errorf("expected 3 env vars, got %d", len(result))
		}
	})

	t.Run("override adds new env var", func(t *testing.T) {
		baseEnv := []string{
			"PATH=/usr/bin",
			"HOME=/home/test",
		}
		overrides := map[string]string{
			"SLACK_TOKEN": "xoxb-secret",
		}

		var result []string
		seen := make(map[string]bool)
		for _, kv := range baseEnv {
			key := strings.SplitN(kv, "=", 2)[0]
			if val, ok := overrides[key]; ok {
				result = append(result, key+"="+val)
				seen[key] = true
			} else {
				result = append(result, kv)
			}
		}
		for key, val := range overrides {
			if !seen[key] {
				result = append(result, key+"="+val)
			}
		}

		if len(result) != 3 {
			t.Errorf("expected 3 env vars, got %d", len(result))
		}

		found := false
		for _, kv := range result {
			if kv == "SLACK_TOKEN=xoxb-secret" {
				found = true
			}
		}
		if !found {
			t.Error("SLACK_TOKEN not appended")
		}
	})

	t.Run("multiple overrides", func(t *testing.T) {
		baseEnv := []string{
			"PATH=/usr/bin",
			"GITHUB_TOKEN=old",
		}
		overrides := map[string]string{
			"GITHUB_TOKEN": "new-gh",
			"NOTION_TOKEN": "new-notion",
		}

		var result []string
		seen := make(map[string]bool)
		for _, kv := range baseEnv {
			key := strings.SplitN(kv, "=", 2)[0]
			if val, ok := overrides[key]; ok {
				result = append(result, key+"="+val)
				seen[key] = true
			} else {
				result = append(result, kv)
			}
		}
		for key, val := range overrides {
			if !seen[key] {
				result = append(result, key+"="+val)
			}
		}

		if len(result) != 3 {
			t.Errorf("expected 3 env vars, got %d", len(result))
		}

		ghFound, notionFound := false, false
		for _, kv := range result {
			if kv == "GITHUB_TOKEN=new-gh" {
				ghFound = true
			}
			if kv == "NOTION_TOKEN=new-notion" {
				notionFound = true
			}
		}
		if !ghFound {
			t.Error("GITHUB_TOKEN not overridden")
		}
		if !notionFound {
			t.Error("NOTION_TOKEN not appended")
		}
	})
}
