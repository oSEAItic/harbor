package cli

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth [connector]",
		Short: "Manage connector credentials",
		Long: `Manage authentication credentials for connectors.

When logged in to Harbor Cloud, credentials are stored encrypted in Harbor Cloud
(client-side AES-256-GCM, key derived from your API key — the server never sees
plaintext). They are also written to the local Harbor Keychain
(~/.harbor/keychain.json, AES-256-GCM encrypted with your API key).

On a new machine or sandbox: run 'harbor login' to automatically sync all
credentials from Harbor Cloud to the local Harbor Keychain. No password prompts.

Resolution order for 'harbor get --local':
  1. OS keychain (macOS / Linux)
  2. Harbor Keychain (~/.harbor/keychain.json) — works in any environment

Examples:
  harbor auth kuse-hive           # Store credential (cloud + Harbor Keychain if logged in)
  harbor auth kuse-hive --local   # Force store in OS keychain only
  harbor auth sync                # Sync all cloud credentials to Harbor Keychain
  harbor auth sync github-mcp     # Sync one connector
  harbor auth status              # List all stored credentials
  harbor auth status github-mcp   # Check if credential exists
  harbor auth delete github-mcp   # Remove credential (cloud + local)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return authList(cmd)
			}
			local, _ := cmd.Flags().GetBool("local")
			return authStore(cmd, args[0], local)
		},
	}

	cmd.Flags().Bool("local", false, "Force store in local OS keychain instead of cloud")

	cmd.AddCommand(newAuthSyncCmd())
	cmd.AddCommand(newAuthStatusCmd())
	cmd.AddCommand(newAuthDeleteCmd())

	return cmd
}

// authStore stores a connector credential.
// If logged in and not --local: encrypts client-side and stores in cloud.
// Otherwise: stores in local OS keychain.
func authStore(cmd *cobra.Command, connector string, forceLocal bool) error {
	cfg, loginErr := cloudauth.Load()
	useCloud := loginErr == nil && !forceLocal

	target := "OS keychain"
	if useCloud {
		target = fmt.Sprintf("Harbor Cloud (%s)", cfg.Endpoint)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Storing credential for: %s\n", connector)
	fmt.Fprintf(cmd.OutOrStdout(), "Target: %s\n", target)
	if useCloud {
		fmt.Fprintln(cmd.OutOrStdout(), "         Use --local to store in OS keychain instead.")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	credential, err := promptSecret("Connector API key")
	if err != nil {
		return fmt.Errorf("reading credential: %w", err)
	}
	if credential == "" {
		return fmt.Errorf("credential is required")
	}

	if useCloud {
		// Encrypt client-side with the API key — no separate password needed.
		// The server stores only ciphertext and cannot decrypt it.
		fmt.Fprintf(cmd.OutOrStdout(), "\nEncrypting and uploading...\n")
		if err := cloudauth.StoreCloudCredential(connector, credential, cfg.APIKey, cfg); err != nil {
			return fmt.Errorf("storing cloud credential: %w", err)
		}
		// Also write to Harbor Keychain so this machine can use it immediately.
		if err := auth.SaveToKeychain(connector, credential, cfg.APIKey); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not write to Harbor Keychain: %v\n", err)
		}
		// Best-effort: also update OS keychain to keep it in sync.
		// Without this, a stale OS keychain entry would shadow the new credential
		// (OS keychain takes precedence over Harbor Keychain in Retrieve()).
		_ = auth.Store(connector, credential)
		fmt.Fprintf(cmd.OutOrStdout(), "Credential stored in Harbor Cloud and Harbor Keychain.\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Available on any machine after: harbor login\n")
		return nil
	}

	if err := auth.Store(connector, credential); err != nil {
		return fmt.Errorf("storing credential: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Credential stored in OS keychain for %s.\n", connector)
	return nil
}

// newAuthSyncCmd returns the 'harbor auth sync [connector]' subcommand.
func newAuthSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [connector]",
		Short: "Sync cloud credentials to Harbor Keychain (and OS keychain if available)",
		Long: `Download encrypted credentials from Harbor Cloud, decrypt them client-side
using your Harbor API key, and write to the local Harbor Keychain
(~/.harbor/keychain.json).

No password prompt — decryption uses the API key stored by 'harbor login'.
If the OS keychain is available, credentials are also written there.

