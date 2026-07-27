package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/connector"
	harborctx "github.com/oseaitic/harbor/internal/context"
	"github.com/oseaitic/harbor/internal/executor"
	"github.com/oseaitic/harbor/internal/farm"
	"github.com/oseaitic/harbor/internal/httpfetch"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/pipeline"
	"github.com/oseaitic/harbor/internal/protocol"
	"github.com/oseaitic/harbor/internal/recall"
	"github.com/oseaitic/harbor/internal/remember"
)

// Serve starts a stdio MCP server that exposes all installed connectors as tools.
// Each connector resource becomes an MCP tool. Tool calls run through the shared
// pipeline (execute → compile → memory), so Claude/Cursor get context compilation
// and memory for free.
func Serve(version string) error {
	s := server.NewMCPServer("harbor", version)

	schemas, err := connector.ExportToolSchemas()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: discovering tools: %v\n", err)
	}

	for _, schema := range schemas {
		tool := mcp.NewToolWithRawSchema(
			schema.Function.Name,
			schema.Function.Description,
			schema.Function.Parameters,
		)
		handler := makeToolHandler(schema.Function)
		s.AddTool(tool, handler)
	}

	// Register harbor_recall tool for cross-session memory access
	memStore, _ := memory.NewStore()
	s.AddTool(recall.ToolDefinition(), recall.MakeHandler(memStore))

	// Register harbor_remember tool for persisting analysis notes
	s.AddTool(remember.ToolDefinition(), remember.MakeHandler(memStore))

	// Register harbor_http tool for auth-proxy HTTP fetching
	s.AddTool(httpfetch.ToolDefinition(), httpfetch.MakeHandler())

	// Farm is a Harbor capability, so every MCP client sees the same ledger as
	// the CLI and Studio instead of maintaining surface-specific state.
	for _, farmTool := range farm.MCPTools(version) {
		s.AddTool(farmTool.Tool, farmTool.Handler)
	}
	for _, farmResource := range farm.MCPResources() {
		s.AddResource(farmResource.Resource, farmResource.Handler)
	}

	return server.ServeStdio(s)
}

// makeToolHandler returns an MCP tool handler that calls the shared pipeline.
func makeToolHandler(fn protocol.ToolFunction) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		connName, resource, err := pipeline.ParseConnectorResource(req.Params.Name)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tool name: %v", err)), nil
		}

		// Extract params from MCP arguments as map[string]string
		params := make(map[string]string)
		if args := req.GetArguments(); args != nil {
			for k, v := range args {
				switch val := v.(type) {
				case string:
					params[k] = val
				default:
					b, _ := json.Marshal(val)
					params[k] = string(b)
				}
			}
		}

		exec := executor.NewLocalExecutor()
		result, err := pipeline.Execute(exec, connName, resource, params, fn.SummaryFields, pipeline.Options{
			Compile: harborctx.DefaultOptions(),
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
		}

		respJSON, err := json.Marshal(result.Response)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(respJSON)), nil
	}
}
