package worklog

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestFeatureLifecycleReport(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	t0 := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return t0 }
	feature, err := store.CreateFeature(ctx, "harbor", "Track feature cycles", "feature", "m", 48*time.Hour, "2026-07-18")
	if err != nil {
		t.Fatal(err)
	}
	if feature.Size != "M" || feature.Status != StatusActive || feature.TargetDate != "2026-07-18" {
		t.Fatalf("unexpected feature: %+v", feature)
	}

	if _, err := store.addEventAt(ctx, feature.ID, EventShipped, "", "", t0.Add(time.Hour)); err == nil {
		t.Fatal("shipping an unverified feature should fail")
	}
	if _, err := store.addEventAt(ctx, feature.ID, EventBlocked, "waiting", "ses_1", t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.addEventAt(ctx, feature.ID, EventResumed, "", "ses_1", t0.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}

	store.now = func() time.Time { return t0.Add(6 * time.Hour) }
	if err := store.BindSession(ctx, SessionBinding{FeatureID: feature.ID, HarborSessionID: "ses_1", Source: "codex"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddScope(ctx, feature.ID, "include", "weekly report"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddScope(ctx, feature.ID, "defer", "calendar sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.addEventAt(ctx, feature.ID, EventVerified, "tests pass", "ses_1", t0.Add(8*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.addEventAt(ctx, feature.ID, EventShipped, "released", "ses_1", t0.Add(10*time.Hour)); err != nil {
		t.Fatal(err)
	}

	report, err := store.BuildReport(ctx, t0.Add(-time.Hour), t0.Add(12*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if report.ShippedCount != 1 || len(report.Features) != 1 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
	stats := report.Features[0]
	if stats.CycleSeconds != int64((10 * time.Hour).Seconds()) {
		t.Fatalf("cycle seconds = %d, want 36000", stats.CycleSeconds)
	}
	if stats.BlockedSeconds != int64((3 * time.Hour).Seconds()) {
		t.Fatalf("blocked seconds = %d, want 10800", stats.BlockedSeconds)
	}
	if stats.VerificationLagSeconds != int64((2 * time.Hour).Seconds()) {
		t.Fatalf("verification lag seconds = %d, want 7200", stats.VerificationLagSeconds)
	}
	if stats.SessionCount != 1 || stats.ScopeAdded != 1 || stats.ScopeDeferred != 1 {
		t.Fatalf("unexpected attribution stats: %+v", stats)
	}
}

func TestFeatureTargetDateCanBeRescheduledAndCleared(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	feature, err := store.CreateFeature(ctx, "studio", "Portfolio timeline", "feature", "M", 0, "2026-08-18")
	if err != nil {
		t.Fatal(err)
	}
	if feature.TargetDate != "2026-08-18" {
		t.Fatalf("target date = %q, want 2026-08-18", feature.TargetDate)
	}

	feature, err = store.SetTargetDate(ctx, feature.ID, "2026-08-21")
	if err != nil {
		t.Fatal(err)
	}
	if feature.TargetDate != "2026-08-21" {
		t.Fatalf("rescheduled target date = %q, want 2026-08-21", feature.TargetDate)
	}
	listed, err := store.ListFeatures(ctx, "studio", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].TargetDate != "2026-08-21" {
		t.Fatalf("unexpected listed feature: %+v", listed)
	}
	detail, err := store.Detail(ctx, feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Feature.TargetDate != "2026-08-21" {
		t.Fatalf("detail target date = %q, want 2026-08-21", detail.Feature.TargetDate)
	}

	feature, err = store.SetTargetDate(ctx, feature.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if feature.TargetDate != "" {
		t.Fatalf("cleared target date = %q, want empty", feature.TargetDate)
	}
}

func TestFeatureTargetDateValidation(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for _, target := range []string{"2026/08/18", "2026-8-18", "2026-02-30"} {
		if _, err := store.CreateFeature(ctx, "studio", "Invalid target", "", "", 0, target); err == nil {
			t.Fatalf("CreateFeature accepted invalid target date %q", target)
		}
	}
	feature, err := store.CreateFeature(ctx, "studio", "Valid target", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetTargetDate(ctx, feature.ID, "not-a-date"); err == nil {
		t.Fatal("SetTargetDate accepted an invalid target date")
	}
	if _, err := store.SetTargetDate(ctx, "feat_missing", "2026-08-18"); err == nil {
		t.Fatal("SetTargetDate accepted a missing feature")
	}
}

func TestEstimateUsesShippedMatchingFeatures(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i, cycle := range []time.Duration{24 * time.Hour, 48 * time.Hour, 96 * time.Hour} {
		start := t0.Add(time.Duration(i) * 7 * 24 * time.Hour)
		store.now = func() time.Time { return start }
		feature, err := store.CreateFeature(ctx, "harbor", "Feature", "integration", "M", 0, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.addEventAt(ctx, feature.ID, EventVerified, "", "", start.Add(cycle-time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.addEventAt(ctx, feature.ID, EventShipped, "", "", start.Add(cycle)); err != nil {
			t.Fatal(err)
		}
	}

	estimate, err := store.Estimate(ctx, "harbor", "integration", "m")
	if err != nil {
		t.Fatal(err)
	}
	if estimate.Samples != 3 || estimate.P50CycleSeconds != int64((48*time.Hour).Seconds()) || estimate.P80CycleSeconds != int64((96*time.Hour).Seconds()) {
		t.Fatalf("unexpected estimate: %+v", estimate)
	}
}

func TestScopeValidationAndSessionDeduplication(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	feature, err := store.CreateFeature(ctx, "harbor", "Feature", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddScope(ctx, feature.ID, "maybe", "unbounded idea"); err == nil {
		t.Fatal("invalid scope decision should fail")
	}
	if _, err := store.AddEvent(ctx, feature.ID, EventVerified, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.AddScope(ctx, feature.ID, "include", "late expansion"); err == nil {
		t.Fatal("verified feature should require reopen before scope expansion")
	}
	if err := store.AddScope(ctx, feature.ID, "defer", "future idea"); err != nil {
		t.Fatalf("deferring scope after verification should be allowed: %v", err)
	}
	binding := SessionBinding{FeatureID: feature.ID, HarborSessionID: "ses_1", Source: "codex", ExternalSessionID: "conv_1"}
	if err := store.BindSession(ctx, binding); err != nil {
		t.Fatal(err)
	}
	binding.ModelName = "gpt-5"
	if err := store.BindSession(ctx, binding); err != nil {
		t.Fatal(err)
	}
	detail, err := store.Detail(ctx, feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(detail.Sessions))
	}
	if detail.Sessions[0].ModelName != "gpt-5" {
		t.Fatalf("model = %q, want gpt-5", detail.Sessions[0].ModelName)
	}
}

func TestReopenedFeatureContinuesCycle(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return t0 }
	feature, err := store.CreateFeature(ctx, "harbor", "Feature", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct {
		kind string
		at   time.Time
	}{
		{EventVerified, t0.Add(2 * time.Hour)},
		{EventShipped, t0.Add(3 * time.Hour)},
		{EventReopened, t0.Add(4 * time.Hour)},
	} {
		if _, err := store.addEventAt(ctx, feature.ID, event.kind, "", "", event.at); err != nil {
			t.Fatal(err)
		}
	}

	detail, err := store.Detail(ctx, feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	stats := calculateStats(detail, t0.Add(8*time.Hour))
	if stats.CycleSeconds != int64((8 * time.Hour).Seconds()) {
		t.Fatalf("cycle seconds = %d, want 28800", stats.CycleSeconds)
	}
	if stats.VerificationLagSeconds != 0 {
		t.Fatalf("verification lag seconds = %d, want 0 after reopen", stats.VerificationLagSeconds)
	}
}

func TestFeatureEventCommitEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	feature, err := store.CreateFeature(ctx, "harbor", "Commit evidence", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddEventWithCommit(ctx, feature.ID, EventCheckpoint, "tests pass", "", "ABCDEF1234567"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddEventWithCommit(ctx, feature.ID, EventCheckpoint, "", "", "HEAD"); err == nil {
		t.Fatal("symbolic commit reference should be rejected")
	}
	detail, err := store.Detail(ctx, feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := detail.Events[len(detail.Events)-1]
	if got.CommitSHA != "abcdef1234567" || got.Note != "tests pass" {
		t.Fatalf("unexpected commit evidence: %+v", got)
	}
}

func TestCheckpointSummaryIsStructuredAndIdempotent(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	feature, err := store.CreateFeature(ctx, "harbor", "Checkpoint summaries", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	base := "0123456789abcdef0123456789abcdef01234567"
	head := "89abcdef0123456789abcdef0123456789abcdef"
	first, err := store.UpsertCheckpointSummary(ctx, CheckpointSummary{
		FeatureID: feature.ID, RepoPath: "/tmp/repo", BaseSHA: base, HeadSHA: head,
		Outcome: "Delivered the summary protocol", Decisions: []string{"Git remains authoritative", "Git remains authoritative"},
		Verification: []string{"go test ./internal/worklog"}, SessionID: "thr_1", Source: "codex", ModelName: "gpt-5",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.UpsertCheckpointSummary(ctx, CheckpointSummary{
		FeatureID: feature.ID, RepoPath: "/tmp/repo", BaseSHA: base, HeadSHA: head,
		Outcome: "Delivered and verified the summary protocol", Remaining: []string{"Studio rendering"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("summary id changed across upsert: %d != %d", first.ID, second.ID)
	}
	detail, err := store.Detail(ctx, feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.CheckpointSummaries) != 1 {
		t.Fatalf("summaries = %d, want 1", len(detail.CheckpointSummaries))
	}
	got := detail.CheckpointSummaries[0]
	if got.Outcome != "Delivered and verified the summary protocol" || len(got.Remaining) != 1 || got.SchemaVersion != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestResolveFeatureContextUsesDeterministicOrder(t *testing.T) {
	ctx := context.Background()
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	feature, err := store.CreateFeature(ctx, "harbor", "Context resolution", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindSession(ctx, SessionBinding{FeatureID: feature.ID, HarborSessionID: "thr_1", ExternalSessionID: "external_1", RepoPath: "/tmp/repo", Branch: "feat/test"}); err != nil {
		t.Fatal(err)
	}
	bySession, err := store.ResolveFeatureContext(ctx, "external_1", "/other", "main", "other")
	if err != nil || bySession.Match != "session" || bySession.Detail == nil || bySession.Detail.Feature.ID != feature.ID {
		t.Fatalf("unexpected session context: %+v, %v", bySession, err)
	}
	byRepo, err := store.ResolveFeatureContext(ctx, "", "/tmp/repo", "feat/test", "other")
	if err != nil || byRepo.Match != "repository" || byRepo.Detail == nil || byRepo.Detail.Feature.ID != feature.ID {
		t.Fatalf("unexpected repository context: %+v, %v", byRepo, err)
	}
	byProject, err := store.ResolveFeatureContext(ctx, "", "", "", "harbor")
	if err != nil || byProject.Match != "project" || byProject.Detail == nil || byProject.Detail.Feature.ID != feature.ID {
		t.Fatalf("unexpected project context: %+v, %v", byProject, err)
	}
}

func TestMigratesV2EventTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worklog.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE schema_version (version INTEGER NOT NULL)`,
		`INSERT INTO schema_version(version) VALUES (2)`,
		`CREATE TABLE features (id TEXT PRIMARY KEY, project TEXT NOT NULL, title TEXT NOT NULL, kind TEXT NOT NULL DEFAULT '', size TEXT NOT NULL DEFAULT '', budget_seconds INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE feature_events (id INTEGER PRIMARY KEY AUTOINCREMENT, feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE, kind TEXT NOT NULL, note TEXT NOT NULL DEFAULT '', session_id TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL)`,
		`INSERT INTO features VALUES ('feat_old', 'harbor', 'Old feature', '', '', 0, 'active', '2026-07-01T00:00:00Z', '2026-07-01T00:00:00Z')`,
		`INSERT INTO feature_events(feature_id, kind, note, created_at) VALUES ('feat_old', 'checkpoint', 'legacy proof', '2026-07-01T01:00:00Z')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := NewStoreAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	detail, err := store.Detail(context.Background(), "feat_old")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Events) != 1 || detail.Events[0].CommitSHA != "" {
		t.Fatalf("unexpected migrated event: %+v", detail.Events)
	}
}

func TestMigratesV1SessionTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worklog.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE schema_version (version INTEGER NOT NULL)`,
		`INSERT INTO schema_version(version) VALUES (1)`,
		`CREATE TABLE features (id TEXT PRIMARY KEY, project TEXT NOT NULL, title TEXT NOT NULL, kind TEXT NOT NULL DEFAULT '', size TEXT NOT NULL DEFAULT '', budget_seconds INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE feature_sessions (feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE, harbor_session_id TEXT NOT NULL, source TEXT NOT NULL DEFAULT '', external_session_id TEXT NOT NULL DEFAULT '', repo_path TEXT NOT NULL DEFAULT '', branch TEXT NOT NULL DEFAULT '', bound_at TEXT NOT NULL, UNIQUE(feature_id, harbor_session_id, source, external_session_id))`,
		`INSERT INTO features VALUES ('feat_old', 'harbor', 'Old feature', '', '', 0, 'active', '2026-07-01T00:00:00Z', '2026-07-01T00:00:00Z')`,
		`INSERT INTO feature_sessions VALUES ('feat_old', 'ses_old', 'codex', 'conv_old', '', '', '2026-07-01T00:00:00Z')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := NewStoreAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	detail, err := store.Detail(context.Background(), "feat_old")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Sessions) != 1 || detail.Sessions[0].ModelName != "" {
		t.Fatalf("unexpected migrated session: %+v", detail.Sessions)
	}
	if detail.Feature.TargetDate != "" {
		t.Fatalf("migrated target date = %q, want empty", detail.Feature.TargetDate)
	}
	if err := store.BindSession(context.Background(), SessionBinding{FeatureID: "feat_old", HarborSessionID: "ses_old", Source: "codex", ExternalSessionID: "conv_old", ModelName: "gpt-5"}); err != nil {
		t.Fatal(err)
	}
	detail, err = store.Detail(context.Background(), "feat_old")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Sessions[0].ModelName != "gpt-5" {
		t.Fatalf("model = %q, want gpt-5", detail.Sessions[0].ModelName)
	}
}
