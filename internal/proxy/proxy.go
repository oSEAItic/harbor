package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/auth"
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
	Metrics     *MetricsLogger
	Credentials map[string]string // env var name → keychain key name
}

// Run starts the MCP proxy. It launches the upstream MCP server as a subprocess,
// discovers its tools, re-registers them on Harbor's own MCP server, and serves
// the agent over stdio. A harbor_learn_schema tool is also registered so the
// agent can teach Harbor how to compress each tool's output.
func Run(cfg Config) error {
	// Build environment with credential injection from OS keychain
	env, err := buildProxyEnv(cfg.Credentials)
	if err != nil {
		return fmt.Errorf("preparing proxy environment: %w", err)
	}

	// Launch upstream MCP server as subprocess
	fmt.Fprintf(os.Stderr, "harbor-proxy: launching upstream %s %v\n", cfg.Command, cfg.Args)

	upstream, err := client.NewStdioMCPClient(cfg.Command, env, cfg.Args...)
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

	// MCP handshake with upstream (30s timeout to avoid hanging on unresponsive servers)
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "harbor-proxy",
		Version: cfg.Version,
	}

	_, err = upstream.Initialize(initCtx, initReq)
	if err != nil {
		return fmt.Errorf("upstream initialize (is %q running?): %w", cfg.Command, err)
	}

	// Discover upstream tools
	toolsResult, err := upstream.ListTools(initCtx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("upstream list tools: %w", err)
	}

	fmt.Fprintf(os.Stderr, "harbor-proxy: discovered %d tools from upstream\n", len(toolsResult.Tools))

	// Set up memory store (best-effort — proxy works without it)
	var memStore *memory.Store
	if ms, err := memory.NewStore(); err != nil {
		fmt.Fprintf(os.Stderr, "harbor-proxy: memory disabled: %v\n", err)
	} else {
		memStore = ms
	}

	metrics := cfg.Metrics
	if metrics == nil {
		if m, err := NewMetricsLogger(); err != nil {
			fmt.Fprintf(os.Stderr, "harbor-proxy: metrics disabled: %v\n", err)
		} else {
			metrics = m
		}
	}

	// Create Harbor's MCP server
	s := server.NewMCPServer("harbor-proxy", cfg.Version)

	// Re-register each upstream tool with a proxy handler
	for _, tool := range toolsResult.Tools {
		h := &proxyHandler{
			upstream:    upstream,
			toolName:    tool.Name,
			schemaStore: cfg.SchemaStore,
			memStore:    memStore,
			metrics:     metrics,
		}

		proxyTool := reRegisterTool(tool)
		s.AddTool(proxyTool, h.handle)

		fmt.Fprintf(os.Stderr, "harbor-proxy: registered tool %q\n", tool.Name)
	}

	// Register the harbor_learn_schema teaching tool
	learnTool := mcp.NewToolWithRawSchema(
		"harbor_learn_schema",
		"IMPORTANT: You MUST call this tool whenever a tool response contains '[Harbor:' at the end. "+
			"This teaches Harbor which fields matter so future calls return compressed, focused data instead of raw noise. "+
			"Look at the raw output, pick the 3-6 most useful fields, and write a one-line template. "+
			"Example: for a list of issues, summary_fields=[\"number\",\"title\",\"state\"], summary_template=\"#{number} {title} [{state}]\".",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"tool_name": {
					"type": "string",
					"description": "The tool name from the [Harbor:] hint"
				},
				"summary_fields": {
					"type": "array",
					"items": {"type": "string"},
					"description": "3-6 most important field names to keep"
				},
				"summary_template": {
					"type": "string",
					"description": "One-line template with {field} placeholders, e.g. '{name} ({type}, {size} bytes)'"
				}
			},
			"required": ["tool_name", "summary_fields", "summary_template"]
		}`),
	)

	s.AddTool(learnTool, makeLearnHandler(cfg.SchemaStore, memStore))

	// Register harbor_recall tool for cross-session memory access
	s.AddTool(recall.ToolDefinition(), recall.MakeHandler(memStore))

	// Serve to agent via stdio
	fmt.Fprintf(os.Stderr, "harbor-proxy: serving %d upstream tools + harbor_learn_schema + harbor_recall to agent\n", len(toolsResult.Tools))
	return server.ServeStdio(s)
}

// buildProxyEnv constructs the environment for the upstream MCP server process.
// Without credentials, it passes through the current environment unchanged.
// With credentials, it resolves each keychain key and injects/overrides the
// corresponding environment variable — so API keys never appear in config files.
func buildProxyEnv(credentials map[string]string) ([]string, error) {
	env := os.Environ()
	if len(credentials) == 0 {
		return env, nil
	}

	// Resolve secrets from OS keychain
	overrides := make(map[string]string, len(credentials))
	for envVar, keychainKey := range credentials {
		secret, err := auth.Retrieve(keychainKey)
		if err != nil {
			return nil, fmt.Errorf("credential %q (keychain key %q): %w", envVar, keychainKey, err)
		}
		overrides[envVar] = secret
		fmt.Fprintf(os.Stderr, "harbor-proxy: injecting credential %s from keychain\n", envVar)
	}

	// Apply overrides: replace existing vars or append new ones
	var result []string
	seen := make(map[string]bool, len(overrides))
	for _, kv := range env {
		key := strings.SplitN(kv, "=", 2)[0]
		if val, ok := overrides[key]; ok {
			result = append(result, key+"="+val)
			seen[key] = true
		} else {
			result = append(result, kv)
		}
	}
	for key, val := range overrides {
		if !seen[key] {
			result = append(result, key+"="+val)
		}
	}

	return result, nil
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
