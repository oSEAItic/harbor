package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/oseaitic/harbor/internal/protocol"
)

// ExportToolSchemas asks each installed connector for its tool schema and
// returns them in OpenAI function-calling format.
func ExportToolSchemas() ([]protocol.ToolSchema, error) {
	installed, err := ListInstalled()
	if err != nil {
		return nil, err
	}

	var schemas []protocol.ToolSchema

	for _, name := range installed {
		binPath := ConnectorPath(name)

		// Each connector supports --describe to emit its tool schema
		cmd := exec.Command(binPath, "--describe")
		cmd.Env = os.Environ()

		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: connector %q does not support --describe, skipping\n", name)
			continue
		}

		var connectorSchemas []protocol.ToolSchema
		if err := json.Unmarshal(out, &connectorSchemas); err != nil {
			fmt.Fprintf(os.Stderr, "warning: connector %q returned invalid tool schema: %v\n", name, err)
			continue
		}

		schemas = append(schemas, connectorSchemas...)
	}

	return schemas, nil
}
