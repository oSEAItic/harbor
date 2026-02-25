package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/schema"
	"github.com/spf13/cobra"
)

func newSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage learned proxy schemas",
	}

	cmd.AddCommand(
		newSchemaHistoryCmd(),
		newSchemaRollbackCmd(),
	)
	return cmd
}

func newSchemaHistoryCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "history <tool_name>",
		Short: "Show learned schema version history for a tool",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a tool name, e.g.: harbor schema history list_issues")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := strings.TrimSpace(args[0])
			store, err := schema.NewStore()
			if err != nil {
				return fmt.Errorf("opening schema store: %w", err)
			}

			history, err := store.History(toolName)
			if err != nil {
				return fmt.Errorf("reading history: %w", err)
			}
			if len(history) == 0 {
				return fmt.Errorf("no schema history for %q", toolName)
			}

			if asJSON {
				out, err := json.MarshalIndent(history, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling history: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Schema history for %s:\n", toolName)
			fmt.Fprintf(cmd.OutOrStdout(), "%-7s %-21s %-18s %-7s %s\n",
				"Ver", "LearnedAt", "Model", "Fields", "Template")
			fmt.Fprintf(cmd.OutOrStdout(), "%-7s %-21s %-18s %-7s %s\n",
				"-------", "---------------------", "------------------", "-------", "-------------------")

			for _, h := range history {
				learnedAt := h.LearnedAt.Format(time.RFC3339)
				if h.LearnedAt.IsZero() {
					learnedAt = "-"
				}
				model := h.LLMModel
				if model == "" {
					model = "-"
				}
				template := h.SummaryTemplate
				if len(template) > 40 {
					template = template[:37] + "..."
				}

				fmt.Fprintf(cmd.OutOrStdout(), "%-7d %-21s %-18s %-7d %s\n",
					h.Version, learnedAt, model, len(h.SummaryFields), template)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Print JSON output")
	return cmd
}

func newSchemaRollbackCmd() *cobra.Command {
	var (
		toVersion int
		asJSON    bool
	)

	cmd := &cobra.Command{
		Use:   "rollback <tool_name>",
		Short: "Rollback active schema for a tool to a previous version",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a tool name, e.g.: harbor schema rollback list_issues")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := strings.TrimSpace(args[0])
			store, err := schema.NewStore()
			if err != nil {
				return fmt.Errorf("opening schema store: %w", err)
			}

			rolled, err := store.Rollback(toolName, toVersion)
			if err != nil {
				return fmt.Errorf("rollback: %w", err)
			}

			if asJSON {
				out, err := json.MarshalIndent(rolled, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling schema: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Rolled back schema for %s to new active version %d.\n", toolName, rolled.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "Fields: %s\n", strings.Join(rolled.SummaryFields, ", "))
			return nil
		},
	}

	cmd.Flags().IntVar(&toVersion, "to", 0, "Target historical version (default: previous version)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print JSON output")
	return cmd
}
