package farm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPToolsExposeSharedFarmWithMutationAnnotations(t *testing.T) {
	tools := mcpTools(func() (*Client, error) { return nil, errors.New("not connected") })
	if len(tools) != 7 {
		t.Fatalf("got %d tools, want 7", len(tools))
	}
	want := []string{"harbor_farm_open", "harbor_farm_status", "harbor_farm_plant", "harbor_farm_harvest", "harbor_farm_connect", "harbor_farm_visit", "harbor_farm_forage"}
	for i, name := range want {
		if tools[i].Tool.Name != name {
			t.Fatalf("tool %d = %q, want %q", i, tools[i].Tool.Name, name)
		}
	}
	for _, index := range []int{0, 1, 5} {
		if tools[index].Tool.Annotations.ReadOnlyHint == nil || !*tools[index].Tool.Annotations.ReadOnlyHint {
			t.Fatalf("%s must be annotated read-only", tools[index].Tool.Name)
		}
	}
	for _, index := range []int{2, 3, 4, 6} {
		tool := tools[index]
		if tool.Tool.Annotations.ReadOnlyHint == nil || *tool.Tool.Annotations.ReadOnlyHint {
			t.Fatalf("%s must be annotated as mutating", tool.Tool.Name)
		}
	}
	for _, index := range []int{2, 3, 6} {
		if tools[index].Tool.Annotations.DestructiveHint == nil || !*tools[index].Tool.Annotations.DestructiveHint {
			t.Fatalf("%s must warn clients before changing the Farm", tools[index].Tool.Name)
		}
	}
	if tools[0].Tool.Meta == nil || tools[0].Tool.Meta.AdditionalFields["openai/outputTemplate"] != farmWidgetURI {
		t.Fatalf("Farm open tool does not point at the widget: %#v", tools[0].Tool.Meta)
	}
}

func TestMCPFarmWidgetUsesPortableAppsResourceAndDirectTools(t *testing.T) {
	resources := MCPResources()
	if len(resources) != 1 || resources[0].Resource.URI != farmWidgetURI {
		t.Fatalf("unexpected Farm resources: %#v", resources)
	}
	if resources[0].Resource.MIMEType != "text/html;profile=mcp-app" {
		t.Fatalf("Farm resource MIME type = %q", resources[0].Resource.MIMEType)
	}
	contents, err := resources[0].Handler(context.Background(), mcp.ReadResourceRequest{})
	if err != nil || len(contents) != 1 {
		t.Fatalf("Farm widget contents = %#v, err = %v", contents, err)
	}
	html, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("Farm widget resource type = %T", contents[0])
	}
	for _, required := range []string{"tools/call", "harbor_farm_plant", "harbor_farm_harvest", "harbor_farm_visit", "prefers-reduced-motion"} {
		if !strings.Contains(html.Text, required) {
			t.Fatalf("Farm widget is missing %q", required)
		}
	}
	for _, required := range []string{"IntersectionObserver", "visibilitychange", "function updateTimers()", "app.addEventListener(\"click\""} {
		if !strings.Contains(html.Text, required) {
			t.Fatalf("Farm widget is missing idle-performance guard %q", required)
		}
	}
	for _, forbidden := range []string{"window.setInterval", "function bind()"} {
		if strings.Contains(html.Text, forbidden) {
			t.Fatalf("Farm widget still contains full-render loop %q", forbidden)
		}
	}
	if got := strings.Count(html.Text, "app.innerHTML ="); got != 1 {
		t.Fatalf("Farm widget has %d whole-tree render sites, want 1", got)
	}
	if got := strings.Count(html.Text, "infinite"); got != 1 {
		t.Fatalf("Farm widget has %d infinite animations, want only the loading indicator", got)
	}
}

func TestMCPFarmWidgetStateOmitsAccountAndRawSessionIdentifiers(t *testing.T) {
	bootstrap := &Bootstrap{}
	bootstrap.Account.Email = "farmer@example.com"
	bootstrap.ActiveSessions = []AgentSession{{ExternalSessionID: "raw-session-id"}}

	state, err := farmWidgetState(bootstrap, nil)
	if err != nil {
		t.Fatalf("Farm widget state: %v", err)
	}
	if _, ok := state["account"]; ok {
		t.Fatal("Farm widget state must not expose account details")
	}
	if _, ok := state["active_sessions"]; ok {
		t.Fatal("Farm widget state must not expose raw session details")
	}
	if got := state["active_session_count"]; got != float64(1) && got != 1 {
		t.Fatalf("active_session_count = %#v, want 1", got)
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
	result, err := tools[2].Handler(context.Background(), plant)
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("invalid plant result = %#v, err = %v", result, err)
	}

	harvest := mcp.CallToolRequest{}
	harvest.Params.Arguments = map[string]any{"plot_index": float64(-1)}
	result, err = tools[3].Handler(context.Background(), harvest)
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("invalid harvest result = %#v, err = %v", result, err)
	}
	if factoryCalled {
		t.Fatal("invalid mutation reached Harbor Cloud")
	}
}
