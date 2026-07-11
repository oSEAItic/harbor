package worklog

import (
	"context"
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
	feature, err := store.CreateFeature(ctx, "harbor", "Track feature cycles", "feature", "m", 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if feature.Size != "M" || feature.Status != StatusActive {
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
		feature, err := store.CreateFeature(ctx, "harbor", "Feature", "integration", "M", 0)
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

	feature, err := store.CreateFeature(ctx, "harbor", "Feature", "", "", 0)
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
	feature, err := store.CreateFeature(ctx, "harbor", "Feature", "", "", 0)
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
