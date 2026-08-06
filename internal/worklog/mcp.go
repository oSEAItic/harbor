package worklog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/oseaitic/harbor/internal/gitevidence"
)

type MCPTool struct {
	Tool    mcp.Tool
	Handler server.ToolHandlerFunc
}

func MCPTools() []MCPTool {
	return []MCPTool{
		{Tool: featureContextTool(), Handler: makeFeatureContextHandler()},
		{Tool: checkpointFinalizeTool(), Handler: makeCheckpointFinalizeHandler()},
	}
}

func featureContextTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_feature_context",
		"Resolve the current Harbor Feature using explicit session binding, then repository binding, then a unique active project Feature. Never creates or guesses a Feature.",
		json.RawMessage(`{
			"type":"object",
			"properties":{
				"session_id":{"type":"string"},
				"repo_path":{"type":"string"},
				"branch":{"type":"string"},
				"project":{"type":"string"}
			},
			"additionalProperties":false
		}`),
	)
	tool.Annotations = mcp.ToolAnnotation{ReadOnlyHint: mcp.ToBoolPtr(true), DestructiveHint: mcp.ToBoolPtr(false), IdempotentHint: mcp.ToBoolPtr(true), OpenWorldHint: mcp.ToBoolPtr(false)}
	return tool
}

func checkpointFinalizeTool() mcp.Tool {
	tool := mcp.NewToolWithRawSchema(
		"harbor_checkpoint_finalize",
		"Persist an Agent-authored outcome summary for a validated Git commit range. Git remains the code-history source of truth; Harbor stores the semantic summary and provenance.",
		json.RawMessage(`{
			"type":"object",
			"properties":{
				"feature_id":{"type":"string"},
				"repo_path":{"type":"string"},
				"base_sha":{"type":"string","pattern":"^[0-9a-fA-F]{7,64}$"},
				"head_sha":{"type":"string","pattern":"^[0-9a-fA-F]{7,64}$"},
				"outcome":{"type":"string"},
				"decisions":{"type":"array","items":{"type":"string"}},
				"verification":{"type":"array","items":{"type":"string"}},
				"remaining":{"type":"array","items":{"type":"string"}},
				"session_id":{"type":"string"},
				"source":{"type":"string"},
				"model_name":{"type":"string"}
			},
			"required":["feature_id","repo_path","base_sha","head_sha","outcome"],
			"additionalProperties":false
		}`),
	)
	tool.Annotations = mcp.ToolAnnotation{ReadOnlyHint: mcp.ToBoolPtr(false), DestructiveHint: mcp.ToBoolPtr(false), IdempotentHint: mcp.ToBoolPtr(true), OpenWorldHint: mcp.ToBoolPtr(false)}
	return tool
}

func makeFeatureContextHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		repoPath := req.GetString("repo_path", "")
		if repoPath == "" {
			repoPath, _ = os.Getwd()
		}
		project := req.GetString("project", "")
		if project == "" && repoPath != "" {
			project = filepath.Base(filepath.Clean(repoPath))
		}
		store, err := NewStore()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer store.Close()
		featureContext, err := store.ResolveFeatureContext(ctx, req.GetString("session_id", ""), repoPath, req.GetString("branch", ""), project)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return structuredResult(featureContext)
	}
}

func makeCheckpointFinalizeHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resolved, err := gitevidence.Resolve(req.GetString("repo_path", ""), req.GetString("base_sha", ""), req.GetString("head_sha", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		store, err := NewStore()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer store.Close()
		summary, err := store.UpsertCheckpointSummary(ctx, CheckpointSummary{
			FeatureID: req.GetString("feature_id", ""), RepoPath: resolved.RepoPath,
			BaseSHA: resolved.BaseSHA, HeadSHA: resolved.HeadSHA, Outcome: req.GetString("outcome", ""),
			Decisions: stringArrayArgument(req, "decisions"), Verification: stringArrayArgument(req, "verification"),
			Remaining: stringArrayArgument(req, "remaining"), SessionID: req.GetString("session_id", ""),
			Source: req.GetString("source", ""), ModelName: req.GetString("model_name", ""), SchemaVersion: 1,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return structuredResult(summary)
	}
}

func stringArrayArgument(req mcp.CallToolRequest, name string) []string {
	value, ok := req.GetArguments()[name]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		if strings, ok := value.([]string); ok {
			return strings
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func structuredResult(value any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encoding Harbor worklog result: %v", err)), nil
	}
	result := mcp.NewToolResultText(string(data))
	result.StructuredContent = value
	return result, nil
}
