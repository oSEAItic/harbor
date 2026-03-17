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
		Short: "Context infrastructure for AI agents",
		Long: `Harbor — Context infrastructure for AI agents.

Every response includes curated fields (noise removed), source + timestamp
(so you can cite and check freshness), and related memories from past sessions.

Workflow:
  harbor get <connector.resource>      Fetch curated context
  harbor remember <connector> "note"   Save analysis for next session
  harbor recall --search "query"       Search past data and notes
  harbor mcp                           Start MCP server (all tools)`,
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
		newFetchCmd(),
		newRememberCmd(),
		newForgetCmd(),
		newListCmd(),
		newInstallCmd(),
		newMCPCmd(version),
		newAuthCmd(),
		newKeysCmd(),
		newRegisterCmd(),
		newLoginCmd(),
		newLogoutCmd(),
		newWhoamiCmd(),
		newSyncCmd(),
		newImportCmd(),
		newCloudCmd(),
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
		newShareCmd(),
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
