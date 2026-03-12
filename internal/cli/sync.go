package cli

import (
	"fmt"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync memories with Harbor Cloud (bidirectional)",
		Long: `Bidirectional memory sync with Harbor Cloud.

Pull: downloads cloud memories to local cache.
Push: uploads local memories not yet in cloud.

This is the same sync that runs silently during 'harbor recall',
but with verbose output so you can see what's happening.

Examples:
  harbor sync`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudauth.Load()
			if err != nil {
				return fmt.Errorf("not logged in — run 'harbor login' first")
			}

			w := cmd.OutOrStdout()
			errW := cmd.ErrOrStderr()

			// ── Pull: cloud → local ─────────────────────────────
			fmt.Fprintf(w, "Pulling from %s...\n", cfg.Endpoint)
			cloudNotes, pullErr := cloudauth.PullMemories(cfg)
			if pullErr != nil {
				fmt.Fprintf(errW, "  ✗ pull failed: %v\n", pullErr)
			} else {
				fmt.Fprintf(w, "  ✓ %d memory(ies) in cloud\n", len(cloudNotes))
				if len(cloudNotes) > 0 {
					ns := memory.NewNoteStore()
					local := make([]memory.Note, 0, len(cloudNotes))
					for _, n := range cloudNotes {
						local = append(local, memory.Note{Key: n.Key, Content: n.Content, UpdatedAt: n.UpdatedAt})
					}
					if err := ns.Save(local); err != nil {
						fmt.Fprintf(errW, "  ✗ saving local notes: %v\n", err)
					}
				}
			}

			// ── Push: local → cloud ─────────────────────────────
			fmt.Fprintln(w, "Pushing local memories to cloud...")
			cloudKeys := make(map[string]bool, len(cloudNotes))
			for _, n := range cloudNotes {
				cloudKeys[n.Key] = true
			}

			store, err := memory.NewStore()
			if err != nil {
				return fmt.Errorf("opening memory store: %w", err)
			}

			entries := store.Query(memory.QueryOptions{Limit: 200})
			pushed, skipped, failed := 0, 0, 0
			for _, e := range entries {
				key := e.Connector + "." + e.Resource + "." + e.ID
				if cloudKeys[key] {
					skipped++
					continue
				}
				content := e.Summary
				if content == "" {
					skipped++
					continue
				}
				if err := cloudauth.PushMemory(key, content, e.Author, cfg); err != nil {
					fmt.Fprintf(errW, "  ✗ push %s: %v\n", key, err)
					failed++
				} else {
					pushed++
				}
			}

			fmt.Fprintf(w, "  ✓ pushed %d, skipped %d (already in cloud or empty)", pushed, skipped)
			if failed > 0 {
				fmt.Fprintf(w, ", failed %d", failed)
			}
			fmt.Fprintln(w)

			return nil
		},
	}
}
