package cli

import (
	"fmt"

	"github.com/oseaitic/harbor/internal/registry"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <connector>",
		Short: "Install a connector from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			fmt.Printf("Installing connector: %s\n", name)

			if err := registry.Install(name); err != nil {
				return fmt.Errorf("install %s: %w", name, err)
			}

			fmt.Printf("Connector %s installed successfully.\n", name)
			return nil
		},
	}
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <connector>",
		Short: "Remove an installed connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			fmt.Printf("Uninstalling connector: %s\n", name)

			if err := registry.Uninstall(name); err != nil {
				return fmt.Errorf("uninstall %s: %w", name, err)
			}

			fmt.Printf("Connector %s removed.\n", name)
			return nil
		},
	}
}
