package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddRefEdges(t *testing.T) {
	dir := t.TempDir()

	// Add edges
	AddRefEdges(dir, "mem_aaa", []string{"mem_bbb", "mem_ccc"})

	g := loadGraph(dir)
	if len(g.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(g.Edges))
	}
	if g.Edges[0].From != "mem_aaa" || g.Edges[0].To != "mem_bbb" {
		t.Errorf("unexpected edge[0]: %+v", g.Edges[0])
	}
	if g.Edges[1].From != "mem_aaa" || g.Edges[1].To != "mem_ccc" {
		t.Errorf("unexpected edge[1]: %+v", g.Edges[1])
	}

	// Dedup: adding same edges again should not increase count
	AddRefEdges(dir, "mem_aaa", []string{"mem_bbb", "mem_ccc"})
	g = loadGraph(dir)
	if len(g.Edges) != 2 {
		t.Fatalf("expected 2 edges after dedup, got %d", len(g.Edges))
	}

	// Self-ref and empty should be skipped
	AddRefEdges(dir, "mem_aaa", []string{"mem_aaa", ""})
	g = loadGraph(dir)
	if len(g.Edges) != 2 {
		t.Fatalf("expected 2 edges after self-ref skip, got %d", len(g.Edges))
	}
}

func TestRefNeighbors(t *testing.T) {
	dir := t.TempDir()

	AddRefEdges(dir, "mem_aaa", []string{"mem_bbb", "mem_ccc"})
	AddRefEdges(dir, "mem_ddd", []string{"mem_aaa"})

	// mem_aaa should have 3 neighbors: bbb, ccc (outgoing), ddd (incoming)
	neighbors := RefNeighbors(dir, "mem_aaa", 10)
	if len(neighbors) != 3 {
		t.Fatalf("expected 3 neighbors, got %d: %v", len(neighbors), neighbors)
	}

	// With limit
	neighbors = RefNeighbors(dir, "mem_aaa", 2)
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors with limit, got %d", len(neighbors))
	}

	// mem_bbb should have 1 neighbor: aaa
	neighbors = RefNeighbors(dir, "mem_bbb", 10)
	if len(neighbors) != 1 || neighbors[0] != "mem_aaa" {
		t.Fatalf("expected [mem_aaa], got %v", neighbors)
	}

	// Non-existent memory should have 0 neighbors
	neighbors = RefNeighbors(dir, "mem_zzz", 10)
	if len(neighbors) != 0 {
		t.Fatalf("expected 0 neighbors, got %d", len(neighbors))
	}
}

func TestPruneOrphanEdges(t *testing.T) {
	dir := t.TempDir()

	AddRefEdges(dir, "mem_aaa", []string{"mem_bbb", "mem_ccc"})
	AddRefEdges(dir, "mem_ddd", []string{"mem_aaa"})

	// Prune: only mem_aaa and mem_bbb are valid
	validIDs := map[string]bool{"mem_aaa": true, "mem_bbb": true}
	PruneOrphanEdges(dir, validIDs)

	g := loadGraph(dir)
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge after prune, got %d", len(g.Edges))
	}
	if g.Edges[0].From != "mem_aaa" || g.Edges[0].To != "mem_bbb" {
		t.Errorf("unexpected surviving edge: %+v", g.Edges[0])
	}
}

func TestLoadGraphMissingFile(t *testing.T) {
	dir := t.TempDir()
	g := loadGraph(dir)
	if g.Version != 1 {
		t.Errorf("expected version 1, got %d", g.Version)
	}
	if len(g.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(g.Edges))
	}
}

func TestLoadGraphCorruptFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, graphFile), []byte("not json"), 0o600)
	g := loadGraph(dir)
	if g.Version != 1 {
		t.Errorf("expected version 1 on corrupt file, got %d", g.Version)
	}
}
