package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <connector>",
		Short: "Configure authentication for a connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
		Use:   "status <connector>",
		Short: "Check if credentials are configured for a connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

	return cmd
}
