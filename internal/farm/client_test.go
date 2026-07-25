package farm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/oseaitic/harbor/internal/cloudauth"
)

func TestClientRecordsThroughHarborCloudAccount(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())
	var mu sync.Mutex
	registered := false
	received := make([]Event, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "hbr_test" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/farm/devices/register":
			registered = true
			json.NewEncoder(w).Encode(map[string]string{"device_id": "00000000-0000-0000-0000-000000000001"})
		case "/api/farm/events/batch":
			var batch struct {
				Events []Event `json:"events"`
			}
			if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
				t.Error(err)
				http.Error(w, "bad", 400)
				return
			}
			mu.Lock()
			received = append(received, batch.Events...)
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]int{"accepted": len(batch.Events)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(&cloudauth.Config{Endpoint: server.URL, APIKey: "hbr_test"}, "test")
	event := Event{EventID: "evt-12345678", ExternalSessionID: "session-1", Source: "codex", Surface: "test", EventType: "usage", InputTokensDelta: 10, OutputTokensDelta: 2, UsageQuality: "exact", OccurredAt: time.Now().UTC()}
	if err := client.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("device was not registered")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0].EventID != event.EventID {
		t.Fatalf("received %#v", received)
	}
	entries, err := os.ReadDir(filepath.Join(os.Getenv("HARBOR_HOME"), "farm-queue"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("sent event remained queued: %#v", entries)
	}
}

func TestClientKeepsEventWhenCloudIsOffline(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())
	client := NewClient(&cloudauth.Config{Endpoint: "http://127.0.0.1:1", APIKey: "hbr_test"}, "test")
	client.HTTPClient.Timeout = 100 * time.Millisecond
	event := Event{EventID: "evt-offline-123", ExternalSessionID: "session-1", Source: "codex", Surface: "test", EventType: "status", State: "waiting_input", UsageQuality: "unavailable", OccurredAt: time.Now().UTC()}
	if err := client.Record(context.Background(), event); err == nil {
		t.Fatal("expected offline error")
	}
	entries, err := os.ReadDir(filepath.Join(os.Getenv("HARBOR_HOME"), "farm-queue"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d queued files, want 1", len(entries))
	}
}
