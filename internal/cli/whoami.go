package cli

import (
	"fmt"

	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/spf13/cobra"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current Harbor Cloud login status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudauth.Load()
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in.")
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'harbor login' to connect to Harbor Cloud.")
				return nil
			}

			prefix := cfg.APIKey
			if len(prefix) > 10 {
				prefix = prefix[:10] + "..."
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Endpoint:  %s\n", cfg.Endpoint)
			fmt.Fprintf(cmd.OutOrStdout(), "API Key:   %s\n", prefix)
			fmt.Fprintf(cmd.OutOrStdout(), "Device:    %s\n", cloudauth.DeviceID())
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in: %s\n", cfg.CreatedAt.Format("2006-01-02 15:04"))
			return nil
		},
	}
}
