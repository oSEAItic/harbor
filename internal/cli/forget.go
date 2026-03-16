package cli

import (
	"fmt"
	"strings"

	"github.com/oseaitic/harbor/internal/memory"
	"github.com/spf13/cobra"
)

func newForgetCmd() *cobra.Command {
	var (
		connector string
		topic     string
		confirm   bool
		permanent bool
	)

	cmd := &cobra.Command{
		Use:   "forget [id...]",
		Short: "Delete memory entries (soft-delete to trash by default)",
		Long: `Remove memory entries by ID, connector, or topic.

By default entries are moved to ~/.harbor/memory/trash/ and auto-cleaned
after 7 days. Use --permanent to delete immediately.

Examples:
  harbor forget mem_abc123                     # trash one entry
  harbor forget mem_abc123 mem_def456          # trash multiple
  harbor forget --connector kuse-hive --confirm  # trash all kuse-hive notes
  harbor forget --topic billing --confirm        # trash all billing notes
  harbor forget mem_abc123 --permanent           # permanent delete`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := memory.NewStore()
			if err != nil {
				return fmt.Errorf("opening memory store: %w", err)
			}

			trash := !permanent

			// Delete by ID
			if len(args) > 0 {
				for _, id := range args {
					entry, err := store.Delete(id, trash)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: %v\n", id, err)
						continue
					}
					action := "trashed"
					if permanent {
						action = "deleted"
					}
					label := entry.Connector
					if entry.Resource != "" && entry.Resource != "_context" {
						label += "/" + entry.Resource
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%s): %s\n", action, entry.ID, label, truncateStr(entry.Summary, 60))
				}
				return nil
			}

			// Delete by query (requires --confirm)
			if connector == "" && topic == "" {
				return fmt.Errorf("provide memory IDs or use --connector/--topic flags")
			}
			if !confirm {
				// Preview what would be deleted.
				opts := memory.QueryOptions{Limit: 50}
				if connector != "" {
					opts.Connector = connector
				}
				if topic != "" {
					opts.Resource = topic
				}
				results := store.Query(opts)
				noteResults := filterNotes(results)
				if len(noteResults) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No matching notes found.")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would delete %d notes:\n", len(noteResults))
				for _, e := range noteResults {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s [%s] %s\n", e.ID, e.Resource, truncateStr(e.Summary, 50))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nAdd --confirm to proceed.")
				return nil
			}

			opts := memory.QueryOptions{Limit: 100}
			if connector != "" {
				opts.Connector = connector
			}
			if topic != "" {
				opts.Resource = topic
			}
			// Only delete notes, not data fetches.
			all := store.Query(opts)
			var count int
			for _, e := range all {
				if e.Schema != memory.SchemaNote {
					continue
				}
				if _, err := store.Delete(e.ID, trash); err == nil {
					count++
				}
			}
			action := "trashed"
			if permanent {
				action = "deleted"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d notes", action, count)
			if connector != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (connector=%s)", connector)
			}
			if topic != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (topic=%s)", topic)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}

	cmd.Flags().StringVar(&connector, "connector", "", "Delete all notes for this connector")
	cmd.Flags().StringVar(&topic, "topic", "", "Delete all notes with this topic")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm bulk deletion")
	cmd.Flags().BoolVar(&permanent, "permanent", false, "Permanently delete (skip trash)")
	return cmd
}

func truncateStr(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func filterNotes(entries []memory.IndexEntry) []memory.IndexEntry {
	var notes []memory.IndexEntry
	for _, e := range entries {
		if e.Schema == memory.SchemaNote {
			notes = append(notes, e)
		}
	}
	return notes
}
