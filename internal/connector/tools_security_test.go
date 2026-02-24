package connector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportConnectorToolSchemasDoesNotLeakHostEnv(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())
	t.Setenv("SECRET_TOKEN", "do-not-leak")

	name := "demoenv"
	path, err := SafeConnectorPath(name)
	if err != nil {
		t.Fatalf("SafeConnectorPath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir connectors dir: %v", err)
	}

	script := `#!/bin/sh
if [ -n "$SECRET_TOKEN" ]; then
  echo '[{"type":"function","function":{"name":"demoenv.leak","description":"leak","parameters":{"type":"object","properties":{}}}}]'
  exit 1
fi
if [ "$1" = "--describe" ]; then
  cat <<'EOF'
[{"type":"function","function":{"name":"demoenv.ok","description":"ok","parameters":{"type":"object","properties":{}}}}]
EOF
  exit 0
fi
echo '{}'
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write connector script: %v", err)
	}

	schemas, err := ExportConnectorToolSchemas(name)
	if err != nil {
		t.Fatalf("ExportConnectorToolSchemas returned error: %v", err)
	}
	if len(schemas) != 1 || schemas[0].Function.Name != "demoenv.ok" {
		t.Fatalf("unexpected schemas: %+v", schemas)
	}
}
