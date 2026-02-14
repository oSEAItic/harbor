package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/protocol"
	"github.com/spf13/cobra"
)

func newGetCmd(outputFormat string) *cobra.Command {
	var params []string

	cmd := &cobra.Command{
		Use:   "get <connector.resource>",
		Short: "Fetch data from a connector resource",
		Long: `Fetch normalized data from a connector.

Examples:
  harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd
  harbor get stripe.payments --param since=7d`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], ".", 2)
			if len(parts) != 2 {
				return fmt.Errorf("resource must be in format <connector>.<resource>, got %q", args[0])
			}
			connectorName, resource := parts[0], parts[1]

			// Parse --param key=value flags into map
			paramMap := make(map[string]string)
			for _, p := range params {
				kv := strings.SplitN(p, "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid param format %q, expected key=value", p)
				}
				paramMap[kv[0]] = kv[1]
			}

			// Retrieve auth
			token, _ := auth.Retrieve(connectorName) // ignore error — connector may not need auth

			req := protocol.Request{
				Connector: connectorName,
				Resource:  resource,
				Params:    paramMap,
				Auth:      token,
			}

			resp, err := connector.Execute(req)
			if err != nil {
				return fmt.Errorf("executing connector: %w", err)
			}

			// Validate response against protocol
			if err := protocol.ValidateResponse(resp); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: response validation: %v\n", err)
			}

			return printResponse(resp, cmd)
		},
	}

	cmd.Flags().StringArrayVarP(&params, "param", "p", nil, "Connector parameters (key=value)")

	return cmd
}

func newRawCmd() *cobra.Command {
	var params []string

	cmd := &cobra.Command{
		Use:   "raw <connector.resource>",
		Short: "Fetch raw API response from a connector",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], ".", 2)
			if len(parts) != 2 {
				return fmt.Errorf("resource must be in format <connector>.<resource>, got %q", args[0])
			}
			connectorName, resource := parts[0], parts[1]

			paramMap := make(map[string]string)
			for _, p := range params {
				kv := strings.SplitN(p, "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid param format %q, expected key=value", p)
				}
				paramMap[kv[0]] = kv[1]
			}

			token, _ := auth.Retrieve(connectorName)

			req := protocol.Request{
				Connector: connectorName,
				Resource:  resource,
				Params:    paramMap,
				Auth:      token,
				Raw:       true,
			}

			resp, err := connector.Execute(req)
			if err != nil {
				return fmt.Errorf("executing connector: %w", err)
			}

			if resp.Raw != nil {
				out, _ := json.MarshalIndent(resp.Raw, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			} else {
				return printResponse(resp, cmd)
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&params, "param", "p", nil, "Connector parameters (key=value)")

	return cmd
}

func printResponse(resp *protocol.Response, cmd *cobra.Command) error {
	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}
