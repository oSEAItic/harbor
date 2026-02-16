package schema

import (
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
		Version:       2,
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
