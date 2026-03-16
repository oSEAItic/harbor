package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	graphFile = "graph.json"
	maxEdges  = 2000
)

// EdgeKind identifies the type of relationship between two memories.
type EdgeKind string

const (
	// EdgeRef is an explicit reference edge: memory A cites memory B.
	EdgeRef EdgeKind = "ref"
)

// GraphEdge represents a directed edge between two memory objects.
type GraphEdge struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Kind      EdgeKind  `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
}

// Graph is the on-disk format for the memory knowledge graph.
type Graph struct {
	Version int         `json:"version"`
	Edges   []GraphEdge `json:"edges"`
}

var graphMu sync.Mutex

// AddRefEdges adds reference edges from fromID to each of the toIDs.
// Duplicates are skipped. The graph is persisted to graph.json.
func AddRefEdges(storeDir string, fromID string, toIDs []string) {
	if len(toIDs) == 0 {
		return
	}
	graphMu.Lock()
	defer graphMu.Unlock()

	g := loadGraph(storeDir)

	// Build set of existing edges for dedup
	existing := make(map[[2]string]bool)
	for _, e := range g.Edges {
		existing[[2]string{e.From, e.To}] = true
	}

	now := time.Now().UTC()
	for _, toID := range toIDs {
		if toID == "" || toID == fromID {
			continue
		}
		key := [2]string{fromID, toID}
		if existing[key] {
			continue
		}
		g.Edges = append(g.Edges, GraphEdge{
			From:      fromID,
			To:        toID,
			Kind:      EdgeRef,
			CreatedAt: now,
		})
		existing[key] = true
	}

	// Auto-prune if too many edges
	if len(g.Edges) > maxEdges {
		g.Edges = g.Edges[len(g.Edges)-maxEdges:]
	}

	saveGraph(storeDir, &g)
}

// RefNeighbors returns memory IDs connected to memoryID via ref edges.
// Both directions are considered: memories that memoryID references, and
// memories that reference memoryID. Returns up to limit unique IDs.
func RefNeighbors(storeDir string, memoryID string, limit int) []string {
	graphMu.Lock()
	defer graphMu.Unlock()

	g := loadGraph(storeDir)

	seen := make(map[string]bool)
	var result []string

	for _, e := range g.Edges {
		if e.Kind != EdgeRef {
			continue
		}
		var neighbor string
		if e.From == memoryID {
			neighbor = e.To
		} else if e.To == memoryID {
			neighbor = e.From
		} else {
			continue
		}
		if seen[neighbor] {
			continue
		}
		seen[neighbor] = true
		result = append(result, neighbor)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// PruneOrphanEdges removes edges where either endpoint is not in validIDs.
func PruneOrphanEdges(storeDir string, validIDs map[string]bool) {
	graphMu.Lock()
	defer graphMu.Unlock()

	g := loadGraph(storeDir)
	if len(g.Edges) == 0 {
		return
	}

	filtered := g.Edges[:0]
	for _, e := range g.Edges {
		if validIDs[e.From] && validIDs[e.To] {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) != len(g.Edges) {
		g.Edges = filtered
		saveGraph(storeDir, &g)
	}
}

func loadGraph(dir string) Graph {
	path := filepath.Join(dir, graphFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Graph{Version: 1}
	}
	var g Graph
	if err := json.Unmarshal(raw, &g); err != nil {
		return Graph{Version: 1}
	}
	return g
}

func saveGraph(dir string, g *Graph) {
	g.Version = 1
	path := filepath.Join(dir, graphFile)
	raw, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o600)
}
