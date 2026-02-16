package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const indexVersion = 1

// loadIndex reads index.json from the memory directory.
// Returns an empty index if the file is missing or corrupted.
func loadIndex(dir string) *Index {
	path := filepath.Join(dir, "index.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return &Index{Version: indexVersion, Entries: []IndexEntry{}}
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		// Self-heal: corrupted index, rebuild from objects
		return rebuildIndex(dir)
	}

	if idx.Version != indexVersion {
		return rebuildIndex(dir)
	}

	return &idx
}

// saveIndex atomically writes the index to disk via temp+rename.
func saveIndex(dir string, idx *Index) error {
	idx.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "index.json")
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

// rebuildIndex scans the objects/ directory and reconstructs index.json.
func rebuildIndex(dir string) *Index {
	idx := &Index{Version: indexVersion, Entries: []IndexEntry{}}
	objDir := filepath.Join(dir, "objects")

	entries, err := os.ReadDir(objDir)
	if err != nil {
		return idx
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(objDir, e.Name()))
		if err != nil {
			continue
		}

		var obj Object
		if err := json.Unmarshal(data, &obj); err != nil {
			continue
		}

		idx.Entries = append(idx.Entries, IndexEntry{
			ID:           obj.ID,
			Connector:    obj.Connector,
			Resource:     obj.Resource,
			Schema:       obj.Schema,
			Params:       obj.Params,
			CreatedAt:    obj.CreatedAt,
			TTLSeconds:   obj.TTLSeconds,
			BytesRaw:     obj.Meta.BytesRaw,
			BytesCompact: obj.Meta.BytesCompact,
			Summary:      obj.Layers.Summary,
		})
	}

	// Best-effort save the rebuilt index
	_ = saveIndex(dir, idx)

	return idx
}
