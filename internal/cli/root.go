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
		Run: func(cmd *cobra.Command, args []string) {
			printBanner(version)
			cmd.Help()
		},
	}

	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "pretty", "Output format: pretty | json")

	root.AddCommand(
		newVersionCmd(version),
		newListCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newAuthCmd(),
		newDoctorCmd(),
		newCapabilitiesCmd(version),
		newAgentCmd(version),
		newGetCmd(outputFormat),
		newRawCmd(),
		newToolsCmd(),
		newMetricsCmd(),
		newSchemaCmd(),
		newRecallCmd(),
		newScaffoldCmd(),
		newBuildCmd(),
		newMCPCmd(version),
		newProxyCmd(version),
	)

	return root
}
