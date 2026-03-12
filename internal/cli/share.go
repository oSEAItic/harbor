package cli

import (
	"fmt"
	"strings"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/spf13/cobra"
)

func newShareCmd() *cobra.Command {
	var (
		ttl    string
		layer  string
		list   bool
		revoke string
	)

	cmd := &cobra.Command{
		Use:   "share <memory_id>",
		Short: "Create a temporary share link for a memory",
		Long: `Create a temporary, public share link for a memory.

The recipient can import the shared memory with 'harbor import <token>'.

Examples:
  harbor share mem_abc123                  Share with default TTL (24h)
  harbor share mem_abc123 --ttl 1h         Share for 1 hour
  harbor share mem_abc123 --layer compact  Share the compact layer
  harbor share --list                      List active shares
  harbor share --revoke <id>               Revoke a share`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudauth.Load()
			if err != nil {
				return fmt.Errorf("not logged in — run 'harbor login' first")
			}

			w := cmd.OutOrStdout()

			// --list mode
			if list {
				shares, err := cloudauth.ListShares(cfg)
				if err != nil {
					return fmt.Errorf("listing shares: %w", err)
				}
				if len(shares) == 0 {
					fmt.Fprintln(w, "No active shares.")
					return nil
				}
				fmt.Fprintf(w, "%d active share(s):\n\n", len(shares))
				fmt.Fprintf(w, "%-24s  %-10s  %-10s  %s\n",
					"Token", "Layer", "Accesses", "Expires")
				fmt.Fprintf(w, "%-24s  %-10s  %-10s  %s\n",
					"------------------------", "----------", "----------", "--------------------")
				for _, s := range shares {
					fmt.Fprintf(w, "%-24s  %-10s  %-10d  %s\n",
						s.ID, s.Layer, s.AccessCount, s.ExpiresAt)
				}
				return nil
			}

			// --revoke mode
			if revoke != "" {
				if err := cloudauth.RevokeShare(revoke, cfg); err != nil {
					return fmt.Errorf("revoking share: %w", err)
				}
				fmt.Fprintf(w, "Share %s revoked.\n", revoke)
				return nil
			}

			// Create share: requires a memory ID
			if len(args) == 0 {
				return fmt.Errorf("memory ID required (e.g. harbor share mem_abc123)")
			}
			memID := args[0]

			if !strings.HasPrefix(memID, "mem_") {
				return fmt.Errorf("expected a memory ID (mem_xxx), got %q", memID)
			}

			store, err := memory.NewStore()
			if err != nil {
				return fmt.Errorf("opening memory store: %w", err)
			}

			obj, err := store.Get(memID)
			if err != nil {
				return fmt.Errorf("memory %q not found: %w", memID, err)
			}

			// Build the cloud memory key
			key := obj.Connector + "." + obj.Resource + "." + obj.ID

			if ttl == "" {
				ttl = "24h"
			}

			result, err := cloudauth.CreateShare(key, layer, ttl, cfg)
			if err != nil {
				return fmt.Errorf("creating share: %w", err)
			}

			fmt.Fprintf(w, "Share created!\n\n")
			fmt.Fprintf(w, "  Token:   %s\n", result.Token)
			fmt.Fprintf(w, "  URL:     %s\n", result.URL)
			fmt.Fprintf(w, "  Expires: %s\n", result.ExpiresAt)
			fmt.Fprintf(w, "\nRecipient can run: harbor import %s\n", result.Token)
			return nil
		},
	}

	cmd.Flags().StringVar(&ttl, "ttl", "", "Share TTL (e.g. 1h, 24h, 7d) — default 24h")
	cmd.Flags().StringVar(&layer, "layer", "", "Layer to share (raw|normalized|compact|summary)")
	cmd.Flags().BoolVar(&list, "list", false, "List active shares")
	cmd.Flags().StringVar(&revoke, "revoke", "", "Revoke a share by ID")

	return cmd
}
