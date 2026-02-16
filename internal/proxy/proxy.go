package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/recall"
	"github.com/oseaitic/harbor/internal/schema"
)

// Config holds configuration for the proxy.
type Config struct {
	Command     string
	Args        []string
	Version     string
	SchemaStore *schema.Store
}

// Run starts the MCP proxy. It launches the upstream MCP server as a subprocess,
// discovers its tools, re-registers them on Harbor's own MCP server, and serves
// the agent over stdio. A harbor_learn_schema tool is also registered so the
// agent can teach Harbor how to compress each tool's output.
func Run(cfg Config) error {
	// Launch upstream MCP server as subprocess
	fmt.Fprintf(os.Stderr, "harbor-proxy: launching upstream %s %v\n", cfg.Command, cfg.Args)

	upstream, err := client.NewStdioMCPClient(cfg.Command, os.Environ(), cfg.Args...)
	if err != nil {
		return fmt.Errorf("launching upstream %q: %w", cfg.Command, err)
	}
	defer upstream.Close()

	// Pipe upstream stderr to our stderr for diagnostics
	if stderr, ok := client.GetStderr(upstream); ok {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					os.Stderr.Write(buf[:n])
				}
				if err != nil {
					return
				}
			}
		}()
	}

	ctx := context.Background()

	// MCP handshake with upstream
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "harbor-proxy",
		Version: cfg.Version,
	}

	_, err = upstream.Initialize(ctx, initReq)
	if err != nil {
		return fmt.Errorf("upstream initialize: %w", err)
	}

	// Discover upstream tools
	toolsResult, err := upstream.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("upstream list tools: %w", err)
	}

	fmt.Fprintf(os.Stderr, "harbor-proxy: discovered %d tools from upstream\n", len(toolsResult.Tools))

	// Set up memory store (best-effort)
	var memStore *memory.Store
	memStore, _ = memory.NewStore()

	// Create Harbor's MCP server
	s := server.NewMCPServer("harbor-proxy", cfg.Version)

	// Re-register each upstream tool with a proxy handler
	for _, tool := range toolsResult.Tools {
		h := &proxyHandler{
			upstream:    upstream,
			toolName:    tool.Name,
			schemaStore: cfg.SchemaStore,
			memStore:    memStore,
		}

		proxyTool := reRegisterTool(tool)
		s.AddTool(proxyTool, h.handle)

		fmt.Fprintf(os.Stderr, "harbor-proxy: registered tool %q\n", tool.Name)
	}

	// Register the harbor_learn_schema teaching tool
	learnTool := mcp.NewToolWithRawSchema(
		"harbor_learn_schema",
		"Teach Harbor how to compress a tool's output. Call this after seeing raw output from a proxied tool to enable automatic compression on future calls. Provide the tool_name, the summary_fields (array of field names to keep), and a summary_template (string with {field} placeholders for a one-line summary per item).",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"tool_name": {
					"type": "string",
					"description": "Name of the upstream tool to learn a schema for"
				},
				"summary_fields": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Field names to keep in compressed output (3-6 recommended)"
				},
				"summary_template": {
					"type": "string",
					"description": "Template with {field} placeholders for a one-line summary per item"
				}
			},
			"required": ["tool_name", "summary_fields", "summary_template"]
		}`),
	)

	s.AddTool(learnTool, makeLearnHandler(cfg.SchemaStore))

	// Register harbor_recall tool for cross-session memory access
	s.AddTool(recall.ToolDefinition(), recall.MakeHandler(memStore))

	// Serve to agent via stdio
	fmt.Fprintf(os.Stderr, "harbor-proxy: serving %d upstream tools + harbor_learn_schema + harbor_recall to agent\n", len(toolsResult.Tools))
	return server.ServeStdio(s)
}

// reRegisterTool creates a new mcp.Tool from an upstream tool, preserving its schema.
func reRegisterTool(upstream mcp.Tool) mcp.Tool {
	// Marshal the upstream tool's input schema to get the raw JSON
	schemaJSON, err := json.Marshal(upstream.InputSchema)
	if err != nil {
		schemaJSON = []byte(`{"type":"object"}`)
	}

	// If the tool has a RawInputSchema, prefer that
	if len(upstream.RawInputSchema) > 0 {
		schemaJSON = upstream.RawInputSchema
	}

	return mcp.NewToolWithRawSchema(
		upstream.Name,
		upstream.Description,
		schemaJSON,
	)
}
