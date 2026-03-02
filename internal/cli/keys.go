package cli

import (
	"fmt"
	"strings"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/spf13/cobra"
)

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage Harbor Cloud API keys",
		Long: `Create and manage API keys for Harbor Cloud.

Keys control what actions can be performed with them.
Use scoped keys for agents to prevent them from writing credentials:

  harbor keys create --name "my-agent" --read-only-credentials
  # Agent uses this key — can sync credentials but cannot overwrite them

Examples:
  harbor keys list                              # List all keys
  harbor keys create --name "agent-key"         # Full-access key
  harbor keys create --name "agent" --read-only-credentials  # Agent-safe key
  harbor keys revoke <id>                       # Revoke a key`,
	}

	cmd.AddCommand(newKeysListCmd())
	cmd.AddCommand(newKeysCreateCmd())
	cmd.AddCommand(newKeysRevokeCmd())
	return cmd
}

func newKeysListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all API keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudauth.Load()
			if err != nil {
				return fmt.Errorf("not logged in — run 'harbor login' first")
			}

			keys, err := cloudauth.ListAPIKeys(cfg)
			if err != nil {
				return fmt.Errorf("listing keys: %w", err)
			}
			if len(keys) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No API keys found.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-20s  %-12s  %s\n", "ID", "Name", "Prefix", "Scopes")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", strings.Repeat("-", 80))
			for _, k := range keys {
				scopes := "full"
				if k.Policy != nil && k.Policy.AllowedScopes != nil {
					scopes = strings.Join(k.Policy.AllowedScopes, ",")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s  %-20s  %-12s  %s\n",
					k.ID, k.Name, k.Prefix+"...", scopes)
			}
			return nil
		},
	}
}

func newKeysCreateCmd() *cobra.Command {
	var (
		name               string
		readOnlyCredentials bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API key",
		Long: `Create a new Harbor Cloud API key.

Use --read-only-credentials to create a key safe for agents.
Agent keys can sync (read) credentials but cannot store or delete them,
preventing agents from accidentally overwriting your connector credentials.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudauth.Load()
			if err != nil {
				return fmt.Errorf("not logged in — run 'harbor login' first")
			}

			if name == "" {
				return fmt.Errorf("--name is required")
			}

			var policy *cloudauth.KeyPolicy
			if readOnlyCredentials {
				policy = &cloudauth.KeyPolicy{
					AllowedScopes: []string{"credentials:read"},
				}
			}

			rawKey, id, err := cloudauth.CreateScopedAPIKey(name, policy, cfg)
			if err != nil {
				return fmt.Errorf("creating key: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "API key created.\n\n")
			fmt.Fprintf(cmd.OutOrStdout(), "ID:   %s\n", id)
			fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", name)
			if readOnlyCredentials {
				fmt.Fprintf(cmd.OutOrStdout(), "Scope: credentials:read (agent-safe — cannot overwrite credentials)\n")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Scope: full\n")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nKey: %s\n", rawKey)
			fmt.Fprintf(cmd.OutOrStdout(), "\nSave this key — it won't be shown again.\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Key name (required)")
	cmd.Flags().BoolVar(&readOnlyCredentials, "read-only-credentials", false,
		"Restrict to credentials:read — safe for agents (cannot overwrite connector credentials)")
	return cmd
}

func newKeysRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an API key",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a key ID (from 'harbor keys list')")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudauth.Load()
			if err != nil {
				return fmt.Errorf("not logged in — run 'harbor login' first")
			}

			if err := cloudauth.RevokeAPIKey(args[0], cfg); err != nil {
				return fmt.Errorf("revoking key: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Key %s revoked.\n", args[0])
			return nil
		},
	}
}
