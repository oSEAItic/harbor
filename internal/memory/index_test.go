package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadIndexMissing(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "objects"), 0o755)

	idx := loadIndex(dir)
	if idx == nil {
		t.Fatal("expected non-nil index")
	}
	if idx.Version != indexVersion {
		t.Errorf("Version = %d, want %d", idx.Version, indexVersion)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(idx.Entries))
	}
}

func TestSaveAndLoadIndex(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "objects"), 0o755)

	idx := &Index{
		Version: indexVersion,
		Entries: []IndexEntry{
			{
				ID:         "mem_abc12345",
				Connector:  "coingecko",
				Resource:   "prices",
				Schema:     "crypto.prices.v1",
				Params:     map[string]string{"ids": "bitcoin"},
				CreatedAt:  time.Now(),
				TTLSeconds: 300,
				BytesRaw:   1024,
				BytesCompact: 256,
			},
		},
	}

	err := saveIndex(dir, idx)
	if err != nil {
		t.Fatalf("saveIndex: %v", err)
	}

	loaded := loadIndex(dir)
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].ID != "mem_abc12345" {
		t.Errorf("ID = %q, want %q", loaded.Entries[0].ID, "mem_abc12345")
	}
	if loaded.Entries[0].Connector != "coingecko" {
		t.Errorf("Connector = %q, want %q", loaded.Entries[0].Connector, "coingecko")
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after save")
	}
}

func TestLoadIndexCorrupted(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "objects"), 0o755)

	// Write garbage to index.json
	indexPath := filepath.Join(dir, "index.json")
	os.WriteFile(indexPath, []byte("not json{{{"), 0o644)

	idx := loadIndex(dir)
	if idx == nil {
		t.Fatal("expected non-nil index after corruption")
	}
	if idx.Version != indexVersion {
		t.Errorf("Version = %d, want %d", idx.Version, indexVersion)
	}
}

func TestLoadIndexWrongVersion(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "objects"), 0o755)

	// Write index with wrong version
	idx := &Index{Version: 999, Entries: []IndexEntry{}}
	data, _ := json.Marshal(idx)
	os.WriteFile(filepath.Join(dir, "index.json"), data, 0o644)

	loaded := loadIndex(dir)
	if loaded.Version != indexVersion {
		t.Errorf("expected rebuilt index with version %d, got %d", indexVersion, loaded.Version)
	}
}

func TestRebuildIndex(t *testing.T) {
	dir := t.TempDir()
	objDir := filepath.Join(dir, "objects")
	os.MkdirAll(objDir, 0o755)

	// Write an object file directly
	obj := &Object{
		ID:         "mem_rebuild1",
		Connector:  "coingecko",
		Resource:   "trending",
		Schema:     "crypto.trending.v1",
		CreatedAt:  time.Now(),
		TTLSeconds: 900,
		Layers: Layers{
			Raw:        json.RawMessage(`{}`),
			Normalized: json.RawMessage(`[]`),
		},
		Meta: ObjectMeta{BytesRaw: 100, BytesCompact: 50},
	}
	data, _ := json.MarshalIndent(obj, "", "  ")
	os.WriteFile(filepath.Join(objDir, "mem_rebuild1.json"), data, 0o644)

	idx := rebuildIndex(dir)
	if len(idx.Entries) != 1 {
		t.Fatalf("expected 1 entry after rebuild, got %d", len(idx.Entries))
	}
	if idx.Entries[0].ID != "mem_rebuild1" {
		t.Errorf("ID = %q, want %q", idx.Entries[0].ID, "mem_rebuild1")
	}
	if idx.Entries[0].Connector != "coingecko" {
		t.Errorf("Connector = %q, want %q", idx.Entries[0].Connector, "coingecko")
	}
}

func TestRebuildIndexEmptyDir(t *testing.T) {
	dir := t.TempDir()
	objDir := filepath.Join(dir, "objects")
	os.MkdirAll(objDir, 0o755)

	idx := rebuildIndex(dir)
	if len(idx.Entries) != 0 {
		t.Errorf("expected 0 entries for empty dir, got %d", len(idx.Entries))
	}
}

func TestSaveIndexAtomicity(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "objects"), 0o755)

	idx := &Index{
		Version: indexVersion,
		Entries: []IndexEntry{
			{ID: "mem_atomic1", Connector: "test", Resource: "data"},
		},
	}

	err := saveIndex(dir, idx)
	if err != nil {
		t.Fatalf("saveIndex: %v", err)
	}

	// Verify no .tmp file remains
	tmpPath := filepath.Join(dir, "index.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful save")
	}

	// Verify index.json exists and is valid
	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("reading index.json: %v", err)
	}

	var loaded Index
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("index.json is not valid JSON: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(loaded.Entries))
	}
}
