package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newUpdateCmd(version, sourceDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Rebuild and reinstall harbor from source",
		Long: `Rebuild harbor from source and replace the current binary.

This command requires that harbor was built with source directory information
embedded (via make build / make install). It runs 'go build' in the source
directory and copies the new binary over the currently running one.

Examples:
  harbor update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sourceDir == "" {
				return fmt.Errorf("source directory not embedded in this build — rebuild with: make install")
			}

			// Verify source directory exists
			goMod := filepath.Join(sourceDir, "go.mod")
			if _, err := os.Stat(goMod); err != nil {
				return fmt.Errorf("source directory %s not found (moved or deleted?)", sourceDir)
			}

			// Find where the current binary is
			currentBin, err := os.Executable()
			if err != nil {
				return fmt.Errorf("cannot determine current binary path: %w", err)
			}
			currentBin, err = filepath.EvalSymlinks(currentBin)
			if err != nil {
				return fmt.Errorf("resolving binary path: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Current: harbor %s\n", version)
			fmt.Fprintf(cmd.OutOrStdout(), "Source:  %s\n", sourceDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Target:  %s\n\n", currentBin)

			// Get new version from git
			newVersion := "dev"
			gitCmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
			gitCmd.Dir = sourceDir
			if out, err := gitCmd.Output(); err == nil {
				newVersion = strings.TrimSpace(string(out))
			}

			// Build to a temp file next to the target
			tmpBin := currentBin + ".new"
			ldflags := fmt.Sprintf("-s -w -X main.version=%s -X main.sourceDir=%s", newVersion, sourceDir)
			buildCmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", tmpBin, "./cmd/harbor")
			buildCmd.Dir = sourceDir
			buildCmd.Stdout = cmd.OutOrStdout()
			buildCmd.Stderr = cmd.ErrOrStderr()

			fmt.Fprintf(cmd.OutOrStdout(), "Building %s...\n", newVersion)
			if err := buildCmd.Run(); err != nil {
				os.Remove(tmpBin)
				return fmt.Errorf("build failed: %w", err)
			}

			// Replace current binary
			if err := os.Rename(tmpBin, currentBin); err != nil {
				os.Remove(tmpBin)
				return fmt.Errorf("replacing binary: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nUpdated: harbor %s -> %s\n", version, newVersion)
			return nil
		},
	}
}
