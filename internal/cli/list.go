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

			// Show any installed connectors not in catalog
			for _, name := range installed {
				if registry.LookupCatalog(name) == nil {
					fmt.Printf("  (unknown) %s\n", name)
				}
			}

			return nil
		},
	}
}
