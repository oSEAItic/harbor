package cli

import (
	"fmt"

	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/registry"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed and available connectors",
		RunE: func(cmd *cobra.Command, args []string) error {
			installed, err := connector.ListInstalled()
			if err != nil {
				return fmt.Errorf("listing installed connectors: %w", err)
			}

			installedSet := make(map[string]bool)
			for _, name := range installed {
				installedSet[name] = true
			}

			fmt.Println("Connectors:")
			catalog := registry.ListCatalog()
			for _, entry := range catalog {
				status := "  "
				if installedSet[entry.ID] {
					status = "* "
				}
				fmt.Printf("  %s%-12s  %s  (%s)\n", status, entry.ID, entry.Description, entry.Version)
			}

			if len(catalog) == 0 {
				fmt.Println("  (catalog is empty)")
			}

			fmt.Println()
			fmt.Println("  * = installed")

			// Show installed connectors not in catalog — introspect via --describe
			for _, name := range installed {
				if registry.LookupCatalog(name) == nil {
					schemas, err := connector.ExportConnectorToolSchemas(name)
					if err == nil && len(schemas) > 0 {
						desc := schemas[0].Function.Description
						fmt.Printf("  * %-12s  %s  (local)\n", name, desc)
					} else {
						fmt.Printf("  * %-12s  (no description)  (local)\n", name)
					}
				}
			}

			return nil
		},
	}
}
