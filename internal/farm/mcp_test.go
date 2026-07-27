package farm

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPToolsExposeSharedFarmWithMutationAnnotations(t *testing.T) {
	tools := mcpTools(func() (*Client, error) { return nil, errors.New("not connected") })
	if len(tools) != 6 {
		t.Fatalf("got %d tools, want 6", len(tools))
	}
	want := []string{"harbor_farm_status", "harbor_farm_plant", "harbor_farm_harvest", "harbor_farm_connect", "harbor_farm_visit", "harbor_farm_forage"}
	for i, name := range want {
		if tools[i].Tool.Name != name {
			t.Fatalf("tool %d = %q, want %q", i, tools[i].Tool.Name, name)
		}
	}
	for _, index := range []int{0, 4} {
		if tools[index].Tool.Annotations.ReadOnlyHint == nil || !*tools[index].Tool.Annotations.ReadOnlyHint {
			t.Fatalf("%s must be annotated read-only", tools[index].Tool.Name)
		}
	}
	for _, index := range []int{1, 2, 3, 5} {
		tool := tools[index]
		if tool.Tool.Annotations.ReadOnlyHint == nil || *tool.Tool.Annotations.ReadOnlyHint {
			t.Fatalf("%s must be annotated as mutating", tool.Tool.Name)
		}
	}
	for _, index := range []int{1, 2, 5} {
		if tools[index].Tool.Annotations.DestructiveHint == nil || !*tools[index].Tool.Annotations.DestructiveHint {
			t.Fatalf("%s must warn clients before changing the Farm", tools[index].Tool.Name)
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
