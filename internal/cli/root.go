package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the top-level harbor command.
func NewRootCmd(version string, sourceDir ...string) *cobra.Command {
	var srcDir string
	if len(sourceDir) > 0 {
		srcDir = sourceDir[0]
	}
	var outputFormat string

	root := &cobra.Command{
		Use:   "harbor",
		Short: "Fetch, normalize, and compress external data for LLMs",
		Long: `Harbor — Fetch, normalize, and compress external data for LLMs.

Quick start:
  harbor list                          See available connectors
  harbor install coingecko             Install a connector
  harbor get coingecko.trending        Fetch data
  harbor mcp                           Start MCP server for agents`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			printBanner(version)
			cmd.Help()
		},
	}

	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "pretty", "Output format: pretty | json")

	// ── Command groups (gh-style) ────────────────────────────────
	root.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "develop", Title: "Develop Commands:"},
		&cobra.Group{ID: "advanced", Title: "Advanced Commands:"},
		&cobra.Group{ID: "help", Title: "Help Commands:"},
	)

	root.SetHelpCommandGroupID("help")
	root.SetCompletionCommandGroupID("help")

	// ── Core ─────────────────────────────────────────────────────
	addCmd(root, "core",
		newGetCmd(outputFormat),
		newListCmd(),
		newInstallCmd(),
		newMCPCmd(version),
		newAuthCmd(),
		newLoginCmd(),
		newLogoutCmd(),
	)

	// ── Develop ──────────────────────────────────────────────────
	addCmd(root, "develop",
		newScaffoldCmd(),
		newBuildCmd(),
		newUninstallCmd(),
		newPublishCmd(),
	)

	// ── Advanced ─────────────────────────────────────────────────
	addCmd(root, "advanced",
		newProxyCmd(version),
		newRawCmd(),
		newToolsCmd(),
		newRecallCmd(),
		newSchemaCmd(),
		newAgentCmd(version),
		newMetricsCmd(),
		newDoctorCmd(),
		newUpdateCmd(version, srcDir),
	)

	// ── Help ─────────────────────────────────────────────────────
	addCmd(root, "help",
		newVersionCmd(version),
	)

	return root
}

// addCmd sets GroupID on each command and adds them to the parent.
func addCmd(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.GroupID = groupID
		parent.AddCommand(cmd)
	}
}
