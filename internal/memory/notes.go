package memory

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
)

// Note is a cloud-synced memory summary stored locally.
// It mirrors the summary pushed to and pulled from Harbor Cloud,
// enabling cross-device memory recall without storing full data.
type Note struct {
	Key       string    `json:"key"`        // e.g. "kuse-hive.traces"
	Content   string    `json:"content"`    // summary text
	UpdatedAt time.Time `json:"updated_at"`
}

// NoteStore persists cloud-pulled memory notes to a local JSON file.
type NoteStore struct {
	path string
}

// NewNoteStore returns a NoteStore backed by ~/.harbor/memory/cloud_notes.json.
func NewNoteStore() *NoteStore {
	return &NoteStore{path: harborhome.Path("memory/cloud_notes.json")}
}

// Load reads all notes from disk. Returns an empty slice if the file does not exist.
func (n *NoteStore) Load() ([]Note, error) {
	data, err := os.ReadFile(n.path)
	if os.IsNotExist(err) {
		return []Note{}, nil
	}
	if err != nil {
		return nil, err
	}
	var notes []Note
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// Save writes the full note list to disk, replacing any previous content.
func (n *NoteStore) Save(notes []Note) error {
	data, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(n.path, data, 0o644)
}

// ForConnector returns all notes whose key starts with "<connector>.".
func (n *NoteStore) ForConnector(connector string) []Note {
	all, err := n.Load()
	if err != nil {
		return nil
	}
	prefix := connector + "."
	var out []Note
	for _, note := range all {
		if strings.HasPrefix(note.Key, prefix) {
			out = append(out, note)
		}
	}
	return out
}
