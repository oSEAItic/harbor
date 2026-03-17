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
	var conn string
	var refs []string

	cmd := &cobra.Command{
		Use:   "remember <topic> <note>",
		Short: "Save analysis conclusions by topic for future sessions",
		Long: `Save your analysis conclusions to Harbor memory, organized by topic.

Notes are organized by topic (e.g. "websocket-bugs", "billing-logic") and
optionally scoped to a connector. This will appear as 'context' in future
sessions — across sessions, devices, and agents.

Before writing, consider summarising:
  - What you analyzed and why
  - Patterns or anomalies you found
  - Conclusions you reached
  - Recommendations you made

Example:
  harbor remember market-trends "BTC dominance rising. SOL underperforming."
  harbor remember --connector kuse-hive ws-reconnect "Root cause is stale token..."
  harbor remember --author "Claude Code" billing "Token amount always 1..."`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := args[0]
			note := strings.Join(args[1:], " ")

			store, err := memory.NewStore()
			if err != nil {
				return fmt.Errorf("opening memory store: %w", err)
			}

			id, err := store.SaveNote(conn, topic, note, author)
			if err != nil {
				return fmt.Errorf("saving note: %w", err)
			}

			// Record reference edges in the knowledge graph.
			if len(refs) > 0 {
				memory.AddRefEdges(store.Dir(), id, refs)
			}

			// Best-effort cloud push with topic/session/connector metadata.
			cfg, cfgErr := cloudauth.Load()
			// If no cloud config and not opted out, offer auto-provision (once).
			if cfgErr != nil && !cloudauth.IsOptedOut() && !cloudauth.IsLoggedIn() {
				fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] Enable cross-device memory sync? (free, 50 memories)\n")
				fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] Run 'harbor cloud enable' to opt in, or 'harbor cloud disable' to never ask again.\n")
			}
			if cfgErr == nil {
				key := conn + "." + topic + "." + id
				// Get session ID from the saved object for cloud sync.
				var sessionID string
				if obj, getErr := store.Get(id); getErr == nil {
					sessionID = obj.SessionID
				}
				if pushErr := cloudauth.PushMemoryFull(key, note, author, topic, sessionID, conn, cfg); pushErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] cloud sync failed for %s: %v\n", key, pushErr)
				}
				// Push ref edges to cloud.
				if len(refs) > 0 {
					var edges []cloudauth.GraphEdge
					for _, ref := range refs {
						edges = append(edges, cloudauth.GraphEdge{From: id, To: ref, Kind: "ref"})
					}
					if pushErr := cloudauth.PushEdges(edges, cfg); pushErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] edge sync failed: %v\n", pushErr)
					}
				}
			}

			label := fmt.Sprintf("topic=%q", topic)
			if conn != "" {
				label += fmt.Sprintf(" connector=%q", conn)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved note (%s, %s)\n", label, id)
			if len(refs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "References: %s\n", strings.Join(refs, ", "))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "This will appear as context in future sessions.\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&author, "author", "", "Agent/model name (e.g. 'Claude Code', 'Gemini')")
	cmd.Flags().StringVar(&conn, "connector", "", "Scope note to a specific connector (optional)")
	cmd.Flags().StringSliceVar(&refs, "refs", nil, "Memory IDs this note references (comma-separated)")
	return cmd
}
