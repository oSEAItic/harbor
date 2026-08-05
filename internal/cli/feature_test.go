package cli

import (
	"bytes"
	"context"
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
	feature, err := store.CreateFeature(context.Background(), "harbor", "CLI commit evidence", "", "", 0)
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
