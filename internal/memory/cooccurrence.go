package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	cooccurrenceFile = "cooccurrence.json"
	sessionWindow    = 5 * time.Minute  // calls within this window are considered co-occurring
	minEdgeCount     = 2                // minimum co-occurrences before cross-connector recall activates
	maxRecentCalls   = 50               // prune recent calls list to prevent unbounded growth
	edgeDecayDays    = 30               // edges older than this lose weight
)

// CooccurrenceEdge tracks how often two connectors are called together.
type CooccurrenceEdge struct {
	From     string    `json:"from"`
	To       string    `json:"to"`
	Count    int       `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

// RecentCall records a single connector invocation for session-window tracking.
type RecentCall struct {
	Connector string    `json:"connector"`
	Timestamp time.Time `json:"timestamp"`
}

// CooccurrenceData is the on-disk format for co-occurrence tracking.
type CooccurrenceData struct {
	Edges       []CooccurrenceEdge `json:"edges"`
	RecentCalls []RecentCall       `json:"recent_calls"`
}

var coMu sync.Mutex

// RecordCall logs a connector invocation and updates co-occurrence edges.
// Any other connector called within sessionWindow gets its edge count incremented.
func RecordCall(storeDir, connector string) {
	coMu.Lock()
	defer coMu.Unlock()

	data := loadCooccurrence(storeDir)
	now := time.Now().UTC()

	// Prune calls older than sessionWindow
	cutoff := now.Add(-sessionWindow)
	fresh := data.RecentCalls[:0]
	for _, c := range data.RecentCalls {
		if c.Timestamp.After(cutoff) {
			fresh = append(fresh, c)
		}
	}
	data.RecentCalls = fresh

	// Increment edges for all recent calls from different connectors
	seen := make(map[string]bool)
	for _, c := range data.RecentCalls {
		if c.Connector == connector || seen[c.Connector] {
			continue
		}
		seen[c.Connector] = true
		upsertEdge(&data, connector, c.Connector, now)
	}

	// Add this call
	data.RecentCalls = append(data.RecentCalls, RecentCall{
		Connector: connector,
		Timestamp: now,
	})

	// Cap recent calls list
	if len(data.RecentCalls) > maxRecentCalls {
		data.RecentCalls = data.RecentCalls[len(data.RecentCalls)-maxRecentCalls:]
	}

	saveCooccurrence(storeDir, &data)
}

// RelatedConnectors returns connectors that frequently co-occur with the given one.
// Results are sorted by edge count (descending). Only edges with count >= minEdgeCount
// are returned. limit controls the max number of results.
func RelatedConnectors(storeDir, connector string, limit int) []string {
	coMu.Lock()
	defer coMu.Unlock()

	data := loadCooccurrence(storeDir)

	type scored struct {
		name  string
		count int
	}
	var candidates []scored

	for _, e := range data.Edges {
		if e.Count < minEdgeCount {
			continue
		}
		var other string
		if e.From == connector {
			other = e.To
		} else if e.To == connector {
			other = e.From
		} else {
			continue
		}
		candidates = append(candidates, scored{name: other, count: e.Count})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := make([]string, len(candidates))
	for i, c := range candidates {
		result[i] = c.name
	}
	return result
}

func upsertEdge(data *CooccurrenceData, a, b string, now time.Time) {
	// Normalize edge direction: alphabetical order
	from, to := a, b
	if from > to {
		from, to = to, from
	}

	for i := range data.Edges {
		if data.Edges[i].From == from && data.Edges[i].To == to {
			data.Edges[i].Count++
			data.Edges[i].LastSeen = now
			return
		}
	}

	data.Edges = append(data.Edges, CooccurrenceEdge{
		From:     from,
		To:       to,
		Count:    1,
		LastSeen: now,
	})
}

func loadCooccurrence(dir string) CooccurrenceData {
	path := filepath.Join(dir, cooccurrenceFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return CooccurrenceData{}
	}
	var data CooccurrenceData
	if err := json.Unmarshal(raw, &data); err != nil {
		return CooccurrenceData{}
	}
	return data
}

func saveCooccurrence(dir string, data *CooccurrenceData) {
	path := filepath.Join(dir, cooccurrenceFile)
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o600)
}
