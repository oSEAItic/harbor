package cli

import (
	"github.com/oseaitic/harbor/internal/mcpserver"
	"github.com/spf13/cobra"
)

func newMCPCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server (stdio transport)",
		Long: `Start a Model Context Protocol server over stdio.

All installed connectors are automatically exposed as MCP tools.
Tool calls go through the full pipeline: execute → compile → memory.

Usage with Claude Desktop:
  {
    "mcpServers": {
      "harbor": { "command": "harbor", "args": ["mcp"] }
    }
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mcpserver.Serve(version)
		},
	}
}
