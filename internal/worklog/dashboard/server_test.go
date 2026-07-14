package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oseaitic/harbor/internal/worklog"
)

func TestDashboardHandler(t *testing.T) {
	store, err := worklog.NewStoreAt(filepath.Join(t.TempDir(), "worklog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	feature, err := store.CreateFeature(context.Background(), "harbor", "Feature calendar", "feature", "M", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindSession(context.Background(), worklog.SessionBinding{FeatureID: feature.ID, HarborSessionID: "ses_1", Source: "codex"}); err != nil {
		t.Fatal(err)
	}

	handler, err := Handler(store)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	response, err := http.Get(server.URL + "/api/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	var data Data
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if len(data.Features) != 1 || data.Features[0].Feature.ID != feature.ID || data.Features[0].SessionCount != 1 {
		t.Fatalf("unexpected dashboard data: %+v", data)
	}

	indexResponse, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer indexResponse.Body.Close()
	if got := indexResponse.Header.Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("missing CSP: %q", got)
	}
}
