package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/oseaitic/harbor/internal/cloudauth"
	harborctx "github.com/oseaitic/harbor/internal/context"
	"github.com/oseaitic/harbor/internal/httpfetch"
	"github.com/oseaitic/harbor/internal/pipeline"
	"github.com/spf13/cobra"
)

func newFetchCmd() *cobra.Command {
	var (
		method     string
		authName   string
		authHeader string
		body       string
		headers    []string
		noMemory   bool
		refresh    bool
	)

	cmd := &cobra.Command{
		Use:   "fetch <url>",
		Short: "Fetch data from any API through Harbor's auth proxy",
		Long: `Make an authenticated HTTP request through Harbor.

Harbor injects credentials from its secure keychain — you never see raw API
keys. Responses are cached in memory and benefit from schema learning, just
like connector-based calls.

The credential name (--auth) must match a key stored via 'harbor auth'.
The auth header format (--auth-header) controls how the credential is injected:

  "Authorization: Bearer"   → Authorization: Bearer <secret>  (default)
  "x-cg-pro-api-key"        → x-cg-pro-api-key: <secret>
  "X-API-Key"                → X-API-Key: <secret>

Examples:
  harbor fetch https://api.coingecko.com/v3/search/trending \
    --auth coingecko --auth-header "x-cg-pro-api-key"

  harbor fetch https://api.github.com/repos/oseaitic/harbor/issues \
    --auth github-pat

  harbor fetch https://api.openai.com/v1/models --auth openai

  harbor fetch https://httpbin.org/post --method POST \
    --body '{"hello":"world"}' --auth myservice`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]

			// Parse --header key:value flags
			headerMap := make(map[string]string)
			for _, h := range headers {
				k, v, ok := parseHeader(h)
				if !ok {
					return fmt.Errorf("invalid header format %q, expected 'Name: Value'", h)
				}
				headerMap[k] = v
			}

			// Execute HTTP fetch with auth injection
			result, err := httpfetch.Fetch(context.Background(), httpfetch.Options{
				URL:        url,
				Method:     method,
				Headers:    headerMap,
				Body:       body,
				AuthName:   authName,
				AuthHeader: authHeader,
			})

			// If credential is missing, offer browser-based setup.
			var missingCred *httpfetch.MissingCredentialError
			if errors.As(err, &missingCred) {
				setupURL := credentialSetupURL(missingCred.Name)
				if setupURL != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] Credential %q not found. Opening setup page...\n", missingCred.Name)
					openBrowser(setupURL)
					fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] After saving your key, re-run this command.\n")
					fmt.Fprintf(cmd.ErrOrStderr(), "[harbor] Or run: harbor auth %s\n", missingCred.Name)
					return nil
				}
				// No cloud account — fall back to CLI hint.
				return fmt.Errorf("credential %q not found. Run: harbor auth %s", missingCred.Name, missingCred.Name)
			}
			if err != nil {
				return err
			}

			// Run through pipeline for memory/schema/recall
			if !noMemory {
				exec := &httpfetch.PassthroughExecutor{Response: result.Response}
				pipeResult, err := pipeline.Execute(exec, result.Connector, result.Resource, nil, nil, pipeline.Options{
					Compile: harborctx.DefaultOptions(),
					Refresh: refresh,
				})
				if err == nil {
					result.Response = pipeResult.Response
				}
			}

			// Output JSON
			out, err := json.MarshalIndent(result.Response, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVarP(&method, "method", "X", "GET", "HTTP method (GET, POST, PUT, DELETE, PATCH)")
	cmd.Flags().StringVar(&authName, "auth", "", "Credential name from Harbor keychain (required)")
	cmd.Flags().StringVar(&authHeader, "auth-header", "Authorization: Bearer", "How to inject the credential into the request")
	cmd.Flags().StringVarP(&body, "body", "d", "", "Request body (for POST/PUT/PATCH)")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Additional headers (repeatable, format: 'Name: Value')")
	cmd.Flags().BoolVar(&noMemory, "no-memory", false, "Skip memory cache")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force fresh fetch (bypass memory)")

	cmd.MarkFlagRequired("auth")

	return cmd
}

// credentialSetupURL generates a Harbor Cloud setup URL for a missing credential.
// The URL includes the full API key as auth — the setup page uses it to store
// the credential via PUT /api/credentials/:name.
// Returns "" if the user has no cloud account and auto-provision fails.
func credentialSetupURL(credName string) string {
	cfg, err := cloudauth.Load()
	if err != nil {
		// Try auto-provision.
		if _, provErr := cloudauth.AutoProvision(); provErr != nil {
			return ""
		}
		cfg, err = cloudauth.Load()
		if err != nil {
			return ""
		}
	}
	return cfg.Endpoint + "/setup/" + credName
}

// openBrowser opens a URL in the user's default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("open", url)
	}
	_ = cmd.Start()
}

// parseHeader splits "Name: Value" into key and value.
func parseHeader(s string) (string, string, bool) {
	for i, c := range s {
		if c == ':' {
			key := s[:i]
			val := s[i+1:]
			if len(val) > 0 && val[0] == ' ' {
				val = val[1:]
			}
			if key == "" {
				return "", "", false
			}
			return key, val, true
		}
	}
	return "", "", false
}
