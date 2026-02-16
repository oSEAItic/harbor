package schema

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateSample(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxChars int
		check    func(t *testing.T, result string)
	}{
		{
			name:     "short string unchanged",
			input:    `{"key": "value"}`,
			maxChars: 4000,
			check: func(t *testing.T, result string) {
				if result != `{"key": "value"}` {
					t.Errorf("got %q", result)
				}
			},
		},
		{
			name:     "long string truncated",
			input:    strings.Repeat("x", 5000),
			maxChars: 100,
			check: func(t *testing.T, result string) {
				if len(result) != 103 { // 100 + "..."
					t.Errorf("len = %d, want 103", len(result))
				}
				if !strings.HasSuffix(result, "...") {
					t.Error("missing ... suffix")
				}
			},
		},
		{
			name:     "JSON array truncated to 3 items",
			input:    `[{"a":1},{"a":2},{"a":3},{"a":4},{"a":5}]`,
			maxChars: 4000,
			check: func(t *testing.T, result string) {
				var arr []json.RawMessage
				if err := json.Unmarshal([]byte(result), &arr); err != nil {
					t.Fatalf("not valid JSON: %v", err)
				}
				if len(arr) != 3 {
					t.Errorf("array len = %d, want 3", len(arr))
				}
			},
		},
		{
			name:     "small JSON array unchanged",
			input:    `[{"a":1},{"a":2}]`,
			maxChars: 4000,
			check: func(t *testing.T, result string) {
				var arr []json.RawMessage
				if err := json.Unmarshal([]byte(result), &arr); err != nil {
					t.Fatalf("not valid JSON: %v", err)
				}
				if len(arr) != 2 {
					t.Errorf("array len = %d, want 2", len(arr))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateSample(tt.input, tt.maxChars)
			tt.check(t, result)
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	prompt := buildPrompt("fs.list_directory", `[{"name":"file.txt"}]`)
	if !strings.Contains(prompt, "fs.list_directory") {
		t.Error("prompt missing tool name")
	}
	if !strings.Contains(prompt, "file.txt") {
		t.Error("prompt missing sample data")
	}
}

func TestLearnWithMockLLM(t *testing.T) {
	// Mock LLM server
	mockResp := `{
		"choices": [{
			"message": {
				"content": "{\"summary_fields\": [\"name\", \"size\", \"type\"], \"summary_template\": \"{name} ({type}, {size} bytes)\"}"
			}
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("unexpected auth: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResp))
	}))
	defer srv.Close()

	cfg := &LLMConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   "test-model",
	}

	sample := `[{"name":"file.txt","size":1024,"type":"file","permissions":"rw-r--r--"}]`
	result, err := Learn(context.Background(), cfg, "fs.list_directory", sample)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}

	if len(result.SummaryFields) != 3 {
		t.Errorf("SummaryFields len = %d, want 3", len(result.SummaryFields))
	}
	if result.SummaryFields[0] != "name" {
		t.Errorf("SummaryFields[0] = %q, want 'name'", result.SummaryFields[0])
	}
	if result.SummaryTemplate != "{name} ({type}, {size} bytes)" {
		t.Errorf("SummaryTemplate = %q", result.SummaryTemplate)
	}
}

func TestLearnAndSaveWithMockLLM(t *testing.T) {
	mockResp := `{
		"choices": [{
			"message": {
				"content": "{\"summary_fields\": [\"title\", \"status\"], \"summary_template\": \"{title}: {status}\"}"
			}
		}]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResp))
	}))
	defer srv.Close()

	cfg := &LLMConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   "test-model",
	}

	store, err := NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	sample := `[{"title":"Bug fix","status":"open","id":42}]`
	if err := LearnAndSave(context.Background(), cfg, store, "github.issues", sample); err != nil {
		t.Fatalf("LearnAndSave: %v", err)
	}

	ls := store.Get("github.issues")
	if ls == nil {
		t.Fatal("schema not saved")
	}
	if ls.ToolName != "github.issues" {
		t.Errorf("ToolName = %q", ls.ToolName)
	}
	if ls.LLMModel != "test-model" {
		t.Errorf("LLMModel = %q", ls.LLMModel)
	}
}
