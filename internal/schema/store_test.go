package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndGet(t *testing.T) {
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	ls := &LearnedSchema{
		ToolName:        "fs.list_directory",
		SummaryFields:   []string{"name", "size", "type"},
		SummaryTemplate: "{name} ({type}, {size} bytes)",
		LearnedAt:       time.Now(),
		LLMModel:        "gpt-4o-mini",
		Version:         1,
	}

	if err := store.Save(ls); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := store.Get("fs.list_directory")
	if got == nil {
		t.Fatal("Get returned nil for saved schema")
	}
	if got.ToolName != ls.ToolName {
		t.Errorf("ToolName = %q, want %q", got.ToolName, ls.ToolName)
	}
	if len(got.SummaryFields) != 3 {
		t.Errorf("SummaryFields len = %d, want 3", len(got.SummaryFields))
	}
	if got.SummaryTemplate != ls.SummaryTemplate {
		t.Errorf("SummaryTemplate = %q, want %q", got.SummaryTemplate, ls.SummaryTemplate)
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
}

func TestGetMissing(t *testing.T) {
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	got := store.Get("nonexistent_tool")
	if got != nil {
		t.Errorf("Get returned %+v for missing schema, want nil", got)
	}
}

func TestHas(t *testing.T) {
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	if store.Has("some_tool") {
		t.Error("Has returned true for missing tool")
	}

	ls := &LearnedSchema{
		ToolName:      "some_tool",
		SummaryFields: []string{"a"},
		LearnedAt:     time.Now(),
		Version:       1,
	}
	if err := store.Save(ls); err != nil {
		t.Fatal(err)
	}

	if !store.Has("some_tool") {
		t.Error("Has returned false after Save")
	}
}

func TestOverwrite(t *testing.T) {
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	v1 := &LearnedSchema{
		ToolName:      "my_tool",
		SummaryFields: []string{"a"},
		LearnedAt:     time.Now(),
		Version:       1,
	}
	if err := store.Save(v1); err != nil {
		t.Fatal(err)
	}

	v2 := &LearnedSchema{
		ToolName:      "my_tool",
		SummaryFields: []string{"a", "b", "c"},
		LearnedAt:     time.Now(),
		Version:       1,
	}
	if err := store.Save(v2); err != nil {
		t.Fatal(err)
	}

	got := store.Get("my_tool")
	if got == nil {
		t.Fatal("Get returned nil after overwrite")
	}
	if got.Version != 2 {
		t.Errorf("Version = %d, want 2", got.Version)
	}
	if len(got.SummaryFields) != 3 {
		t.Errorf("SummaryFields len = %d, want 3", len(got.SummaryFields))
	}
}

func TestHistoryAndRollback(t *testing.T) {
	store, err := NewStoreAt(filepath.Join(t.TempDir(), "schemas"))
	if err != nil {
		t.Fatal(err)
	}

	for _, fields := range [][]string{
		{"name", "size"},
		{"name", "size", "type"},
		{"name"},
	} {
		if err := store.Save(&LearnedSchema{
			ToolName:      "fs.list",
			SummaryFields: fields,
			LearnedAt:     time.Now(),
			Version:       1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	h, err := store.History("fs.list")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(h) != 3 {
		t.Fatalf("history len = %d, want 3", len(h))
	}
	if h[0].Version != 1 || h[2].Version != 3 {
		t.Fatalf("unexpected version range: first=%d last=%d", h[0].Version, h[2].Version)
	}

	rolled, err := store.Rollback("fs.list", 1)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.Version != 4 {
		t.Fatalf("rollback version = %d, want 4", rolled.Version)
	}

	current := store.Get("fs.list")
	if current == nil {
		t.Fatal("Get returned nil after rollback")
	}
	if current.Version != 4 {
		t.Fatalf("current version = %d, want 4", current.Version)
	}
	if len(current.SummaryFields) != 2 {
		t.Fatalf("rollback fields len = %d, want 2", len(current.SummaryFields))
	}
}

func TestCorruptedFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "schemas")
	store, err := NewStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write garbage to the schema file
	path := filepath.Join(dir, "broken_tool.json")
	if err := os.WriteFile(path, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := store.Get("broken_tool")
	if got != nil {
		t.Errorf("Get returned %+v for corrupted file, want nil", got)
	}
}

func TestHistoryFallsBackToCurrentFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "schemas")
	store, err := NewStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}

	current := &LearnedSchema{
		ToolName:      "legacy_tool",
		SummaryFields: []string{"id"},
		LearnedAt:     time.Now(),
		Version:       3,
	}
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "legacy_tool.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := store.History("legacy_tool")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(h) != 1 {
		t.Fatalf("history len = %d, want 1", len(h))
	}
	if h[0].Version != 3 {
		t.Fatalf("history version = %d, want 3", h[0].Version)
	}
}

func TestStore_CloudPush_CalledOnSave(t *testing.T) {
	store, err := NewStoreAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	called := make(chan *LearnedSchema, 1)
	store.CloudPush = func(toolName string, ls *LearnedSchema) {
		called <- ls
	}

	ls := &LearnedSchema{
		ToolName:        "github__list_issues",
		SummaryFields:   []string{"number", "title", "state"},
		SummaryTemplate: "#{number} {title} [{state}]",
		LearnedAt:       time.Now(),
	}
	if err := store.Save(ls); err != nil {
		t.Fatalf("Save: %v", err)
	}

	select {
	case pushed := <-called:
		if pushed.ToolName != ls.ToolName {
			t.Errorf("CloudPush got ToolName=%q, want %q", pushed.ToolName, ls.ToolName)
		}
		if len(pushed.SummaryFields) != 3 {
			t.Errorf("CloudPush got %d fields, want 3", len(pushed.SummaryFields))
		}
	case <-time.After(time.Second):
		t.Fatal("CloudPush was not called within 1s")
	}
}

func TestStore_CloudPush_NilSafe(t *testing.T) {
	store, err := NewStoreAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// CloudPush is nil by default — Save must not panic.
	ls := &LearnedSchema{
		ToolName:        "fs__list_directory",
		SummaryFields:   []string{"name"},
		SummaryTemplate: "{name}",
		LearnedAt:       time.Now(),
	}
	if err := store.Save(ls); err != nil {
		t.Fatalf("Save with nil CloudPush: %v", err)
	}
}

func TestStore_CloudPush_NotCalledOnReadOnly(t *testing.T) {
	store, err := NewStoreAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	var pushCount int
	store.CloudPush = func(_ string, _ *LearnedSchema) { pushCount++ }

	// Get on missing key — must not trigger CloudPush.
	_ = store.Get("nonexistent")
	if pushCount != 0 {
		t.Errorf("CloudPush called %d times on Get, want 0", pushCount)
	}
}
