package memory

import (
	"strings"
	"testing"
)

func TestCanonicalKey(t *testing.T) {
	tests := []struct {
		connector string
		resource  string
		params    map[string]string
		want      string
	}{
		{"coingecko", "prices", map[string]string{"ids": "bitcoin", "vs_currencies": "usd"}, "coingecko.prices?ids=bitcoin&vs_currencies=usd"},
		{"coingecko", "prices", map[string]string{"vs_currencies": "usd", "ids": "bitcoin"}, "coingecko.prices?ids=bitcoin&vs_currencies=usd"}, // sorted
		{"yahoo", "quote", nil, "yahoo.quote?"},
		{"coingecko", "trending", map[string]string{}, "coingecko.trending?"},
	}

	for _, tt := range tests {
		got := canonicalKey(tt.connector, tt.resource, tt.params)
		if got != tt.want {
			t.Errorf("canonicalKey(%q, %q, %v) = %q, want %q", tt.connector, tt.resource, tt.params, got, tt.want)
		}
	}
}

func TestSortedParams(t *testing.T) {
	// Deterministic regardless of map iteration order
	params := map[string]string{"z": "3", "a": "1", "m": "2"}
	got := sortedParams(params)
	if got != "a=1&m=2&z=3" {
		t.Errorf("sortedParams = %q, want %q", got, "a=1&m=2&z=3")
	}

	// Empty params
	got = sortedParams(nil)
	if got != "" {
		t.Errorf("sortedParams(nil) = %q, want empty", got)
	}

	got = sortedParams(map[string]string{})
	if got != "" {
		t.Errorf("sortedParams({}) = %q, want empty", got)
	}
}

func TestParamsMatch(t *testing.T) {
	entry := map[string]string{"ids": "bitcoin", "vs_currencies": "usd", "extra": "field"}

	// Subset match
	if !paramsMatch(entry, map[string]string{"ids": "bitcoin"}) {
		t.Error("expected subset match")
	}

	// Full match
	if !paramsMatch(entry, map[string]string{"ids": "bitcoin", "vs_currencies": "usd"}) {
		t.Error("expected full match")
	}

	// No match
	if paramsMatch(entry, map[string]string{"ids": "ethereum"}) {
		t.Error("expected no match for different value")
	}

	// Missing key
	if paramsMatch(entry, map[string]string{"missing": "key"}) {
		t.Error("expected no match for missing key")
	}

	// Nil query matches everything
	if !paramsMatch(entry, nil) {
		t.Error("nil query should match everything")
	}
}

func TestGenerateID(t *testing.T) {
	id := generateID("coingecko", "prices", map[string]string{"ids": "bitcoin"})

	if !strings.HasPrefix(id, "mem_") {
		t.Errorf("ID %q does not start with 'mem_'", id)
	}
	if len(id) != 12 { // "mem_" + 8 hex chars
		t.Errorf("ID %q has length %d, want 12", id, len(id))
	}

	// Different calls produce different IDs (timestamp-based)
	id2 := generateID("coingecko", "prices", map[string]string{"ids": "bitcoin"})
	if id == id2 {
		t.Error("consecutive calls should produce different IDs")
	}
}