Examples:
  harbor auth sync                # Sync all cloud credentials
  harbor auth sync github-mcp     # Sync one connector`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudauth.Load()
			if err != nil {
				return fmt.Errorf("not logged in — run 'harbor login' first")
			}

			if len(args) == 1 {
				return syncOne(cmd, cfg, args[0])
			}

			entries, err := cloudauth.ListCloudCredentials(cfg)
			if err != nil {
				return fmt.Errorf("listing cloud credentials: %w", err)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cloud credentials found.")
				return nil
			}

			synced, failed := 0, 0
			for _, e := range entries {
				if err := syncOne(cmd, cfg, e.Connector); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s: %v\n", e.Connector, err)
					failed++
				} else {
					synced++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nSynced %d credential(s)", synced)
			if failed > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), ", %d failed", failed)
			}
			fmt.Fprintln(cmd.OutOrStdout(), ".")
			return nil
		},
	}
}

// syncOne downloads one connector credential from cloud, decrypts with API key,
// and writes to Harbor Keychain + OS keychain (best-effort).
func syncOne(cmd *cobra.Command, cfg *cloudauth.Config, connector string) error {
	blob, err := cloudauth.FetchCloudCredentialBlob(connector, cfg)
	if err != nil {
		return err
	}
	plaintext, err := cloudauth.DecryptCredential(blob, cfg.APIKey)
	if err != nil {
		return err
	}
	if err := auth.SaveToKeychain(connector, plaintext, cfg.APIKey); err != nil {
		return fmt.Errorf("writing Harbor Keychain: %w", err)
	}
	// Best-effort: also write to OS keychain (may fail in sandboxes — that's OK).
	_ = auth.Store(connector, plaintext)
	fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s → Harbor Keychain\n", connector)
	return nil
}

// newAuthStatusCmd returns the 'harbor auth status [connector]' subcommand.
func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status [connector]",
		Short:   "Check credential status (local keychain + cloud)",
		Example: "  harbor auth status\n  harbor auth status github-mcp",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return authList(cmd)
			}
			name := args[0]

			_, localErr := auth.Retrieve(name)
			localOK := localErr == nil

			cloudOK := false
			if cfg, err := cloudauth.Load(); err == nil {
				if entries, err := cloudauth.ListCloudCredentials(cfg); err == nil {
					for _, e := range entries {
						if e.Connector == name {
							cloudOK = true
							break
						}
					}
				}
			}

			switch {
			case localOK && cloudOK:
				fmt.Fprintf(cmd.OutOrStdout(), "%s: configured (local keychain + cloud)\n", name)
			case localOK:
				fmt.Fprintf(cmd.OutOrStdout(), "%s: configured (local keychain only)\n", name)
			case cloudOK:
				fmt.Fprintf(cmd.OutOrStdout(), "%s: configured (cloud only — run 'harbor auth sync %s' to use locally)\n", name, name)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "%s: not configured\n", name)
			}
			return nil
		},
	}
}

// newAuthDeleteCmd returns the 'harbor auth delete <connector>' subcommand.
func newAuthDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <connector>",
		Short: "Remove stored credentials (local keychain + cloud)",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires a connector name, e.g.: harbor auth delete github-mcp")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			removed := false

			if err := auth.Delete(name); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed from local keychain: %s\n", name)
				removed = true
			}

			if cfg, err := cloudauth.Load(); err == nil {
				if err := cloudauth.DeleteCloudCredential(name, cfg); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Removed from Harbor Cloud: %s\n", name)
					removed = true
				}
			}

			if !removed {
				fmt.Fprintf(cmd.OutOrStdout(), "No credential found for %s.\n", name)
			}
			return nil
		},
	}
}

func authList(cmd *cobra.Command) error {
	local := auth.List()

	var cloudEntries []cloudauth.CredentialEntry
	if cfg, err := cloudauth.Load(); err == nil {
		cloudEntries, _ = cloudauth.ListCloudCredentials(cfg)
	}

	if len(local) == 0 && len(cloudEntries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No credentials stored.")
		fmt.Fprintln(cmd.OutOrStdout(), "\nStore one with: harbor auth <connector>")
		return nil
	}

	if len(local) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Local keychain (%d):\n", len(local))
		for _, e := range local {
			age := formatDuration(time.Since(e.CreatedAt))
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  (stored %s ago)\n", e.Name, age)
		}
	}

	if len(cloudEntries) > 0 {
		if len(local) > 0 {
			fmt.Fprintln(cmd.OutOrStdout())
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Harbor Cloud (%d):\n", len(cloudEntries))
		for _, e := range cloudEntries {
			age := formatDuration(time.Since(e.UpdatedAt))
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  (stored %s ago)\n", e.Connector, age)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\nRun 'harbor auth sync' to copy cloud credentials to local keychain.")
	}

	return nil
}

// promptSecret reads a secret from the terminal without echo.
// Falls back to line-buffered stdin if not running in a terminal.
func promptSecret(prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)
	if term.IsTerminal(int(syscall.Stdin)) {
		secret, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(secret)), nil
	}
	// Non-terminal fallback (piped input / tests)
	var line string
	_, err := fmt.Scanln(&line)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
