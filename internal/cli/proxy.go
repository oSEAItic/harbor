package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/oseaitic/harbor/internal/proxy"
	"github.com/oseaitic/harbor/internal/schema"
)

func newProxyCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "proxy",
		Short: "Transparent MCP proxy with schema learning",
		Long: `Start a transparent MCP proxy that wraps any upstream MCP server.

Harbor discovers the upstream server's tools, re-registers them on its own
MCP server, and proxies tool calls. On first use of each tool the raw output
is returned with a hint asking the agent to call harbor_learn_schema. Once
taught, all future calls are automatically compressed and cached.

No API keys or extra configuration required — the connected agent does the
schema learning itself.

Usage with Claude Desktop:
  {
    "mcpServers": {
      "notion": {
        "command": "harbor",
        "args": ["proxy", "notion-mcp-server"]
      }
    }
  }`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaStore, err := schema.NewStore()
			if err != nil {
				return fmt.Errorf("schema store: %w", err)
			}

			return proxy.Run(proxy.Config{
				Command:     args[0],
				Args:        args[1:],
				Version:     version,
				SchemaStore: schemaStore,
			})
		},
	}
}
