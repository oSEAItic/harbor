package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"os/exec"
)

const githubRepo = "oseaitic/harbor"

func newUpdateCmd(version, sourceDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update harbor to the latest version",
		Long: `Update harbor to the latest released version.

If harbor was installed from source (make install), rebuilds from source.
Otherwise, downloads the latest binary from GitHub releases.

Examples:
  harbor update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sourceDir != "" {
				return updateFromSource(cmd, version, sourceDir)
			}
			return updateFromGitHub(cmd, version)
		},
	}
}

// updateFromSource rebuilds harbor from the embedded source directory.
func updateFromSource(cmd *cobra.Command, version, sourceDir string) error {
	goMod := filepath.Join(sourceDir, "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		return fmt.Errorf("source directory %s not found (moved or deleted?)", sourceDir)
	}

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

	newVersion := "dev"
	gitCmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	gitCmd.Dir = sourceDir
	if out, err := gitCmd.Output(); err == nil {
		newVersion = strings.TrimSpace(string(out))
	}

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

	if err := os.Rename(tmpBin, currentBin); err != nil {
		os.Remove(tmpBin)
		return fmt.Errorf("replacing binary: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nUpdated: harbor %s -> %s\n", version, newVersion)
	return nil
}

// updateFromGitHub downloads the latest release binary from GitHub.
func updateFromGitHub(cmd *cobra.Command, currentVersion string) error {
	out := cmd.OutOrStdout()

	// Detect platform
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	switch goos {
	case "darwin", "linux":
	default:
		return fmt.Errorf("unsupported OS: %s", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("unsupported architecture: %s", goarch)
	}

	fmt.Fprintf(out, "Current: harbor %s\n", currentVersion)
	fmt.Fprintf(out, "Fetching latest release from GitHub...\n")

	latestVersion, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("fetching latest release: %w", err)
	}

	if latestVersion == currentVersion || "v"+currentVersion == latestVersion {
		fmt.Fprintf(out, "Already up to date (harbor %s).\n", currentVersion)
		return nil
	}

	tag := latestVersion
	ver := strings.TrimPrefix(tag, "v")
	filename := fmt.Sprintf("harbor_%s_%s_%s.tar.gz", ver, goos, goarch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, tag, filename)

	fmt.Fprintf(out, "Downloading harbor %s (%s/%s)...\n", tag, goos, goarch)

	tmpDir, err := os.MkdirTemp("", "harbor-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, filename)
	if err := downloadFile(url, archivePath); err != nil {
		return fmt.Errorf("downloading: %w", err)
	}

	newBinPath := filepath.Join(tmpDir, "harbor")
	if err := extractBinary(archivePath, newBinPath); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}

	currentBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	currentBin, err = filepath.EvalSymlinks(currentBin)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	// Write to a temp file next to the target, then rename atomically.
	tmpBin := currentBin + ".new"
	data, err := os.ReadFile(newBinPath)
	if err != nil {
		return fmt.Errorf("reading downloaded binary: %w", err)
	}
	if err := os.WriteFile(tmpBin, data, 0o755); err != nil {
		return fmt.Errorf("writing new binary: %w", err)
	}
	if err := os.Rename(tmpBin, currentBin); err != nil {
		os.Remove(tmpBin)
		return fmt.Errorf("replacing binary (try with sudo): %w", err)
	}

	fmt.Fprintf(out, "\nUpdated: harbor %s -> %s\n", currentVersion, tag)
	return nil
}

func fetchLatestRelease() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("no releases found")
	}
	return payload.TagName, nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == "harbor" || strings.HasSuffix(hdr.Name, "/harbor") {
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			return err
		}
	}
	return fmt.Errorf("harbor binary not found in archive")
}
