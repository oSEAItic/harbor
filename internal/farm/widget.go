package farm

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const farmWidgetURI = "ui://harbor-farm/field-v1.html"

//go:embed farm_widget.html
var farmWidgetHTML string

// MCPResources exposes the portable MCP Apps surface used by compatible
// ChatGPT and Codex hosts. The normal Farm tools remain usable without UI.
func MCPResources() []struct {
	Resource mcp.Resource
	Handler  server.ResourceHandlerFunc
} {
	resource := mcp.NewResource(
		farmWidgetURI,
		"Harbor Farm field",
		mcp.WithResourceDescription("Interactive waiting-time Farm with six plots, session crops, and neighbor activity."),
		mcp.WithMIMEType("text/html;profile=mcp-app"),
	)
	return []struct {
		Resource mcp.Resource
		Handler  server.ResourceHandlerFunc
	}{
		{
			Resource: resource,
			Handler: func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{mcp.TextResourceContents{
					URI:      farmWidgetURI,
					MIMEType: "text/html;profile=mcp-app",
					Text:     farmWidgetHTML,
					Meta: map[string]any{
						"ui": map[string]any{"prefersBorder": false},
					},
				}}, nil
			},
		},
	}
}

func farmOpenTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_farm_open",
		"Open and render the interactive Harbor Farm. Prefer this visual tool whenever the user asks to open, play, view, or check their Farm; use harbor_farm_status only for headless data work.",
		json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	)
	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(true),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(true),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
	tool.Meta = mcp.NewMetaFromMap(map[string]any{
		"ui":                             map[string]any{"resourceUri": farmWidgetURI},
		"openai/outputTemplate":          farmWidgetURI,
		"openai/toolInvocation/invoking": "Walking down to Harbor Farm...",
		"openai/toolInvocation/invoked":  "Harbor Farm is open.",
	})
	return tool
}

func makeFarmOpenHandler(factory mcpClientFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := factory()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		bootstrap, loadErr := client.Bootstrap(ctx)
		if loadErr != nil && bootstrap == nil {
			return mcp.NewToolResultError(loadErr.Error()), nil
		}
		state, err := farmWidgetState(bootstrap, loadErr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encoding interactive Farm: %v", err)), nil
		}
		message := "Interactive Harbor Farm opened."
		if loadErr != nil {
			message = "Interactive Harbor Farm opened from the offline cache."
		}
		result := mcp.NewToolResultText(message)
		result.StructuredContent = state
		return result, nil
	}
}

func farmWidgetState(bootstrap *Bootstrap, loadErr error) (map[string]any, error) {
	data, err := json.Marshal(bootstrap)
	if err != nil {
		return nil, err
	}
	state := make(map[string]any)
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	delete(state, "account")
	delete(state, "active_sessions")
	delete(state, "today_usage")
	state["active_session_count"] = len(bootstrap.ActiveSessions)
	state["ui_version"] = 1
	if loadErr != nil {
		state["warning"] = loadErr.Error()
	}
	return state, nil
}
