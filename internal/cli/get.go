package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/connector"
	harborctx "github.com/oseaitic/harbor/internal/context"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/pipeline"
	"github.com/oseaitic/harbor/internal/protocol"
	"github.com/spf13/cobra"
)

func newGetCmd(outputFormat string) *cobra.Command {
	var (
		params    []string
		full      bool
		budget    int
		limit     int
		fields    string
		expandIdx int
		noMemory  bool
		refresh   bool
		layer     string
		errors    string
	)

	cmd := &cobra.Command{
		Use:   "get <connector.resource> [value]",
		Short: "Fetch data from a connector resource",
		Long: `Fetch normalized data from a connector.

By default, returns an agent-friendly overview with summary fields and a
natural language summary. Results are saved to memory and served from
memory on subsequent calls within the TTL window.

If a resource has exactly one required parameter, you can pass its value
as a positional argument instead of using -p:

  harbor get kuse-hive.trace a71a549c-...
  harbor get coingecko.prices bitcoin          (shorthand for -p ids=bitcoin)

  --full           Return full uncompiled output (for debugging)
  --budget N       Fit output to ~N tokens
  --limit N        Max items to include
  --fields f1,f2   Only include these fields
  --expand N       Full detail for item at index N
  --no-memory      Skip saving to and reading from memory
  --refresh        Force fresh fetch (bypass memory)
  --layer          Layer to serve from memory (raw|normalized|compact|summary)
  --errors         Error mode: soft (degrade) or hard (fail fast)

Examples:
  harbor get coingecko.prices bitcoin
  harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd
  harbor get coingecko.trending
  harbor get coingecko.trending --budget 500
  harbor get coingecko.trending --expand 0
  harbor get coingecko.trending --limit 5 --fields id,name,score
  harbor get coingecko.trending --full
  harbor get coingecko.trending --refresh
  harbor get coingecko.trending --no-memory`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || len(args) > 2 {
				return fmt.Errorf("requires <connector.resource> [value], e.g.: harbor get coingecko.prices")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			connectorName, resource, err := pipeline.ParseConnectorResource(args[0])
			if err != nil {
				return err
			}

			// Parse --param key=value flags into map
			paramMap := make(map[string]string)
			for _, p := range params {
				kv := strings.SplitN(p, "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid param format %q, expected key=value", p)
				}
				paramMap[kv[0]] = kv[1]
			}

			// B1: Positional shorthand — resolve positional value to required param
			if len(args) == 2 && len(paramMap) == 0 {
				if err := resolvePositionalParam(connectorName, resource, args[1], paramMap); err != nil {
					return err
				}
			}

			// B3: Missing required params — show schema hint
			if len(paramMap) == 0 {
				if err := checkRequiredParams(connectorName, resource, "get"); err != nil {
					return err
				}
			}

			// CLI-specific: handle --layer flag for memory output
			if !noMemory && !refresh && layer != "" {
				store, err := memory.NewStore()
				if err == nil {
					entry := store.Latest(connectorName, resource, paramMap)
					if store.IsFresh(entry) {
						obj, err := store.Get(entry.ID)
						if err == nil {
							return outputFromMemory(cmd, obj, layer)
						}
					}
				}
			}

			// Build compile options from flags
			opts := harborctx.DefaultOptions()
			if full {
				opts.Mode = "full"
			} else if expandIdx >= 0 {
				opts.Mode = "expand"
				opts.ExpandIdx = expandIdx
			}
			if budget > 0 {
				opts.Budget = budget
			}
			if limit > 0 {
				opts.Limit = limit
			}
			if fields != "" {
				opts.Fields = strings.Split(fields, ",")
			}
			if errors != string(pipeline.ErrorModeSoft) && errors != string(pipeline.ErrorModeHard) {
				return fmt.Errorf("invalid --errors value %q, expected soft or hard", errors)
			}

			result, err := pipeline.Execute(connectorName, resource, paramMap, nil, pipeline.Options{
				NoMemory: noMemory,
				Refresh:  refresh,
				Compile:  opts,
				Errors:   pipeline.ErrorMode(errors),
			})
			if err != nil {
				return err
			}

			// CLI-specific: handle --layer flag on pipeline memory hit
			if result.FromMem && layer != "" {
				store, _ := memory.NewStore()
				if obj, err := store.Get(result.MemoryID); err == nil {
					return outputFromMemory(cmd, obj, layer)
				}
			}

			return printResponse(result.Response, cmd)
		},
	}

	cmd.Flags().StringArrayVarP(&params, "param", "p", nil, "Connector parameters (key=value)")
	cmd.Flags().BoolVar(&full, "full", false, "Return full uncompiled output")
	cmd.Flags().IntVar(&budget, "budget", 0, "Target token budget for output")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of items to include")
	cmd.Flags().StringVar(&fields, "fields", "", "Comma-separated list of fields to include")
	cmd.Flags().IntVar(&expandIdx, "expand", -1, "Expand full detail for item at index")
	cmd.Flags().BoolVar(&noMemory, "no-memory", false, "Skip saving to and reading from memory")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force fresh fetch (bypass memory)")
	cmd.Flags().StringVar(&layer, "layer", "", "Layer to serve from memory (raw|normalized|compact|summary)")
	cmd.Flags().StringVar(&errors, "errors", "soft", "Error mode: soft | hard")

	// B2: Hint about -p when user passes unknown flags like --execution_id
	cmd.SetFlagErrorFunc(flagErrorWithParamHint)

	return cmd
}

