package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/oseaitic/harbor/internal/proxy"
	"github.com/oseaitic/harbor/internal/schema"
)

func newProxyCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "proxy [--credential ENV=KEY]... <upstream-command> [args...]",
		Short: "Transparent MCP proxy with schema learning",
		Long: `Start a transparent MCP proxy that wraps any upstream MCP server.

Harbor discovers the upstream server's tools, re-registers them on its own
MCP server, and proxies tool calls. On first use of each tool the raw output
is returned with a hint asking the agent to call harbor_learn_schema. Once
taught, all future calls are automatically compressed and cached.

Credential injection (optional):
  --credential ENV_VAR=keychain_key
  Reads the secret from the OS keychain and injects it as ENV_VAR into the
  upstream process. Supports multiple --credential flags. Store credentials
  first with: harbor auth <keychain_key>

Examples:
  # Basic proxy (no credentials needed)
  harbor proxy npx @modelcontextprotocol/server-filesystem /tmp

  # With credential injection from OS keychain
  harbor proxy --credential GITHUB_TOKEN=github-pat \
    npx @modelcontextprotocol/server-github

  # Claude Desktop / Claude Code config (.mcp.json)
  {
    "mcpServers": {
      "github": {
        "command": "harbor",
        "args": [
          "proxy",
          "--credential", "GITHUB_TOKEN=github-pat",
          "npx", "@modelcontextprotocol/server-github"
        ]
      }
    }
  }`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			credentials, args := extractCredentialFlags(args)

			if len(args) == 0 {
				return fmt.Errorf("upstream command is required")
			}

			schemaStore, err := schema.NewStore()
			if err != nil {
				return fmt.Errorf("schema store: %w", err)
			}

			return proxy.Run(proxy.Config{
				Command:     args[0],
				Args:        args[1:],
				Version:     version,
				SchemaStore: schemaStore,
				Credentials: credentials,
			})
		},
	}
}

// extractCredentialFlags manually parses --credential flags from args since
// DisableFlagParsing is true (to pass remaining args through to upstream).
// Each credential is in the format ENV_VAR=keychain_key.
func extractCredentialFlags(args []string) (map[string]string, []string) {
	credentials := make(map[string]string)
	var remaining []string

	i := 0
	for i < len(args) {
		if args[i] == "--credential" && i+1 < len(args) {
			parts := strings.SplitN(args[i+1], "=", 2)
			if len(parts) == 2 {
				credentials[parts[0]] = parts[1]
			}
			i += 2
		} else {
			remaining = append(remaining, args[i])
			i++
		}
	}
	return credentials, remaining
}
