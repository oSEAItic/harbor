package cli

import (
	"testing"
	"time"
)

func TestParseWorklogDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"90m":  90 * time.Minute,
		"2d":   48 * time.Hour,
		"1.5d": 36 * time.Hour,
	}
	for input, want := range tests {
		got, err := parseWorklogDuration(input)
		if err != nil {
			t.Fatalf("parseWorklogDuration(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseWorklogDuration(%q) = %s, want %s", input, got, want)
		}
	}
}
