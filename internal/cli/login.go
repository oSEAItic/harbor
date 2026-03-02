package cli

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/schema"
	"github.com/spf13/cobra"
)

const defaultEndpoint = "https://harbor-cloud.oseaitic.com"

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

			// Auto-sync connector credentials from cloud to Harbor Keychain.
			// Silent on error — credentials can always be synced manually with 'harbor auth sync'.
			if entries, err := cloudauth.ListCloudCredentials(&cfg); err == nil && len(entries) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Syncing %d connector credential(s) to Harbor Keychain...\n", len(entries))
				synced := 0
				for _, e := range entries {
					blob, err := cloudauth.FetchCloudCredentialBlob(e.Connector, &cfg)
					if err != nil {
						continue
					}
					plaintext, err := cloudauth.DecryptCredential(blob, cfg.APIKey)
					if err != nil {
						continue
					}
					if auth.SaveToKeychain(e.Connector, plaintext, cfg.APIKey) == nil {
						_ = auth.Store(e.Connector, plaintext) // best-effort OS keychain
						synced++
					}
				}
				if synced > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %d credential(s) ready for local use\n", synced)
				}
			}

			// Pull cloud memory summaries to local notes cache.
			// Silent on error — memories can be re-pulled on next login.
			if notes, err := cloudauth.PullMemories(&cfg); err == nil && len(notes) > 0 {
				noteStore := memory.NewNoteStore()
				local := make([]memory.Note, 0, len(notes))
				for _, n := range notes {
					local = append(local, memory.Note{Key: n.Key, Content: n.Content, UpdatedAt: n.UpdatedAt})
				}
				if saveErr := noteStore.Save(local); saveErr == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %d memory note(s) synced from cloud\n", len(notes))
				}
			}

			// Pull cloud learned schemas to local schema store.
			// Only writes schemas that don't exist locally — local always wins.
			if cloudSchemas, err := cloudauth.PullSchemas(&cfg); err == nil && len(cloudSchemas) > 0 {
				if schemaStore, err := schema.NewStore(); err == nil {
					synced := 0
					for _, s := range cloudSchemas {
						if schemaStore.Has(s.ToolName) {
							continue // local schema takes precedence
						}
						ls := &schema.LearnedSchema{
							ToolName:        s.ToolName,
							SummaryFields:   s.SummaryFields,
							SummaryTemplate: s.SummaryTemplate,
							LearnedAt:       s.LearnedAt,
							LLMModel:        "cloud-sync",
						}
						if schemaStore.Save(ls) == nil {
							synced++
						}
					}
					if synced > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %d schema(s) synced from cloud\n", synced)
					}
				}
			}

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
