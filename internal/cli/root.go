package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the top-level harbor command.
func NewRootCmd(version string) *cobra.Command {
	var outputFormat string

	root := &cobra.Command{
		Use:   "harbor",
		Short: "Unified CLI for external data connectors",
		Long: `Harbor is a protocol-native CLI that standardizes access to external
data sources via connectors. Connectors expose normalized JSON output
with source provenance and schema versioning.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "pretty", "Output format: pretty | json")

	root.AddCommand(
		newVersionCmd(version),
		newListCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newAuthCmd(),
		newGetCmd(outputFormat),
		newRawCmd(),
		newToolsCmd(),
	)

	return root
}
