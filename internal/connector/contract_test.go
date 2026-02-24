package connector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunContractSuite(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "connector.js")

	script := `#!/usr/bin/env node
const args = process.argv.slice(2);
const resourceIdx = args.indexOf("--resource");
const paramsIdx = args.indexOf("--params");
const resource = resourceIdx >= 0 ? args[resourceIdx+1] : "";
const params = paramsIdx >= 0 ? JSON.parse(args[paramsIdx+1]) : {};
const out = {
  data: [{resource, ok: true, ...params}],
  meta: {
    source: "test",
    connector_version: "0.1.0",
    schema: "test."+resource+".v1",
    fetched_at: "2026-02-24T00:00:00Z",
    request_id: "req-1"
  },
  raw: null,
  errors: []
};
console.log(JSON.stringify(out));
`

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	suite := &ContractSuite{
		Cases: []ContractCase{
			{Name: "case1", Resource: "prices", Params: map[string]string{"symbol": "BTC"}},
			{Name: "case2", Resource: "quote", Params: map[string]string{"symbol": "AAPL"}},
		},
	}

	if err := RunContractSuite(binPath, suite); err != nil {
		t.Fatalf("RunContractSuite: %v", err)
	}
}

func TestRunContractCaseFailsOnInvalidProtocol(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bad.js")
	if err := os.WriteFile(binPath, []byte(`#!/usr/bin/env node
console.log(JSON.stringify({data:[],meta:{source:"x"}}));
`), 0o755); err != nil {
		t.Fatal(err)
	}

	err := RunContractCase(binPath, ContractCase{Resource: "prices"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "protocol validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
