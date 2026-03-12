package cloudauth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateShare(t *testing.T) {
	want := ShareResult{
		Token:     "abc123",
		URL:       "https://harbor.example/s/abc123",
		ExpiresAt: "2026-03-13T00:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/shares" {
			t.Errorf("expected /api/shares, got %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("expected X-API-Key test-key, got %s", r.Header.Get("X-API-Key"))
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["memory_key"] != "github.issues.mem_001" {
			t.Errorf("expected memory_key github.issues.mem_001, got %s", body["memory_key"])
		}
		if body["ttl"] != "24h" {
			t.Errorf("expected ttl 24h, got %s", body["ttl"])
		}
		if body["layer"] != "summary" {
			t.Errorf("expected layer summary, got %s", body["layer"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	cfg := &Config{Endpoint: srv.URL, APIKey: "test-key"}
	got, err := CreateShare("github.issues.mem_001", "summary", "24h", cfg)
	if err != nil {
		t.Fatalf("CreateShare returned error: %v", err)
	}
	if got.Token != want.Token {
		t.Errorf("Token = %q, want %q", got.Token, want.Token)
	}
	if got.URL != want.URL {
		t.Errorf("URL = %q, want %q", got.URL, want.URL)
	}
	if got.ExpiresAt != want.ExpiresAt {
		t.Errorf("ExpiresAt = %q, want %q", got.ExpiresAt, want.ExpiresAt)
	}
}

func TestCreateShare_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "internal error")
	}))
	defer srv.Close()

	cfg := &Config{Endpoint: srv.URL, APIKey: "test-key"}
	_, err := CreateShare("some.key", "full", "1h", cfg)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if got := err.Error(); got != "server returned 500" {
		t.Errorf("error = %q, want %q", got, "server returned 500")
	}
}

func TestResolveShare(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/s/tok_xyz" {
			t.Errorf("expected /s/tok_xyz, got %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"data": "content here",
			"meta": map[string]string{
				"shared_by":  "alice",
				"memory_key": "github.issues.mem_abc",
				"expires_in": "2h30m",
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	got, err := ResolveShare("tok_xyz", srv.URL)
	if err != nil {
		t.Fatalf("ResolveShare returned error: %v", err)
	}
	if got.Content != "content here" {
		t.Errorf("Content = %q, want %q", got.Content, "content here")
	}
	if got.Author != "alice" {
		t.Errorf("Author = %q, want %q", got.Author, "alice")
	}
	if got.Connector != "github" {
		t.Errorf("Connector = %q, want %q", got.Connector, "github")
	}
	if got.ExpiresIn != "2h30m" {
		t.Errorf("ExpiresIn = %q, want %q", got.ExpiresIn, "2h30m")
	}
}

func TestResolveShare_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "not found")
	}))
	defer srv.Close()

	_, err := ResolveShare("bad_token", srv.URL)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if got := err.Error(); got != "server returned 404" {
		t.Errorf("error = %q, want %q", got, "server returned 404")
	}
}

func TestListShares(t *testing.T) {
	want := []ShareInfo{
		{
			ID:          "share_1",
			MemoryKey:   "github.issues.mem_001",
			Layer:       "summary",
			ExpiresAt:   "2026-03-14T00:00:00Z",
			CreatedAt:   "2026-03-12T00:00:00Z",
			AccessCount: 5,
		},
		{
			ID:          "share_2",
			MemoryKey:   "slack.messages.mem_002",
			Layer:       "full",
			ExpiresAt:   "2026-03-15T00:00:00Z",
			CreatedAt:   "2026-03-12T12:00:00Z",
			AccessCount: 0,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/shares" {
			t.Errorf("expected /api/shares, got %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("expected X-API-Key test-key, got %s", r.Header.Get("X-API-Key"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	cfg := &Config{Endpoint: srv.URL, APIKey: "test-key"}
	got, err := ListShares(cfg)
	if err != nil {
		t.Fatalf("ListShares returned error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d shares, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID {
			t.Errorf("share[%d].ID = %q, want %q", i, got[i].ID, want[i].ID)
		}
		if got[i].MemoryKey != want[i].MemoryKey {
			t.Errorf("share[%d].MemoryKey = %q, want %q", i, got[i].MemoryKey, want[i].MemoryKey)
		}
		if got[i].Layer != want[i].Layer {
			t.Errorf("share[%d].Layer = %q, want %q", i, got[i].Layer, want[i].Layer)
		}
		if got[i].AccessCount != want[i].AccessCount {
			t.Errorf("share[%d].AccessCount = %d, want %d", i, got[i].AccessCount, want[i].AccessCount)
		}
	}
}

func TestRevokeShare(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/shares/share_1" {
			t.Errorf("expected /api/shares/share_1, got %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("expected X-API-Key test-key, got %s", r.Header.Get("X-API-Key"))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfg := &Config{Endpoint: srv.URL, APIKey: "test-key"}
	err := RevokeShare("share_1", cfg)
	if err != nil {
		t.Fatalf("RevokeShare returned error: %v", err)
	}
}

func TestRevokeShare_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "not found")
	}))
	defer srv.Close()

	cfg := &Config{Endpoint: srv.URL, APIKey: "test-key"}
	err := RevokeShare("nonexistent", cfg)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if got := err.Error(); got != "server returned 404" {
		t.Errorf("error = %q, want %q", got, "server returned 404")
	}
}

func TestSplitKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "three parts",
			input: "github.issues.mem_abc",
			want:  []string{"github", "issues", "mem_abc"},
		},
		{
			name:  "single part",
			input: "github",
			want:  []string{"github"},
		},
		{
			name:  "two parts",
			input: "slack.messages",
			want:  []string{"slack", "messages"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{""},
		},
		{
			name:  "four parts",
			input: "a.b.c.d",
			want:  []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitKey(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitKey(%q) returned %d parts, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("splitKey(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
