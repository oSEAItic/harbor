package farm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
		{Tool: farmConnectTool(), Handler: makeFarmConnectHandler(factory)},
		{Tool: farmVisitTool(), Handler: makeFarmVisitHandler(factory)},
		{Tool: farmForageTool(), Handler: makeFarmForageHandler(factory)},
	}
}

func farmConnectTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_farm_connect",
		"Connect another Harbor account as a Farm neighbor using their eight-character Farm code.",
		json.RawMessage(`{"type":"object","properties":{"farm_code":{"type":"string","minLength":8,"maxLength":8}},"required":["farm_code"],"additionalProperties":false}`),
	)
	tool.Annotations = mcp.ToolAnnotation{ReadOnlyHint: mcp.ToBoolPtr(false), DestructiveHint: mcp.ToBoolPtr(false), IdempotentHint: mcp.ToBoolPtr(true), OpenWorldHint: mcp.ToBoolPtr(true)}
	return tool
}

func farmVisitTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_farm_visit",
		"Visit a connected neighbor's Farm, including ready plots and their public revealed session crops.",
		json.RawMessage(`{"type":"object","properties":{"farm_code":{"type":"string","minLength":8,"maxLength":8}},"required":["farm_code"],"additionalProperties":false}`),
	)
	tool.Annotations = mcp.ToolAnnotation{ReadOnlyHint: mcp.ToBoolPtr(true), DestructiveHint: mcp.ToBoolPtr(false), IdempotentHint: mcp.ToBoolPtr(true), OpenWorldHint: mcp.ToBoolPtr(true)}
	return tool
}

func farmForageTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_farm_forage",
		"Gather one limited clipping from a connected neighbor's ready crop. Each visitor can gather once and the owner retains at least 80 percent of the harvest.",
		json.RawMessage(`{"type":"object","properties":{"farm_code":{"type":"string","minLength":8,"maxLength":8},"plot_index":{"type":"integer","minimum":0,"maximum":5}},"required":["farm_code","plot_index"],"additionalProperties":false}`),
	)
	tool.Annotations = mcp.ToolAnnotation{ReadOnlyHint: mcp.ToBoolPtr(false), DestructiveHint: mcp.ToBoolPtr(true), IdempotentHint: mcp.ToBoolPtr(false), OpenWorldHint: mcp.ToBoolPtr(true)}
	return tool
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

func makeFarmConnectHandler(factory mcpClientFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		farmCode := strings.ToUpper(strings.TrimSpace(req.GetString("farm_code", "")))
		if len(farmCode) != 8 {
			return mcp.NewToolResultError("farm_code must be 8 characters"), nil
		}
		client, err := factory()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := client.ConnectNeighbor(ctx, farmCode); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("Connected to Farm " + farmCode + "."), nil
	}
}

func makeFarmVisitHandler(factory mcpClientFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		farmCode := strings.ToUpper(strings.TrimSpace(req.GetString("farm_code", "")))
		client, err := factory()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		neighbor, err := client.VisitNeighbor(ctx, farmCode)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := json.Marshal(neighbor)
		if err != nil {
			return mcp.NewToolResultError("encoding neighbor Farm: " + err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeFarmForageHandler(factory mcpClientFactory) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		farmCode := strings.ToUpper(strings.TrimSpace(req.GetString("farm_code", "")))
		plot := req.GetInt("plot_index", -1)
		if plot < 0 || plot >= 6 {
			return mcp.NewToolResultError("plot_index must be 0-5"), nil
		}
		client, err := factory()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, err := client.ForageNeighbor(ctx, farmCode, plot)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("encoding forage result: " + err.Error()), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
