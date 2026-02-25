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

── Connector Development Workflow ──────────────────────────────

  Step 1:  harbor scaffold <name>
           Creates connectors/<name>/ with src/index.ts, package.json, tsconfig.json

  Step 2:  Edit connectors/<name>/src/index.ts
           • Set BASE_URL to your API endpoint
           • Define toolSchemas array (name, description, parameters, summary_fields)
           • Implement async fetch handlers returning { data: T[], raw: unknown }

  Step 3:  harbor build <name>
           Runs npm install + esbuild bundle + validates tool schemas via --describe

  Step 4:  harbor build <name> --install
           Installs the bundled connector to ~/.harbor/connectors/

  Step 5:  harbor get <name>.<resource> --param key=value
           Test the connector end-to-end

── SDK Imports (from "harbor-sdk") ─────────────────────────────

  parseArgs        Parse CLI args into { resource, params, auth, raw }
  output           Write JSON response envelope to stdout
  buildMeta        Create metadata { source, connector_version, schema, fetched_at, request_id }
  errorResponse    Create error envelope with code + message
  handleDescribe   Return true and print toolSchemas when --describe flag is set
  execCLI          Shell out to external CLIs (e.g. psql, curl) with timeout

── Response Format ─────────────────────────────────────────────

  Connector stdout must be a JSON envelope:
  {
    "data":   [ ... ],
    "meta":   { "source", "connector_version", "schema", "fetched_at", "request_id" },
    "errors": [],
    "raw":    { ...original API response... }
  }

── Tips ────────────────────────────────────────────────────────

  • Each handler returns { data: array, raw: original_response }
  • summary_fields controls compressed output — pick 3-6 most important fields
  • Use HARBOR_AUTH env var for API keys, set via: harbor auth <name>
  • harbor build validates schema format automatically
  • Add contract.tests.json for automated testing

── Example ─────────────────────────────────────────────────────

  harbor scaffold newsapi
  → Created connectors/newsapi/
  →   src/index.ts    (boilerplate — fill in handlers)
  →   package.json
  →   tsconfig.json`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a connector name, e.g.: harbor scaffold newsapi")
			}
			return nil
		},
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
