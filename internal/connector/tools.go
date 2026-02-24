package connector

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/oseaitic/harbor/internal/protocol"
	"github.com/oseaitic/harbor/internal/tooling"
)

// schemaCache caches connector tool schemas per process lifetime.
var (
	schemaCache   = make(map[string][]protocol.ToolSchema)
	schemaCacheMu sync.Mutex
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
		connSchemas, err := getConnectorSchemas(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: connector %q: %v\n", name, err)
			continue
		}
		schemas = append(schemas, connSchemas...)
	}

	return schemas, nil
}

// ExportToolIR exports all installed connector tools in Harbor's canonical Tool IR.
func ExportToolIR() ([]tooling.ToolIR, error) {
	schemas, err := ExportToolSchemas()
	if err != nil {
		return nil, err
	}
	return tooling.FromOpenAISchemas(schemas), nil
}

// ExportMCPToolSchemas exports all installed connector tools as MCP-compatible
// JSON tool definitions.
func ExportMCPToolSchemas() ([]tooling.MCPTool, error) {
	ir, err := ExportToolIR()
	if err != nil {
		return nil, err
	}
	return tooling.ToMCPSchemas(ir), nil
}

// GetResourceSchema returns the ToolFunction for a specific connector resource.
// Results are cached per process lifetime.
func GetResourceSchema(connectorName, resource string) (*protocol.ToolFunction, error) {
	schemas, err := getConnectorSchemas(connectorName)
	if err != nil {
		return nil, err
	}

	fullName := connectorName + "." + resource
	for _, s := range schemas {
		if s.Function.Name == fullName {
			return &s.Function, nil
		}
	}

	return nil, fmt.Errorf("resource %q not found in connector %q schema", resource, connectorName)
}

// ExportConnectorToolSchemas exports tool schemas for a single connector.
func ExportConnectorToolSchemas(name string) ([]protocol.ToolSchema, error) {
	return getConnectorSchemas(name)
}

// getConnectorSchemas fetches and caches tool schemas for a connector.
func getConnectorSchemas(name string) ([]protocol.ToolSchema, error) {
	schemaCacheMu.Lock()
	defer schemaCacheMu.Unlock()

	if cached, ok := schemaCache[name]; ok {
		return cached, nil
	}

	binPath := ConnectorPath(name)

	cmd := exec.Command(binPath, "--describe")
	cmd.Env = os.Environ()

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("does not support --describe, skipping")
	}

	var connectorSchemas []protocol.ToolSchema
	if err := json.Unmarshal(out, &connectorSchemas); err != nil {
		return nil, fmt.Errorf("returned invalid tool schema: %v", err)
	}

	schemaCache[name] = connectorSchemas
	return connectorSchemas, nil
}
