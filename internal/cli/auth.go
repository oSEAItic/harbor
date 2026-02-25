package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth [connector]",
		Short: "Manage connector credentials (stored in OS keychain)",
		Long: `Manage authentication credentials for connectors.

Credentials are stored securely in your OS keychain (macOS Keychain,
Linux secret-service, Windows Credential Manager).

Without arguments, lists all stored credentials.

Examples:
  harbor auth                     # List all stored credentials
  harbor auth github-mcp          # Store API key for github-mcp
  harbor auth status              # List all stored credentials
  harbor auth status github-mcp   # Check if credentials exist
  harbor auth delete github-mcp   # Remove stored credentials`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// No args → list all credentials
			if len(args) == 0 {
				return authList(cmd)
			}

			name := args[0]
			reader := bufio.NewReader(os.Stdin)

			fmt.Printf("Configuring auth for: %s\n", name)
			fmt.Print("API Key: ")
			key, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			key = strings.TrimSpace(key)

			if err := auth.Store(name, key); err != nil {
				return fmt.Errorf("storing credentials: %w", err)
			}

			fmt.Printf("Credentials stored for %s.\n", name)
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "status [connector]",
		Short: "Check credential status (all or specific connector)",
		Example: `  harbor auth status              # List all
  harbor auth status github-mcp   # Check specific`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return authList(cmd)
			}
			name := args[0]
			_, err := auth.Retrieve(name)
			if err != nil {
				fmt.Printf("%s: not configured\n", name)
				return nil
			}
			fmt.Printf("%s: configured\n", name)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <connector>",
		Short: "Remove stored credentials for a connector",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a connector name, e.g.: harbor auth delete github-mcp")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := auth.Delete(name); err != nil {
				return fmt.Errorf("deleting credentials for %s: %w", name, err)
			}
			fmt.Printf("Credentials removed for %s.\n", name)
			return nil
		},
	})

	return cmd
}

func authList(cmd *cobra.Command) error {
	entries := auth.List()
	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No credentials stored.")
		fmt.Fprintln(cmd.OutOrStdout(), "\nStore one with: harbor auth <name>")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%d credential(s) stored:\n\n", len(entries))
	for _, e := range entries {
		age := formatDuration(time.Since(e.CreatedAt))
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  (stored %s ago)\n", e.Name, age)
	}
	return nil
}
