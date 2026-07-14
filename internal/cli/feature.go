package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/worklog"
	"github.com/spf13/cobra"
)

func newFeatureCmd(outputFormat *string) *cobra.Command {
	cmd := &cobra.Command{Use: "feature", Short: "Track feature delivery cycles locally"}
	cmd.AddCommand(
		newFeatureStartCmd(outputFormat),
		newFeatureListCmd(outputFormat),
		newFeatureShowCmd(outputFormat),
		newFeatureBindCmd(),
		newFeatureEventCmd("checkpoint", worklog.EventCheckpoint, "Record a working checkpoint"),
		newFeatureEventCmd("block", worklog.EventBlocked, "Mark a feature as blocked"),
		newFeatureEventCmd("resume", worklog.EventResumed, "Resume a blocked feature"),
		newFeatureEventCmd("verify", worklog.EventVerified, "Mark acceptance criteria as verified"),
		newFeatureEventCmd("ship", worklog.EventShipped, "Mark a verified feature as shipped"),
		newFeatureEventCmd("reopen", worklog.EventReopened, "Reopen a verified or shipped feature"),
		newFeatureScopeCmd(),
	)
	return cmd
}

func newFeatureStartCmd(outputFormat *string) *cobra.Command {
	var project, kind, size, budget string
	cmd := &cobra.Command{
		Use:   "start <title>",
		Short: "Start tracking a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(project) == "" {
				return fmt.Errorf("--project is required")
			}
			var budgetDuration time.Duration
			var err error
			if strings.TrimSpace(budget) != "" {
				budgetDuration, err = parseWorklogDuration(budget)
				if err != nil {
					return fmt.Errorf("invalid --budget %q: %w", budget, err)
				}
			}
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			feature, err := store.CreateFeature(cmd.Context(), project, args[0], kind, size, budgetDuration)
			if err != nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, feature)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Started %s [%s] %s\n", feature.ID, feature.Project, feature.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project or repository name (required)")
	cmd.Flags().StringVar(&kind, "type", "", "Feature type, such as bug or integration")
	cmd.Flags().StringVar(&size, "size", "", "Initial size class, such as S, M, or L")
	cmd.Flags().StringVar(&budget, "budget", "", "Delivery budget, such as 8h or 2d")
	return cmd
}

func newFeatureListCmd(outputFormat *string) *cobra.Command {
	var project, status string
	cmd := &cobra.Command{
		Use: "list", Short: "List tracked features",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			features, err := store.ListFeatures(cmd.Context(), project, status)
			if err != nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, features)
			}
			if len(features) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching features.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-12s %-10s %-4s %s\n", "ID", "PROJECT", "STATUS", "SIZE", "TITLE")
			for _, f := range features {
				fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-12s %-10s %-4s %s\n", f.ID, truncateStr(f.Project, 12), f.Status, f.Size, f.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Filter by project")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	return cmd
}

func newFeatureShowCmd(outputFormat *string) *cobra.Command {
	return &cobra.Command{
		Use: "show <feature-id>", Short: "Show a feature timeline", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			detail, err := store.Detail(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, detail)
			}
			f := detail.Feature
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\nProject: %s\nStatus: %s", f.ID, f.Title, f.Project, f.Status)
			if f.Kind != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nType: %s", f.Kind)
			}
			if f.Size != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nSize: %s", f.Size)
			}
			if f.BudgetSeconds > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nBudget: %s", formatDuration(time.Duration(f.BudgetSeconds)*time.Second))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\n\nTimeline:")
			for _, event := range detail.Events {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-10s %s\n", event.CreatedAt.Local().Format("2006-01-02 15:04"), event.Kind, event.Note)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nSessions: %d  Models: %s  Scope changes: %d\n", len(detail.Sessions), formatSessionModels(detail.Sessions), len(detail.Scope))
			return nil
		},
	}
}

func newFeatureBindCmd() *cobra.Command {
	var session, source, model, external, repo, branch string
	cmd := &cobra.Command{
		Use: "bind <feature-id>", Short: "Bind an agent session to a feature", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if session == "" {
				session = strings.TrimSpace(os.Getenv("HARBOR_SESSION"))
			}
			if session == "" {
				session = strings.TrimSpace(external)
			}
			if model == "" {
				model = strings.TrimSpace(os.Getenv("HARBOR_MODEL"))
			}
			if session == "" {
				return fmt.Errorf("provide --session, --external-session, or set HARBOR_SESSION")
			}
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			err = store.BindSession(cmd.Context(), worklog.SessionBinding{FeatureID: args[0], HarborSessionID: session, Source: source, ModelName: model, ExternalSessionID: external, RepoPath: repo, Branch: branch})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bound session %s to %s\n", session, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&session, "session", "", "Harbor session ID (defaults to HARBOR_SESSION)")
	cmd.Flags().StringVar(&source, "source", "", "Agent source, such as codex or claude-code")
	cmd.Flags().StringVar(&model, "model", "", "Model name (defaults to HARBOR_MODEL)")
	cmd.Flags().StringVar(&external, "external-session", "", "Agent client's conversation or session ID")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository path")
	cmd.Flags().StringVar(&branch, "branch", "", "Git branch")
	return cmd
}

func newFeatureEventCmd(use, event, short string) *cobra.Command {
	var note, session string
	cmd := &cobra.Command{
		Use: use + " <feature-id>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if session == "" {
				session = strings.TrimSpace(os.Getenv("HARBOR_SESSION"))
			}
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			feature, err := store.AddEvent(cmd.Context(), args[0], event, note, session)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: recorded %s (status: %s)\n", feature.ID, event, feature.Status)
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional event note")
	cmd.Flags().StringVar(&session, "session", "", "Optional Harbor session ID")
	return cmd
}

func newFeatureScopeCmd() *cobra.Command {
	var decision string
	cmd := &cobra.Command{
		Use: "scope <feature-id> <description>", Short: "Record an explicit scope decision", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := worklog.NewStore()
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.AddScope(cmd.Context(), args[0], decision, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded %s scope for %s\n", decision, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&decision, "decision", "", "Scope decision: include, swap, defer, or reject")
	_ = cmd.MarkFlagRequired("decision")
	return cmd
}

func parseWorklogDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "d") {
		days := strings.TrimSuffix(value, "d")
		parsed, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, err
		}
		return parsed * 24, nil
	}
	return time.ParseDuration(value)
}

func formatSessionModels(sessions []worklog.SessionBinding) string {
	seen := make(map[string]bool)
	var models []string
	for _, session := range sessions {
		model := strings.TrimSpace(session.ModelName)
		if model != "" && !seen[model] {
			seen[model] = true
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		return "-"
	}
	return strings.Join(models, ", ")
}

func printJSON(cmd *cobra.Command, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
