package cli

import (
	"encoding/json"
	"fmt"

	"github.com/oseaitic/harbor/internal/connector"
	"github.com/spf13/cobra"
)

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Tool schema utilities for LLM/agent integration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "export",
		Short: "Export OpenAI-compatible tool schemas for all installed connectors",
		RunE: func(cmd *cobra.Command, args []string) error {
			schemas, err := connector.ExportToolSchemas()
			if err != nil {
				return fmt.Errorf("exporting tool schemas: %w", err)
			}

			out, err := json.MarshalIndent(schemas, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling schemas: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	})

	return cmd
}
