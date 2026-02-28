package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oseaitic/harbor/internal/cloudauth"
)

func newRegisterCmd() *cobra.Command {
	var endpoint string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Create a Harbor Cloud account using an invite code",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			fmt.Fprintln(out, "Create your Harbor Cloud account")
			fmt.Fprintln(out, "────────────────────────────────")

			email, err := prompt("Email")
			if err != nil {
				return err
			}
			email = strings.TrimSpace(email)
			if email == "" {
				return fmt.Errorf("email is required")
			}

			password, err := promptSecret("Password")
			if err != nil {
				return err
			}
			if len(password) < 8 {
				return fmt.Errorf("password must be at least 8 characters")
			}

			confirm, err := promptSecret("Confirm password")
			if err != nil {
				return err
			}
			if password != confirm {
				return fmt.Errorf("passwords do not match")
			}

			inviteCode, err := prompt("Invite code")
			if err != nil {
				return err
			}
			inviteCode = strings.TrimSpace(inviteCode)
			if inviteCode == "" {
				return fmt.Errorf("invite code is required")
			}

			fmt.Fprintf(out, "\nRegistering...")
			_, err = cloudauth.Register(endpoint, email, password, inviteCode)
			if err != nil {
				fmt.Fprintln(out)
				return fmt.Errorf("registration failed: %w", err)
			}
			fmt.Fprintln(out, " done")

			// Auto-login after registration
			fmt.Fprintf(out, "Logging in...")
			sessionToken, apiKey, err := cloudauth.LoginWithPassword(endpoint, email, password)
			if err != nil {
				fmt.Fprintln(out)
				return fmt.Errorf("auto-login failed: %w", err)
			}
			fmt.Fprintln(out, " done")

			// Server creates a default API key on first login; create one if it didn't
			if apiKey == "" {
				apiKey, err = cloudauth.CreateAPIKey(endpoint, sessionToken, "default")
				if err != nil {
					return fmt.Errorf("creating API key: %w", err)
				}
			}

			if err := cloudauth.Save(cloudauth.Config{
				Endpoint: endpoint,
				APIKey:   apiKey,
			}); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "Logged in to %s as %s\n", endpoint, email)
			fmt.Fprintln(out, "Your API key has been saved. Run 'harbor get <connector>.<tool>' to start fetching data.")
			return nil
		},
	}

	cmd.Flags().StringVar(&endpoint, "endpoint", "https://harbor-cloud.oseaitic.com", "Harbor Cloud endpoint")
	return cmd
}

// prompt reads a line of plain text from stdin with a label.
func prompt(label string) (string, error) {
	fmt.Printf("%s: ", label)
	var line string
	_, err := fmt.Scanln(&line)
	if err != nil && err.Error() == "unexpected newline" {
		return "", nil
	}
	return line, err
}
