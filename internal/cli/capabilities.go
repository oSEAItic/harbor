package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newCapabilitiesCmd(version string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Emit Harbor CLI capabilities for agent bootstrapping",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := buildCapabilitiesReport(version)
			if asJSON {
				out, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling capabilities: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Recommended integration: harbor mcp")
			fmt.Fprintln(cmd.OutOrStdout(), "\nCommand templates:")
			for _, c := range report.CommandTemplates {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", c.Name, c.Template)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nUse --json for full machine-readable output.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output machine-readable JSON")
	return cmd
}
