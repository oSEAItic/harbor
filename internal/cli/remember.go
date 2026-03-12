package cli

import (
	"fmt"
	"strings"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/spf13/cobra"
)

func newRememberCmd() *cobra.Command {
	var author string

	cmd := &cobra.Command{
		Use:   "remember <connector> <note>",
		Short: "Save analysis conclusions about a connector for future sessions",
		Long: `Save your analysis conclusions about a connector to Harbor memory.

Your note will appear as 'context' the next time any agent accesses this
connector — across sessions, devices, and agents.

Before writing, consider summarising:
  - What you analyzed and why
  - Patterns or anomalies you found
  - Conclusions you reached
  - Recommendations you made

Example:
  harbor remember coingecko "BTC dominance rising. SOL underperforming."
  harbor remember --author "Claude Code" coingecko "Market analysis..."`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			connector := args[0]
			note := strings.Join(args[1:], " ")

			store, err := memory.NewStore()
			if err != nil {
				return fmt.Errorf("opening memory store: %w", err)
			}

			id, err := store.SaveNote(connector, note, author)
			if err != nil {
				return fmt.Errorf("saving note: %w", err)
			}

			// Best-effort cloud push with author.
			if cfg, cfgErr := cloudauth.Load(); cfgErr == nil {
				key := connector + "._context." + id
				if pushErr := cloudauth.PushMemory(key, note, author, cfg); pushErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] cloud sync failed for %s: %v\n", key, pushErr)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Saved note for %q (%s)\n", connector, id)
			fmt.Fprintf(cmd.OutOrStdout(), "This will appear as context in future sessions with this connector.\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&author, "author", "", "Agent/model name (e.g. 'Claude Code', 'Gemini')")
	return cmd
}
