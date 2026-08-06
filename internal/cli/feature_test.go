package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/oseaitic/harbor/internal/worklog"
)

func TestParseWorklogDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"90m":  90 * time.Minute,
		"2d":   48 * time.Hour,
		"1.5d": 36 * time.Hour,
	}
	for input, want := range tests {
		got, err := parseWorklogDuration(input)
		if err != nil {
			t.Fatalf("parseWorklogDuration(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseWorklogDuration(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestFeatureCheckpointCommitFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HARBOR_HOME", home)
	store, err := worklog.NewStoreAt(filepath.Join(home, "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	feature, err := store.CreateFeature(context.Background(), "harbor", "CLI commit evidence", "", "", 0, "")
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := newFeatureEventCmd("checkpoint", worklog.EventCheckpoint, "Record a working checkpoint", true)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{feature.ID, "--commit", "0123456789abcdef", "--note", "tests pass"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	store, err = worklog.NewStoreAt(filepath.Join(home, "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	detail, err := store.Detail(context.Background(), feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	event := detail.Events[len(detail.Events)-1]
	if event.CommitSHA != "0123456789abcdef" || event.Note != "tests pass" {
		t.Fatalf("unexpected checkpoint: %+v", event)
	}
}

func TestFeaturePlanSetsAndClearsTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HARBOR_HOME", home)
	store, err := worklog.NewStoreAt(filepath.Join(home, "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	feature, err := store.CreateFeature(context.Background(), "harbor", "CLI delivery target", "", "", 0, "")
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	format := "json"
	plan := newFeaturePlanCmd(&format)
	plan.SetOut(&bytes.Buffer{})
	plan.SetArgs([]string{feature.ID, "--target", "2026-08-18"})
	if err := plan.Execute(); err != nil {
		t.Fatal(err)
	}

	store, err = worklog.NewStoreAt(filepath.Join(home, "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	planned, err := store.GetFeature(context.Background(), feature.ID)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if planned.TargetDate != "2026-08-18" {
		store.Close()
		t.Fatalf("target date = %q, want 2026-08-18", planned.TargetDate)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	clear := newFeaturePlanCmd(&format)
	clear.SetOut(&bytes.Buffer{})
	clear.SetArgs([]string{feature.ID, "--clear-target"})
	if err := clear.Execute(); err != nil {
		t.Fatal(err)
	}

	store, err = worklog.NewStoreAt(filepath.Join(home, "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	planned, err = store.GetFeature(context.Background(), feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if planned.TargetDate != "" {
		t.Fatalf("cleared target date = %q, want empty", planned.TargetDate)
	}
}

func TestFeatureCheckpointFinalizeCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HARBOR_HOME", home)
	repo := t.TempDir()
	runFeatureGit(t, repo, "init")
	runFeatureGit(t, repo, "config", "user.name", "Harbor Test")
	runFeatureGit(t, repo, "config", "user.email", "harbor@example.com")
	writeFeatureCommit(t, repo, "one.txt", "one", "one")
	base := featureGitValue(t, repo, "rev-parse", "HEAD")
	writeFeatureCommit(t, repo, "two.txt", "two", "two")
	head := featureGitValue(t, repo, "rev-parse", "HEAD")

	store, err := worklog.NewStoreAt(filepath.Join(home, "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	feature, err := store.CreateFeature(context.Background(), filepath.Base(repo), "CLI summary", "", "", 0, "")
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	format := "json"
	cmd := newFeatureCheckpointFinalizeCmd(&format)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{feature.ID, "--repo", repo, "--base", base, "--head", head, "--outcome", "Delivered structured summaries", "--decision", "Keep Git authoritative", "--verification", "go test ./...", "--remaining", "Studio rendering", "--source", "codex", "--session", "thr_1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"outcome": "Delivered structured summaries"`)) {
		t.Fatalf("unexpected output: %s", output.String())
	}

	store, err = worklog.NewStoreAt(filepath.Join(home, "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	detail, err := store.Detail(context.Background(), feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.CheckpointSummaries) != 1 || detail.CheckpointSummaries[0].HeadSHA != head {
		t.Fatalf("unexpected summaries: %+v", detail.CheckpointSummaries)
	}
}

func writeFeatureCommit(t *testing.T, repo, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runFeatureGit(t, repo, "add", name)
	runFeatureGit(t, repo, "commit", "-m", message)
}

func featureGitValue(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace(out))
}

func runFeatureGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
