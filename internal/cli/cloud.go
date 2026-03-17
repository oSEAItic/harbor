package cli

import (
	"fmt"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/spf13/cobra"
)

func newCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Manage Harbor Cloud connection",
		Long: `Manage cross-device memory sync via Harbor Cloud.

  harbor cloud status   Show connection status
  harbor cloud enable   Auto-provision a free account (50 memories)
  harbor cloud disable  Opt out of cloud sync (local-only mode)`,
	}

	cmd.AddCommand(newCloudStatusCmd())
	cmd.AddCommand(newCloudEnableCmd())
	cmd.AddCommand(newCloudDisableCmd())
	return cmd
}

func newCloudStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cloud connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cloudauth.IsOptedOut() {
				fmt.Fprintln(cmd.OutOrStdout(), "Cloud sync: disabled (opted out)")
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'harbor cloud enable' to opt back in.")
				return nil
			}
			cfg, err := cloudauth.Load()
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Cloud sync: not configured")
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'harbor cloud enable' for free auto-provisioned account,")
				fmt.Fprintln(cmd.OutOrStdout(), "or 'harbor login' to connect an existing account.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cloud sync: connected\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Endpoint:   %s\n", cfg.Endpoint)
			fmt.Fprintf(cmd.OutOrStdout(), "API key:    %s...\n", cfg.APIKey[:10])
			return nil
		},
	}
}

func newCloudEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Auto-provision a free cloud account (50 memories)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Remove opt-out marker if it exists.
			_ = cloudauth.ClearOptOut()

			// Check if already configured.
			if cloudauth.IsLoggedIn() {
				fmt.Fprintln(cmd.OutOrStdout(), "Cloud sync already configured. Use 'harbor cloud status' to check.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Provisioning free Harbor Cloud account...")
			apiKey, err := cloudauth.AutoProvision()
			if err != nil {
				return fmt.Errorf("auto-provision failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cloud sync enabled.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "API key: %s...\n", apiKey[:10])
			fmt.Fprintf(cmd.OutOrStdout(), "Free tier: 50 memories. Register at harbor-cloud.oseaitic.com for unlimited.\n")
			return nil
		},
	}
}

func newCloudDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Opt out of cloud sync (local-only mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cloudauth.OptOut(); err != nil {
				return fmt.Errorf("failed to opt out: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cloud sync disabled. Memories will only be stored locally.")
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'harbor cloud enable' to opt back in anytime.")
			return nil
		},
	}
}
