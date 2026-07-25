package farm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/cloudauth"
)

// MCPTools exposes the same Harbor-owned Farm ledger used by the CLI and Studio.
func MCPTools(version string) []struct {
	Tool    mcp.Tool
	Handler server.ToolHandlerFunc
} {
	factory := func() (*Client, error) {
		cfg, err := cloudauth.Load()
		if err != nil {
			return nil, fmt.Errorf("Harbor Cloud is not connected; run 'harbor cloud enable' or 'harbor login'")
		}
		return NewClient(cfg, version), nil
	}
	return mcpTools(factory)
}

type mcpClientFactory func() (*Client, error)

func mcpTools(factory mcpClientFactory) []struct {
	Tool    mcp.Tool
	Handler server.ToolHandlerFunc
} {
	return []struct {
		Tool    mcp.Tool
		Handler server.ToolHandlerFunc
	}{
		{Tool: farmStatusTool(), Handler: makeFarmStatusHandler(factory)},
		{Tool: farmPlantTool(), Handler: makeFarmPlantHandler(factory)},
		{Tool: farmHarvestTool(), Handler: makeFarmHarvestHandler(factory)},
	}
}

func farmStatusTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_farm_status",
		"Read the authenticated user's Harbor Farm profile, plots, token usage, and active agent sessions. This is the same ledger shown in Harbor Studio and the harbor farm CLI.",
		json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	)
	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(true),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(true),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
	return tool
}

func farmPlantTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_farm_plant",
		"Mutate the shared Harbor Farm by planting one crop in an empty plot and spending Farm coins.",
		json.RawMessage(`{
			"type":"object",
			"properties":{
				"plot_index":{"type":"integer","minimum":0,"maximum":5,"description":"Zero-based Farm plot index"},
				"crop_type":{"type":"string","enum":["wheat","carrot","tomato"]}
			},
			"required":["plot_index","crop_type"],
			"additionalProperties":false
		}`),
	)
	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(true),
		IdempotentHint:  mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
	return tool
}

func farmHarvestTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_farm_harvest",
		"Mutate the shared Harbor Farm by harvesting a ready crop into Farm coins and XP.",
		json.RawMessage(`{
			"type":"object",
			"properties":{"plot_index":{"type":"integer","minimum":0,"maximum":5,"description":"Zero-based Farm plot index"}},
			"required":["plot_index"],
			"additionalProperties":false
		}`),
	)
	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(true),
		IdempotentHint:  mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
	return tool
}

func makeFarmStatusHandler(factory mcpClientFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := factory()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		bootstrap, err := client.Bootstrap(ctx)
		if err != nil && bootstrap == nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, marshalErr := json.Marshal(bootstrap)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encoding Farm status: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeFarmPlantHandler(factory mcpClientFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		plot := req.GetInt("plot_index", -1)
		crop := req.GetString("crop_type", "")
		if plot < 0 || plot >= 6 {
			return mcp.NewToolResultError("plot_index must be 0-5"), nil
		}
		if crop != "wheat" && crop != "carrot" && crop != "tomato" {
			return mcp.NewToolResultError("crop_type must be wheat, carrot, or tomato"), nil
		}
		client, err := factory()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.Plant(ctx, plot, crop, uuid.NewString()); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Planted %s in plot %d.", crop, plot)), nil
	}
}

func makeFarmHarvestHandler(factory mcpClientFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		plot := req.GetInt("plot_index", -1)
		if plot < 0 || plot >= 6 {
			return mcp.NewToolResultError("plot_index must be 0-5"), nil
		}
		client, err := factory()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.Harvest(ctx, plot); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Harvested plot %d.", plot)), nil
	}
}
