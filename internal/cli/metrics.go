package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/metrics"
	"github.com/oseaitic/harbor/internal/proxy"
	"github.com/spf13/cobra"
)

func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Metrics utilities",
	}

	cmd.AddCommand(newMetricsReportCmd())
	return cmd
}

func newMetricsReportCmd() *cobra.Command {
	var (
		since  string
		tool   string
		path   string
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show proxy compression and memory metrics",
		Long: `Summarize Harbor proxy metrics from proxy_compression.jsonl.

Examples:
  harbor metrics report
  harbor metrics report --since 24h
  harbor metrics report --tool list_files
  harbor metrics report --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(path) == "" {
				p, err := proxy.DefaultMetricsPath()
				if err != nil {
					return err
				}
				path = p
			}

			var sinceDuration time.Duration
			if strings.TrimSpace(since) != "" {
				d, err := parseDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since %q: %w", since, err)
				}
				sinceDuration = d
			}

			events, err := metrics.Load(path)
			if err != nil {
				return err
			}
			report := metrics.BuildSummary(path, events, metrics.Query{
				Since: sinceDuration,
				Tool:  strings.TrimSpace(tool),
			})

			if asJSON {
				out, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling report: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			printMetricsReport(cmd, report)
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Only include metrics newer than duration")
	cmd.Flags().StringVar(&tool, "tool", "", "Filter by tool name")
	cmd.Flags().StringVar(&path, "path", "", "Override metrics JSONL path")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print JSON report")
	return cmd
}

func printMetricsReport(cmd *cobra.Command, report metrics.Summary) {
	fmt.Fprintf(cmd.OutOrStdout(), "Metrics report: %s\n", report.Path)
	if report.Since > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Since: %s\n", report.Since)
	}
	if report.TotalCalls > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Window: %s -> %s\n",
			report.WindowStart.Format(time.RFC3339),
			report.WindowEnd.Format(time.RFC3339),
		)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Total calls: %d\n", report.TotalCalls)
	fmt.Fprintf(cmd.OutOrStdout(), "Memory hit rate: %.1f%%\n", report.MemoryHitRate*100)
	fmt.Fprintf(cmd.OutOrStdout(), "Schema applied rate: %.1f%%\n", report.SchemaAppliedRate*100)
	fmt.Fprintf(cmd.OutOrStdout(), "Drift rate: %.1f%%\n", report.DriftRate*100)
	fmt.Fprintf(cmd.OutOrStdout(), "Avg compression ratio: %.3f\n", report.AvgCompressionRatio)
	fmt.Fprintf(cmd.OutOrStdout(), "Approx tokens saved: %d\n", report.ApproxTokensSavedTotal)

	if len(report.Tools) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNo matching tool metrics.")
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nTop tools:")
	fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-7s %-7s %-8s %-8s %-9s %s\n",
		"Tool", "Calls", "Hit%", "Schema%", "Drift%", "AvgRatio", "TokSaved")
	fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-7s %-7s %-8s %-8s %-9s %s\n",
		"------------------------", "-------", "-------", "--------", "--------", "---------", "--------")

	for i, t := range report.Tools {
		if i >= 10 {
			break
		}
		hitRate := 0.0
		schemaRate := 0.0
		driftRate := 0.0
		if t.Calls > 0 {
			hitRate = float64(t.MemoryHits) / float64(t.Calls) * 100
			schemaRate = float64(t.SchemaAppliedCalls) / float64(t.Calls) * 100
			driftRate = float64(t.DriftDetectedCalls) / float64(t.Calls) * 100
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-7d %-6.1f%% %-7.1f%% %-7.1f%% %-9.3f %d\n",
			t.ToolName, t.Calls, hitRate, schemaRate, driftRate, t.AvgCompressionRatio, t.ApproxTokensSavedTotal)
	}
}
