package tooling

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/oseaitic/harbor/internal/protocol"
)

func TestRoundTripOpenAI(t *testing.T) {
	openai := []protocol.ToolSchema{
		{
			Type: "function",
			Function: protocol.ToolFunction{
				Name:            "github.issues",
				Description:     "List issues",
				Parameters:      json.RawMessage(`{"type":"object","properties":{"repo":{"type":"string"}},"required":["repo"]}`),
				SummaryFields:   []string{"id", "title", "state"},
				SummaryTemplate: "{id} {title} ({state})",
			},
		},
	}

	ir := FromOpenAISchemas(openai)
	got := ToOpenAISchemas(ir)

	if !reflect.DeepEqual(got, openai) {
		t.Fatalf("round-trip mismatch:\n got=%#v\nwant=%#v", got, openai)
	}
}

func TestExportersGolden(t *testing.T) {
	ir := []ToolIR{
		{
			Name:            "github.issues",
			Description:     "List repository issues",
			Parameters:      json.RawMessage(`{"type":"object","properties":{"owner":{"type":"string"},"repo":{"type":"string"}},"required":["owner","repo"]}`),
			SummaryFields:   []string{"id", "title", "state"},
			SummaryTemplate: "{id}: {title} ({state})",
		},
		{
			Name:        "stripe.balance",
			Description: "Get Stripe account balance",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
	}

	openaiBytes, err := json.MarshalIndent(ToOpenAISchemas(ir), "", "  ")
	if err != nil {
		t.Fatalf("marshal openai: %v", err)
	}
	assertGolden(t, "openai.golden.json", openaiBytes)

	mcpBytes, err := json.MarshalIndent(ToMCPSchemas(ir), "", "  ")
	if err != nil {
		t.Fatalf("marshal mcp: %v", err)
	}
	assertGolden(t, "mcp.golden.json", mcpBytes)
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}

	got = append(got, '\n')
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
