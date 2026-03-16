package cli

import (
	"fmt"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <share_token>",
		Short: "Import a shared memory into your local Harbor",
		Long: `Import a memory that was shared via 'harbor share'.

The share token looks like hbr_s_xxx and can be found in the share URL.

Examples:
  harbor import hbr_s_abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]

			// Use cloud endpoint from login config, or fall back to default.
			endpoint := defaultEndpoint
			if cfg, err := cloudauth.Load(); err == nil {
				endpoint = cfg.Endpoint
			}

			w := cmd.OutOrStdout()

			fmt.Fprintf(w, "Resolving share %s...\n", token)
			shared, err := cloudauth.ResolveShare(token, endpoint)
			if err != nil {
				return fmt.Errorf("resolving share: %w", err)
			}

			store, err := memory.NewStore()
			if err != nil {
				return fmt.Errorf("opening memory store: %w", err)
			}

			connector := shared.Connector
			if connector == "" {
				connector = "shared"
			}

			id, err := store.SaveNote(connector, "_context", shared.Content, shared.Author)
			if err != nil {
				return fmt.Errorf("saving imported memory: %w", err)
			}

			fmt.Fprintf(w, "Imported as %s (connector: %s)\n", id, connector)
			if shared.ExpiresIn != "" {
				fmt.Fprintf(w, "Original share expires in %s\n", shared.ExpiresIn)
			}
			return nil
		},
	}
}
