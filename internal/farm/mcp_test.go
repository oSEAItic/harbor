package farm

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPToolsExposeSharedFarmWithMutationAnnotations(t *testing.T) {
	tools := mcpTools(func() (*Client, error) { return nil, errors.New("not connected") })
	if len(tools) != 3 {
		t.Fatalf("got %d tools, want 3", len(tools))
	}
	if tools[0].Tool.Name != "harbor_farm_status" || tools[1].Tool.Name != "harbor_farm_plant" || tools[2].Tool.Name != "harbor_farm_harvest" {
		t.Fatalf("unexpected Farm tools: %q, %q, %q", tools[0].Tool.Name, tools[1].Tool.Name, tools[2].Tool.Name)
	}
	if tools[0].Tool.Annotations.ReadOnlyHint == nil || !*tools[0].Tool.Annotations.ReadOnlyHint {
		t.Fatal("Farm status must be annotated read-only")
	}
	for _, tool := range tools[1:] {
		if tool.Tool.Annotations.ReadOnlyHint == nil || *tool.Tool.Annotations.ReadOnlyHint {
			t.Fatalf("%s must be annotated as mutating", tool.Tool.Name)
		}
		if tool.Tool.Annotations.DestructiveHint == nil || !*tool.Tool.Annotations.DestructiveHint {
			t.Fatalf("%s must warn clients before changing the Farm", tool.Tool.Name)
		}
	}
}

func TestMCPFarmMutationValidationRunsBeforeCloudAccess(t *testing.T) {
	factoryCalled := false
	tools := mcpTools(func() (*Client, error) {
		factoryCalled = true
		return nil, errors.New("must not be called")
	})

	plant := mcp.CallToolRequest{}
	plant.Params.Arguments = map[string]any{"plot_index": float64(9), "crop_type": "wheat"}
	result, err := tools[1].Handler(context.Background(), plant)
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("invalid plant result = %#v, err = %v", result, err)
	}

	harvest := mcp.CallToolRequest{}
	harvest.Params.Arguments = map[string]any{"plot_index": float64(-1)}
	result, err = tools[2].Handler(context.Background(), harvest)
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("invalid harvest result = %#v, err = %v", result, err)
	}
	if factoryCalled {
		t.Fatal("invalid mutation reached Harbor Cloud")
	}
}
