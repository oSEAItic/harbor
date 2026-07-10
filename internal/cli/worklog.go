package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/worklog"
	"github.com/spf13/cobra"
)

func newWorklogCmd(outputFormat *string) *cobra.Command {
	cmd := &cobra.Command{Use: "worklog", Short: "Report feature cycle and scope metrics"}
	cmd.AddCommand(newWorklogReportCmd(outputFormat), newWorklogEstimateCmd(outputFormat))
	return cmd
}

func newWorklogReportCmd(outputFormat *string) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use: "report", Short: "Summarize recent feature delivery activity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			duration, err := parseWorklogDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since %q: %w", since, err)
			}
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			now := time.Now().UTC()
			report, err := store.BuildReport(cmd.Context(), now.Add(-duration), now)
			if err != nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Worklog since %s\n", report.Since.Local().Format("2006-01-02 15:04"))
			fmt.Fprintf(cmd.OutOrStdout(), "Active: %d  Blocked: %d  Verified: %d  Shipped: %d\n", report.ActiveCount, report.BlockedCount, report.VerifiedCount, report.ShippedCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Sessions: %d  Scope added: %d\n", report.TotalSessions, report.TotalScopeAdded)
			if len(report.Features) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No feature activity in this period.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nFEATURE             STATUS     CYCLE    BLOCKED  SESS  SCOPE+")
			for _, stats := range report.Features {
				fmt.Fprintf(cmd.OutOrStdout(), "%-19s %-10s %-8s %-8s %-5d %d\n", truncateStr(stats.Feature.Title, 19), stats.Feature.Status, formatDuration(time.Duration(stats.CycleSeconds)*time.Second), formatDuration(time.Duration(stats.BlockedSeconds)*time.Second), stats.SessionCount, stats.ScopeAdded)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Reporting window, such as 24h or 7d")
	return cmd
}

func newWorklogEstimateCmd(outputFormat *string) *cobra.Command {
	var project, kind, size string
	cmd := &cobra.Command{
		Use: "estimate", Short: "Estimate cycle time from shipped features",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			estimate, err := store.Estimate(cmd.Context(), strings.TrimSpace(project), strings.TrimSpace(kind), strings.TrimSpace(size))
			if err != nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, estimate)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Matching shipped features: %d\n", estimate.Samples)
			if estimate.Samples == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No historical estimate available.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "P50 cycle: %s\nP80 cycle: %s\n", formatDuration(time.Duration(estimate.P50CycleSeconds)*time.Second), formatDuration(time.Duration(estimate.P80CycleSeconds)*time.Second))
			if estimate.Samples < 5 {
				fmt.Fprintln(cmd.OutOrStdout(), "Caution: fewer than 5 samples; treat this estimate as directional.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Filter by project")
	cmd.Flags().StringVar(&kind, "type", "", "Filter by feature type")
	cmd.Flags().StringVar(&size, "size", "", "Filter by size class")
	return cmd
}
