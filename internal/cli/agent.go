package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newAgentCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent integration utilities",
	}

	cmd.AddCommand(newAgentBootstrapCmd(version))
	cmd.AddCommand(newCapabilitiesCmd(version))
	return cmd
}

func newAgentBootstrapCmd(version string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Emit a full machine-readable Harbor bootstrap payload for agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := buildAgentBootstrapReport(version)
			if asJSON {
				out, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling bootstrap report: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Agent bootstrap ready.")
			fmt.Fprintln(cmd.OutOrStdout(), "Recommended integration: harbor mcp")
			fmt.Fprintln(cmd.OutOrStdout(), "Run `harbor agent bootstrap --json` for full machine-readable output.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output machine-readable JSON")
	return cmd
}
