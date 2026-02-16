package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/memory"
	"github.com/spf13/cobra"
)

func newRecallCmd() *cobra.Command {
	var (
		params []string
		layer  string
		since  string
		list   bool
		search string
	)

	cmd := &cobra.Command{
		Use:   "recall [connector.resource]",
		Short: "Recall data from memory",
		Long: `Recall previously fetched data from Harbor's memory layer.

By default, returns the compact layer (summary fields + natural language summary).

  --layer raw|normalized|compact|summary   Pick a specific layer
  --since 1h                               Only show memories newer than duration
  --list                                   List recent memories
  --search "query"                         Search memories by keyword
  -p key=value                             Filter by params

Examples:
  harbor recall coingecko.trending
  harbor recall coingecko.trending --layer summary
  harbor recall yahoo.quote -p symbols=AAPL --layer raw
  harbor recall --list
  harbor recall --list --since 1h`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := memory.NewStore()
			if err != nil {
				return fmt.Errorf("opening memory store: %w", err)
			}

			// Parse --since duration
			var sinceDuration time.Duration
			if since != "" {
				d, err := parseDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since %q: %w", since, err)
				}
				sinceDuration = d
			}

			// --list mode: show table of recent memories
			if list {
				return recallList(cmd, store, sinceDuration)
			}

			// --search mode: keyword search across memories
			if search != "" {
				return recallSearch(cmd, store, search, sinceDuration)
			}

			// Require connector.resource for non-list modes
			if len(args) == 0 {
				return fmt.Errorf("specify <connector.resource> or use --list")
			}

			parts := strings.SplitN(args[0], ".", 2)
			if len(parts) != 2 {
				return fmt.Errorf("resource must be in format <connector>.<resource>, got %q", args[0])
			}
			connectorName, resource := parts[0], parts[1]

			// Parse params
			paramMap := make(map[string]string)
			for _, p := range params {
				kv := strings.SplitN(p, "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid param format %q, expected key=value", p)
				}
				paramMap[kv[0]] = kv[1]
			}

			// Find latest matching memory
			entry := store.Latest(connectorName, resource, paramMap)
			if entry == nil {
				return fmt.Errorf("no memory found for %s.%s", connectorName, resource)
			}

			// Check --since filter
			if sinceDuration > 0 && time.Since(entry.CreatedAt) > sinceDuration {
				return fmt.Errorf("no memory for %s.%s within %s", connectorName, resource, since)
			}

			obj, err := store.Get(entry.ID)
			if err != nil {
				return fmt.Errorf("loading memory object: %w", err)
			}

			return outputFromMemory(cmd, obj, layer)
		},
	}

	cmd.Flags().StringArrayVarP(&params, "param", "p", nil, "Filter by connector parameters (key=value)")
	cmd.Flags().StringVar(&layer, "layer", "", "Layer to recall (raw|normalized|compact|summary)")
	cmd.Flags().StringVar(&since, "since", "", "Only show memories newer than duration (e.g. 1h, 30m)")
	cmd.Flags().BoolVar(&list, "list", false, "List recent memories")
	cmd.Flags().StringVar(&search, "search", "", "Search memories by keyword")

	return cmd
}

// recallList prints a table of recent memory entries.
func recallList(cmd *cobra.Command, store *memory.Store, since time.Duration) error {
	entries := store.Query(memory.QueryOptions{
		Since: since,
		Limit: 50,
	})

	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No memories found.")
		return nil
	}

	// Header
	fmt.Fprintf(cmd.OutOrStdout(), "%-14s  %-25s  %-8s  %-6s  %s\n",
		"ID", "Resource", "Age", "Fresh?", "Schema")
	fmt.Fprintf(cmd.OutOrStdout(), "%-14s  %-25s  %-8s  %-6s  %s\n",
		"--------------", "-------------------------", "--------", "------", "-------------------")

	for _, e := range entries {
		age := time.Since(e.CreatedAt).Truncate(time.Second)
		fresh := store.IsFresh(&e)

		freshStr := "no"
		if fresh {
			freshStr = "yes"
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-14s  %-25s  %-8s  %-6s  %s\n",
			e.ID,
			e.Connector+"."+e.Resource,
			formatDuration(age),
			freshStr,
			e.Schema,
		)
	}

	return nil
}

// recallSearch performs keyword search across memories and prints results.
func recallSearch(cmd *cobra.Command, store *memory.Store, query string, since time.Duration) error {
	results := store.Search(query, memory.QueryOptions{
		Since: since,
		Limit: 30,
	})

	if len(results) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No memories found matching %q.\n", query)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%d memories matching %q:\n\n", len(results), query)

	// Header
	fmt.Fprintf(cmd.OutOrStdout(), "%-14s  %-25s  %-8s  %-6s  %s\n",
		"ID", "Resource", "Age", "Fresh?", "Summary")
	fmt.Fprintf(cmd.OutOrStdout(), "%-14s  %-25s  %-8s  %-6s  %s\n",
		"--------------", "-------------------------", "--------", "------", "-------------------")

	for _, r := range results {
		freshStr := "no"
		if r.Fresh {
			freshStr = "yes"
		}

		summary := r.Summary
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-14s  %-25s  %-8s  %-6s  %s\n",
			r.ID,
			r.Connector+"."+r.Resource,
			formatDuration(r.Age),
			freshStr,
			summary,
		)
	}

	return nil
}

// parseDuration parses a human-friendly duration string (supports h, m, s suffixes).
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// formatDuration returns a human-friendly short duration string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
