package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oseaitic/harbor/internal/generator"
	"github.com/spf13/cobra"
)

func newScaffoldCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scaffold <name>",
		Short: "Create a new connector scaffold",
		Long: `Create a new connector directory with boilerplate code.

This generates the directory structure and TypeScript boilerplate
for a new Harbor connector. An agent or developer then fills in the
API logic and resource handlers.

See docs/CONNECTOR_SPEC.md for the full connector specification.

Example:
  harbor scaffold newsapi
  → Created connectors/newsapi/
  →   src/index.ts    (boilerplate — fill in handlers)
  →   package.json
  →   tsconfig.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Find the connectors directory relative to working dir
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			connectorDir := filepath.Join(cwd, "connectors")

			// Create connectors dir if it doesn't exist
			if err := os.MkdirAll(connectorDir, 0o755); err != nil {
				return fmt.Errorf("creating connectors dir: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Creating connector scaffold: %s\n", name)

			if err := generator.Scaffold(name, connectorDir); err != nil {
				return fmt.Errorf("scaffolding %s: %w", name, err)
			}

			dir := filepath.Join(connectorDir, name)
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s/\n", dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  src/index.ts    (boilerplate — fill in handlers)\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  package.json\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  tsconfig.json\n")
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  1. Edit %s/src/index.ts with your API logic\n", dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  2. Run: harbor build %s --install\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "  3. Try: harbor get %s.<resource>\n", name)

			return nil
		},
	}
}
