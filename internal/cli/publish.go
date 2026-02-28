package cli

import (
	"fmt"
	"os"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/registry"
	"github.com/spf13/cobra"
)

func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish <connector>",
		Short: "Publish an installed connector to your Harbor Cloud registry",
		Long: `Upload a locally installed connector bundle and its schema to Harbor Cloud.

Once published, the connector's schema will appear in tools/list for any agent
connecting to your Harbor Cloud MCP endpoint. You can also install it on
another machine with 'harbor install <connector>'.

The connector must already be installed locally. Run 'harbor build --install'
first if you are publishing a connector built from source.

Examples:
  harbor publish myconnector           # Publish ~/.harbor/connectors/myconnector
  harbor build myapi --install         # Build first
  harbor publish myapi                 # Then publish`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := connector.ValidateConnectorName(name); err != nil {
				return err
			}

			// Check the connector is installed locally
			binPath := connector.ConnectorPath(name)
			if _, err := os.Stat(binPath); os.IsNotExist(err) {
				return fmt.Errorf(
					"connector %q is not installed\nRun 'harbor build %s --install' first, then retry",
					name, name)
			}

			// Require cloud login
			cfg, err := cloudauth.Load()
			if err != nil {
				return fmt.Errorf("not logged in to Harbor Cloud\nRun 'harbor login' first")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Publishing %s to %s...\n", name, cfg.Endpoint)

			sha256, size, err := registry.PublishToCloud(name, cfg)
			if err != nil {
				return fmt.Errorf("publish failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"Published %s (%s, sha256: %s)\n",
				name, formatBytes(size), sha256[:16]+"...")
			fmt.Fprintf(cmd.OutOrStdout(),
				"  schema visible in Claude Chat tools/list\n")
			fmt.Fprintf(cmd.OutOrStdout(),
				"  install on another machine: harbor install %s\n", name)
			return nil
		},
	}
}

func formatBytes(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}
