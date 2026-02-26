package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/spf13/cobra"
)

const defaultEndpoint = "https://api.harbor.dev"

func newLoginCmd() *cobra.Command {
	var (
		endpoint string
		key      string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Harbor Cloud",
		Long: `Log in to Harbor Cloud to run connectors remotely.

After logging in, commands like 'harbor get' will execute through the
cloud by default. Use --local to force local execution.

Examples:
  harbor login                                         # Interactive
  harbor login --endpoint https://api.harbor.dev       # Set endpoint, prompt for key
  harbor login --endpoint https://... --key sk-...     # Non-interactive`,
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			if endpoint == "" {
				fmt.Printf("Endpoint [%s]: ", defaultEndpoint)
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				endpoint = strings.TrimSpace(line)
				if endpoint == "" {
					endpoint = defaultEndpoint
				}
			}

			if key == "" {
				fmt.Print("API Key: ")
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				key = strings.TrimSpace(line)
			}

			if key == "" {
				return fmt.Errorf("API key is required")
			}

			cfg := cloudauth.Config{
				Endpoint: strings.TrimRight(endpoint, "/"),
				APIKey:   key,
			}

			if err := cloudauth.Save(cfg); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}

			fmt.Printf("Logged in to %s\n", cfg.Endpoint)
			return nil
		},
	}

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Harbor Cloud endpoint URL")
	cmd.Flags().StringVar(&key, "key", "", "API key")

	return cmd
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out from Harbor Cloud",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cloudauth.IsLoggedIn() {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in.")
				return nil
			}

			if err := cloudauth.Delete(); err != nil {
				return fmt.Errorf("removing credentials: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}
