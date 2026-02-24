package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run Harbor environment diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := buildDoctorReport()
			if asJSON {
				out, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling doctor report: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Harbor home: %s\n", report.HarborHome)
			fmt.Fprintf(cmd.OutOrStdout(), "Connectors dir: %s (writable=%t)\n", report.ConnectorsDir, report.ConnectorsDirWritable)
			if report.NodeFound {
				fmt.Fprintf(cmd.OutOrStdout(), "Node: found (%s)\n", report.NodePath)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Node: not found")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed connectors: %d\n", len(report.InstalledConnectors))

			for _, c := range report.Connectors {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"  - %s: exec=%t describe=%t resources=%d auth=%t\n",
					c.Name,
					c.Executable,
					c.DescribeOK,
					c.ResourceCount,
					c.AuthConfigured,
				)
			}

			if len(report.Issues) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nIssues:")
				for _, issue := range report.Issues {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", issue)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output machine-readable JSON")
	return cmd
}
