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
		newFarmWatchCmd(version),
		newFarmPlantCmd(version),
		newFarmHarvestCmd(version),
		newFarmConnectCmd(version),
		newFarmVisitCmd(outputFormat, version),
		newFarmForageCmd(outputFormat, version),
		newFarmEventCmd(version),
		newFarmTelemetryCmd(version),
	)
	return cmd
}

func newFarmWatchCmd(version string) *cobra.Command {
	var interval time.Duration
	var noClear bool
	cmd := &cobra.Command{
		Use: "watch", Short: "Keep a compact Farm view open beside a running agent",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if interval < time.Second {
				return fmt.Errorf("interval must be at least 1s")
			}
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			for {
				bootstrap, loadErr := client.Bootstrap(cmd.Context())
				if loadErr != nil && bootstrap == nil {
					return loadErr
				}
				if !noClear {
					fmt.Fprint(cmd.OutOrStdout(), "\x1b[2J\x1b[H")
				}
				renderFarmWatch(cmd, bootstrap, loadErr)
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 3*time.Second, "Refresh interval")
	cmd.Flags().BoolVar(&noClear, "no-clear", false, "Append snapshots instead of redrawing")
	return cmd
}

func renderFarmWatch(cmd *cobra.Command, bootstrap *farm.Bootstrap, loadErr error) {
	fmt.Fprintf(cmd.OutOrStdout(), "HARBOR FARM  L%d  %d coins  %d XP\n", bootstrap.Profile.Level, bootstrap.Profile.Coins, bootstrap.Profile.XP)
	fmt.Fprintf(cmd.OutOrStdout(), "Agents %d  Today %d tokens  Neighbors %d\n\n", len(bootstrap.ActiveSessions), bootstrap.TodayUsage.InputTokens+bootstrap.TodayUsage.OutputTokens, len(bootstrap.Social.Neighbors))
	if len(bootstrap.SessionCrops) > 0 {
		crop := bootstrap.SessionCrops[0]
		filled := min(20, max(0, crop.Progress/5))
		fmt.Fprintf(cmd.OutOrStdout(), "Session crop  %-24s [%s%s] %3d%%\n", crop.DisplayName, strings.Repeat("#", filled), strings.Repeat("-", 20-filled), crop.Progress)
		fmt.Fprintf(cmd.OutOrStdout(), "              %s · %s\n\n", crop.Stage, crop.Rarity)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Session crop  waiting for a connected agent")
		fmt.Fprintln(cmd.OutOrStdout())
	}
	for _, plot := range bootstrap.Plots {
		crop := "open soil"
		state := "plant with: harbor farm plant " + strconv.Itoa(plot.PlotIndex) + " wheat"
		if plot.CropType != nil {
			crop = *plot.CropType
			if plot.IsReady {
				state = "READY · harbor farm harvest " + strconv.Itoa(plot.PlotIndex)
			} else {
				state = fmt.Sprintf("%ds", plot.RemainingSeconds)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%d] %-12s %s\n", plot.PlotIndex, crop, state)
	}
	if loadErr != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nOffline cache: %v\n", loadErr)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nCtrl+C closes this view. Agent execution is unaffected.")
	}
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
			fmt.Fprintf(cmd.OutOrStdout(), "Session nursery: %d crops · %d neighbors · farm code %s\n", len(bootstrap.SessionCrops), len(bootstrap.Social.Neighbors), bootstrap.Social.FarmCode)
			if len(bootstrap.SessionCrops) > 0 {
				latest := bootstrap.SessionCrops[0]
				fmt.Fprintf(cmd.OutOrStdout(), "Latest: %s · %s · %d%%\n", latest.DisplayName, latest.Stage, latest.Progress)
			}
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Offline cache: %v\n", err)
			}
			return nil
		},
	}
}

func newFarmConnectCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use: "connect <farm-code>", Short: "Connect a Harbor Farm neighbor", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			farmCode := strings.ToUpper(strings.TrimSpace(args[0]))
			if len(farmCode) != 8 {
				return fmt.Errorf("farm code must be 8 characters")
			}
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			if err := client.ConnectNeighbor(cmd.Context(), farmCode); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Connected to farm %s.\n", farmCode)
			return nil
		},
	}
}

func newFarmVisitCmd(outputFormat *string, version string) *cobra.Command {
	return &cobra.Command{
		Use: "visit <farm-code>", Short: "Visit a connected neighbor's farm", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			farmCode := strings.ToUpper(strings.TrimSpace(args[0]))
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			neighbor, err := client.VisitNeighbor(cmd.Context(), farmCode)
			if err != nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, neighbor)
			}
			ready := 0
			for _, plot := range neighbor.Plots {
				if plot.IsReady {
					ready++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s · Level %d · %d ready plots · %d session crops\n", neighbor.DisplayName, neighbor.Level, ready, len(neighbor.SessionCrops))
			return nil
		},
	}
}

func newFarmForageCmd(outputFormat *string, version string) *cobra.Command {
	return &cobra.Command{
		Use: "forage <farm-code> <plot>", Short: "Gather a clipping from a neighbor's ready crop", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			farmCode := strings.ToUpper(strings.TrimSpace(args[0]))
			plot, err := strconv.Atoi(args[1])
			if err != nil || plot < 0 || plot >= 6 {
				return fmt.Errorf("plot must be 0-5")
			}
			client, err := farmClient(version)
			if err != nil {
				return err
			}
			result, err := client.ForageNeighbor(cmd.Context(), farmCode, plot)
			if err != nil {
				return err
			}
			if *outputFormat == "json" {
				return printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Gathered %s from plot %d · +%d coins · %d clippings remain.\n", result.CropType, result.PlotIndex, result.Reward, result.RemainingClippings)
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
