package cli

import (
	"fmt"

	"github.com/oseaitic/harbor/internal/registry"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	var fromPath string
	var checksum string

	cmd := &cobra.Command{
		Use:   "install <connector>",
		Short: "Install a connector from the catalog or a local bundle",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a connector name, e.g.: harbor install coingecko")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			fmt.Printf("Installing connector: %s\n", name)

			var err error
			if fromPath != "" {
				fmt.Printf("  source: %s (local)\n", fromPath)
				err = registry.InstallFromLocal(name, fromPath, registry.InstallOptions{
					ExpectedSHA256: checksum,
				})
			} else {
				err = registry.Install(name, registry.InstallOptions{
					ExpectedSHA256: checksum,
				})
			}
			if err != nil {
				return fmt.Errorf("install %s: %w", name, err)
			}

			fmt.Printf("Connector %s installed successfully.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromPath, "from", "", "Install from a local bundle file instead of downloading")
	cmd.Flags().StringVar(&checksum, "sha256", "", "Expected SHA-256 for the connector bundle (hex)")

	return cmd
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <connector>",
		Short: "Remove an installed connector",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a connector name, e.g.: harbor uninstall coingecko")
			}
			return nil
		},
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
