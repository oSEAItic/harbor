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

			fmt.Println("Installed connectors:")
			if len(installed) == 0 {
				fmt.Println("  (none)")
			}
			for _, c := range installed {
				fmt.Printf("  - %s\n", c)
			}

			fmt.Println()

			available, err := registry.ListAvailable()
			if err != nil {
				fmt.Println("Available connectors:")
				fmt.Println("  (could not fetch registry)")
				return nil
			}

			fmt.Println("Available connectors:")
			if len(available) == 0 {
				fmt.Println("  (none)")
			}
			for _, c := range available {
				fmt.Printf("  - %s\n", c.ID)
			}

			return nil
		},
	}
}
