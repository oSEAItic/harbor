package httpfetch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	harborctx "github.com/oseaitic/harbor/internal/context"
	"github.com/oseaitic/harbor/internal/pipeline"
)

// ToolDefinition returns the MCP tool definition for harbor_http.
func ToolDefinition() mcp.Tool {
	return mcp.NewToolWithRawSchema(
		"harbor_http",
		"Make an authenticated HTTP request through Harbor's auth proxy. "+
			"Harbor injects credentials from its secure keychain — you never see raw API keys. "+
			"Responses are cached in memory and benefit from schema learning. "+
			"Use 'auth' to specify which keychain credential to use (run 'harbor auth list' to see available). "+
			"Use 'auth_header' to control how the credential is injected into the request.",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "Full URL to fetch (e.g. https://api.coingecko.com/v3/search/trending)"
				},
				"method": {
					"type": "string",
					"enum": ["GET", "POST", "PUT", "DELETE", "PATCH"],
					"description": "HTTP method (default: GET)"
				},
				"body": {
					"type": "string",
					"description": "Request body for POST/PUT/PATCH"
				},
				"auth": {
					"type": "string",
					"description": "Credential name from Harbor keychain (e.g. 'coingecko', 'github-pat')"
				},
				"auth_header": {
					"type": "string",
					"description": "How to inject the credential. Examples: 'Authorization: Bearer', 'x-cg-pro-api-key', 'X-API-Key'. Default: 'Authorization: Bearer'"
				}
			},
			"required": ["url", "auth"]
		}`),
	)
}

// MakeHandler returns an MCP tool handler for harbor_http.
func MakeHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url := req.GetString("url", "")
		method := req.GetString("method", "GET")
		body := req.GetString("body", "")
		authName := req.GetString("auth", "")
		authHeader := req.GetString("auth_header", "Authorization: Bearer")

		if url == "" {
			return mcp.NewToolResultError("url is required"), nil
		}
		if authName == "" {
			return mcp.NewToolResultError("auth is required — specify which Harbor keychain credential to use"), nil
		}

		// Execute HTTP fetch
		result, err := Fetch(ctx, Options{
			URL:        url,
			Method:     method,
			Body:       body,
			AuthName:   authName,
			AuthHeader: authHeader,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetch error: %v", err)), nil
		}

		// Run through pipeline for memory + schema + recall
		exec := &PassthroughExecutor{Response: result.Response}
		pipeResult, err := pipeline.Execute(exec, result.Connector, result.Resource, nil, nil, pipeline.Options{
			Compile: harborctx.DefaultOptions(),
		})
		if err == nil {
			result.Response = pipeResult.Response
		}

		respJSON, err := json.Marshal(result.Response)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(respJSON)), nil
	}
}