// outputFromMemory serves a response from a stored memory object.
func outputFromMemory(cmd *cobra.Command, obj *memory.Object, layerFlag string) error {
	layerName := memory.LayerCompact
	if layerFlag != "" {
		layerName = memory.LayerName(layerFlag)
	}

	resp := &protocol.Response{
		Meta: protocol.Meta{
			Source:           obj.Meta.Source,
			ConnectorVersion: obj.Meta.ConnectorVersion,
			Schema:           obj.Schema,
			FetchedAt:        obj.CreatedAt,
			RequestID:        obj.Meta.RequestID,
			MemoryID:         obj.ID,
			FromMemory:       true,
		},
		Errors: []protocol.ErrorDetail{},
	}

	switch layerName {
	case memory.LayerRaw:
		resp.Data = obj.Layers.Raw
	case memory.LayerNormalized:
		resp.Data = obj.Layers.Normalized
	case memory.LayerCompact:
		resp.Data = obj.Layers.Compact
		resp.Summary = obj.Layers.Summary
	case memory.LayerSummary:
		resp.Data = []byte("[]")
		resp.Summary = obj.Layers.Summary
	default:
		return fmt.Errorf("unknown layer %q, expected: raw, normalized, compact, summary", layerFlag)
	}

	return printResponse(resp, cmd)
}

func newRawCmd() *cobra.Command {
	var params []string

	cmd := &cobra.Command{
		Use:   "raw <connector.resource> [value]",
		Short: "Fetch raw API response from a connector",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || len(args) > 2 {
				return fmt.Errorf("requires <connector.resource> [value], e.g.: harbor raw coingecko.prices")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			connectorName, resource, err := pipeline.ParseConnectorResource(args[0])
			if err != nil {
				return err
			}

			paramMap := make(map[string]string)
			for _, p := range params {
				kv := strings.SplitN(p, "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid param format %q, expected key=value", p)
				}
				paramMap[kv[0]] = kv[1]
			}

			// B1: Positional shorthand
			if len(args) == 2 && len(paramMap) == 0 {
				if err := resolvePositionalParam(connectorName, resource, args[1], paramMap); err != nil {
					return err
				}
			}

			// B3: Missing required params hint
			if len(paramMap) == 0 {
				if err := checkRequiredParams(connectorName, resource, "raw"); err != nil {
					return err
				}
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

	// B2: Hint about -p when user passes unknown flags like --execution_id
	cmd.SetFlagErrorFunc(flagErrorWithParamHint)

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

// defaultTTL returns the TTL for a connector+resource combination.
func defaultTTL(connector, resource string) time.Duration {
	return memory.DefaultTTL(connector, resource)
}

// flagErrorWithParamHint wraps Cobra flag errors to hint about -p for unknown flags.
func flagErrorWithParamHint(_ *cobra.Command, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "unknown flag") {
		return fmt.Errorf("%s\nHint: use -p key=value to pass connector parameters", msg)
	}
	return err
}

// resolvePositionalParam maps a positional value arg to the single required parameter
// of a connector resource. If the schema is unavailable or has != 1 required param,
// returns an error.
func resolvePositionalParam(connectorName, resource, value string, paramMap map[string]string) error {
	schema, err := connector.GetResourceSchema(connectorName, resource)
	if err != nil {
		// Schema unavailable — can't resolve positional arg
		return fmt.Errorf("unexpected positional argument %q (could not resolve: %v)\nHint: use -p key=value to pass connector parameters", value, err)
	}
	required := parseRequiredParams(schema.Parameters)
	if len(required) == 1 {
		paramMap[required[0]] = value
		return nil
	}
	if len(required) == 0 {
		return fmt.Errorf("unexpected positional argument %q (%s.%s has no required parameters)\nHint: use -p key=value to pass connector parameters", value, connectorName, resource)
	}
	return fmt.Errorf("unexpected positional argument %q (%s.%s has %d required parameters: %s)\nHint: use -p key=value to pass connector parameters", value, connectorName, resource, len(required), strings.Join(required, ", "))
}

// checkRequiredParams returns a descriptive error when a resource has required
// parameters but none were provided. Returns nil if schema is unavailable or
// the resource has no required params (lets the connector handle it).
func checkRequiredParams(connectorName, resource, mode string) error {
	schema, err := connector.GetResourceSchema(connectorName, resource)
	if err != nil {
		return nil // schema unavailable — let connector handle validation
	}
	required := parseRequiredParams(schema.Parameters)
	if len(required) == 0 {
		return nil
	}
	fullName := connectorName + "." + resource
	var usage strings.Builder
	fmt.Fprintf(&usage, "%s requires parameter: %s\n\nUsage:\n", fullName, strings.Join(required, ", "))
	if len(required) == 1 {
		fmt.Fprintf(&usage, "  harbor %s %s <%s>\n", mode, fullName, required[0])
	}
	for _, key := range required {
		fmt.Fprintf(&usage, "  harbor %s %s -p %s=<value>\n", mode, fullName, key)
	}
	return fmt.Errorf("%s", usage.String())
}
