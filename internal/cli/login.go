package cli

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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
		Long: `Log in to Harbor Cloud using your API key.

After logging in, commands like 'harbor get' will execute through the
cloud by default. Use --local to force local execution.

Don't have an account? Sign up at https://harbor.oseaitic.com

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
			endpoint = strings.TrimRight(endpoint, "/")

			if key == "" {
				fmt.Print("API Key: ")
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				key = strings.TrimSpace(line)
			}

			if key == "" {
				return fmt.Errorf("API key is required\n\nDon't have an account? Sign up at https://harbor.oseaitic.com")
			}

			// Validate before saving
			fmt.Fprintf(cmd.OutOrStdout(), "Verifying credentials...")
			if err := validateAPIKey(endpoint, key); err != nil {
				fmt.Fprintln(cmd.OutOrStdout())
				return fmt.Errorf("login failed: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), " ok")

			cfg := cloudauth.Config{
				Endpoint: endpoint,
				APIKey:   key,
			}
			if err := cloudauth.Save(cfg); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %s\n", cfg.Endpoint)
			return nil
		},
	}

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Harbor Cloud endpoint URL")
	cmd.Flags().StringVar(&key, "key", "", "API key")

	return cmd
}

// validateAPIKey makes a lightweight authenticated call to verify the API key works.
func validateAPIKey(endpoint, apiKey string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, endpoint+"/api/credentials", nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", endpoint, err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
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
