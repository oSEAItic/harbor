package memory_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/oseaitic/harbor/internal/memory"
)

func TestCooccurrenceEdgeBuildup(t *testing.T) {
	dir := t.TempDir()

	// Simulate: call coingecko, then yahoo within 5min window
	memory.RecordCall(dir, "coingecko")
	memory.RecordCall(dir, "yahoo")

	// Check file
	raw, _ := os.ReadFile(filepath.Join(dir, "cooccurrence.json"))
	t.Logf("After 2 calls (different connectors):\n%s", raw)

	// Should have 1 edge with count=1
	related := memory.RelatedConnectors(dir, "coingecko", 5)
	if len(related) != 0 {
		t.Errorf("expected 0 related (count=1 < minEdgeCount=2), got %v", related)
	}

	// Second round: call them together again
	memory.RecordCall(dir, "coingecko")
	memory.RecordCall(dir, "yahoo")

	raw, _ = os.ReadFile(filepath.Join(dir, "cooccurrence.json"))
	t.Logf("After 4 calls (2 rounds):\n%s", raw)

	// Now edge count should be >= 2
	related = memory.RelatedConnectors(dir, "coingecko", 5)
	t.Logf("Related connectors for coingecko: %v", related)
	if len(related) != 1 || related[0] != "yahoo" {
		t.Errorf("expected [yahoo], got %v", related)
	}

	// Reverse direction should also work
	related = memory.RelatedConnectors(dir, "yahoo", 5)
	if len(related) != 1 || related[0] != "coingecko" {
		t.Errorf("expected [coingecko], got %v", related)
	}

	// Third connector: binance
	memory.RecordCall(dir, "binance")
	memory.RecordCall(dir, "coingecko")
	memory.RecordCall(dir, "binance")
	memory.RecordCall(dir, "coingecko")

	related = memory.RelatedConnectors(dir, "coingecko", 5)
	t.Logf("After adding binance: coingecko related = %v", related)

	// Verify final state
	raw, _ = os.ReadFile(filepath.Join(dir, "cooccurrence.json"))
	var data struct {
		Edges []struct {
			From  string `json:"from"`
			To    string `json:"to"`
			Count int    `json:"count"`
		} `json:"edges"`
	}
	json.Unmarshal(raw, &data)
	for _, e := range data.Edges {
		t.Logf("  edge: %s → %s (count=%d)", e.From, e.To, e.Count)
	}
}
