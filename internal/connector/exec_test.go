package connector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oseaitic/harbor/internal/protocol"
)

func TestExecuteValidatesProtocolResponse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	validOutput := `{"data":[],"meta":{"source":"test","connector_version":"1.0.0","schema":"demo.v1","fetched_at":"2026-02-24T00:00:00Z","request_id":"req-1"},"errors":[]}`
	writeTestConnector(t, "demo", validOutput)

	resp, err := Execute(protocol.Request{
		Connector: "demo",
		Resource:  "list",
		Params:    map[string]string{"x": "1"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if resp.Meta.RequestID != "req-1" {
		t.Fatalf("unexpected request_id %q", resp.Meta.RequestID)
	}
}

func TestExecuteRejectsInvalidProtocolResponse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	invalidOutput := `{"data":[],"meta":{"source":"test","connector_version":"1.0.0","schema":"demo.v1","fetched_at":"2026-02-24T00:00:00Z"},"errors":[]}`
	writeTestConnector(t, "demo", invalidOutput)

	_, err := Execute(protocol.Request{
		Connector: "demo",
		Resource:  "list",
	})
	if err == nil {
		t.Fatal("expected Execute to fail for invalid protocol response")
	}
	if !strings.Contains(err.Error(), "invalid protocol response") {
		t.Fatalf("expected protocol validation error, got: %v", err)
	}
}

func TestExecuteDoesNotLeakUnrelatedEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SECRET_TOKEN", "top-secret-value")

	path := ConnectorPath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir connectors dir: %v", err)
	}

	script := `#!/bin/sh
if [ -n "$SECRET_TOKEN" ]; then
cat <<'EOF'
{"data":[],"meta":{"source":"test","connector_version":"1.0.0","schema":"demo.v1","fetched_at":"2026-02-24T00:00:00Z","request_id":"req-leak"},"errors":[{"code":"execution_error","message":"secret leaked"}]}
EOF
exit 1
fi
cat <<'EOF'
{"data":[{"ok":true}],"meta":{"source":"test","connector_version":"1.0.0","schema":"demo.v1","fetched_at":"2026-02-24T00:00:00Z","request_id":"req-safe"},"errors":[]}
EOF
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write connector script: %v", err)
	}

	resp, err := Execute(protocol.Request{
		Connector: "demo",
		Resource:  "list",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if resp.Meta.RequestID != "req-safe" {
		t.Fatalf("unexpected request_id %q", resp.Meta.RequestID)
	}
}

func writeTestConnector(t *testing.T, name, output string) {
	t.Helper()

	path := ConnectorPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir connectors dir: %v", err)
	}

	script := "#!/bin/sh\ncat <<'EOF'\n" + output + "\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write connector script: %v", err)
	}
}
