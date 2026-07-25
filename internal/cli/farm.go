package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/farm"
	"github.com/spf13/cobra"
)

func newFarmCmd(outputFormat *string, version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "farm",
		Short: "Turn verified agent activity into Harbor Farm progress",
		Long: `Harbor Farm uses your existing Harbor Cloud account.

Only metadata and token counts are accepted. Prompts, outputs, tool arguments,
and file content are not part of the event contract. Waiting earns no coins.`,
	}
	cmd.AddCommand(
		newFarmStatusCmd(outputFormat, version),
		newFarmPlantCmd(version),
		newFarmHarvestCmd(version),
		newFarmEventCmd(version),
		newFarmTelemetryCmd(version),
	)
	return cmd
}

func newFarmTelemetryCmd(version string) *cobra.Command {
	cmd := &cobra.Command{Use: "telemetry", Short: "Receive opt-in metadata from external agents"}
	cmd.AddCommand(&cobra.Command{
		Use: "serve", Short: "Serve the private localhost OTLP JSON receiver",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			receiver := farm.NewTelemetryReceiver(client)
			return receiver.Serve(cmd.Context(), func(status farm.TelemetryStatus) {
				data, _ := json.Marshal(status)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			})
		},
	})
	return cmd
}

func farmClient(version string) (*farm.Client, error) {
	cfg, err := cloudauth.Load()
	if err != nil {
		return nil, fmt.Errorf("Harbor Cloud is not connected; run 'harbor cloud enable' or 'harbor login'")
	}
	return farm.NewClient(cfg, version), nil
}

func newFarmStatusCmd(outputFormat *string, version string) *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show Farm profile, plots, and active agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			bootstrap, err := client.Bootstrap(cmd.Context())
			if err != nil && bootstrap == nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, bootstrap)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Harbor Farm · Level %d · %d coins · %d XP\n", bootstrap.Profile.Level, bootstrap.Profile.Coins, bootstrap.Profile.XP)
			fmt.Fprintf(cmd.OutOrStdout(), "Today: %d input + %d output tokens · %d active agents\n", bootstrap.TodayUsage.InputTokens, bootstrap.TodayUsage.OutputTokens, len(bootstrap.ActiveSessions))
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Offline cache: %v\n", err)
			}
			return nil
		},
	}
}

func newFarmPlantCmd(version string) *cobra.Command {
	var idempotencyKey string
	cmd := &cobra.Command{
		Use: "plant <plot> <wheat|carrot|tomato>", Short: "Plant a crop",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			plot, err := strconv.Atoi(args[0])
			if err != nil || plot < 0 || plot >= 6 {
				return fmt.Errorf("plot must be 0-5")
			}
			crop := strings.ToLower(args[1])
			if crop != "wheat" && crop != "carrot" && crop != "tomato" {
				return fmt.Errorf("crop must be wheat, carrot, or tomato")
			}
			if idempotencyKey == "" {
				idempotencyKey = uuid.NewString()
			}
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			if err := client.Plant(cmd.Context(), plot, crop, idempotencyKey); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Planted %s in plot %d.\n", crop, plot)
			return nil
		},
	}
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Stable retry key")
	return cmd
}

func newFarmHarvestCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use: "harvest <plot>", Short: "Harvest a ready crop", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plot, err := strconv.Atoi(args[0])
			if err != nil || plot < 0 || plot >= 6 {
				return fmt.Errorf("plot must be 0-5")
			}
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			if err := client.Harvest(cmd.Context(), plot); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Harvested plot %d.\n", plot)
			return nil
		},
	}
}

func newFarmEventCmd(version string) *cobra.Command {
	var raw string
	cmd := &cobra.Command{
		Use: "event", Short: "Queue one metadata-only agent event",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var event farm.Event
			if err := json.Unmarshal([]byte(raw), &event); err != nil {
				return fmt.Errorf("invalid --json event: %w", err)
			}
			if event.EventID == "" {
				event.EventID = uuid.NewString()
			}
			if event.OccurredAt.IsZero() {
				event.OccurredAt = time.Now().UTC()
			}
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			return client.Record(cmd.Context(), event)
		},
	}
	cmd.Flags().StringVar(&raw, "json", "", "Metadata-only event JSON")
	_ = cmd.MarkFlagRequired("json")
	return cmd
}
