package memory

import (
	"encoding/json"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStoreAt(t.TempDir())
	if err != nil {
		t.Fatalf("NewStoreAt: %v", err)
	}
	return s
}

func testObject(connector, resource string, params map[string]string) *Object {
	return &Object{
		Connector:  connector,
		Resource:   resource,
		Schema:     connector + "." + resource + ".v1",
		Params:     params,
		TTLSeconds: 600,
		Layers: Layers{
			Raw:        json.RawMessage(`{"raw":true}`),
			Normalized: json.RawMessage(`[{"id":"1"}]`),
			Compact:    json.RawMessage(`[{"id":"1"}]`),
			Summary:    "test summary",
		},
		Meta: ObjectMeta{
			Source:           connector,
			ConnectorVersion: "0.1.0",
			RequestID:        "req-123",
		},
	}
}

func TestSaveAndGet(t *testing.T) {
	s := testStore(t)
	obj := testObject("coingecko", "prices", map[string]string{"ids": "bitcoin"})

	id, err := s.Save(obj)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if id == "" {
		t.Fatal("Save returned empty ID")
	}
	if id != obj.ID {
		t.Fatalf("Save returned %q but obj.ID is %q", id, obj.ID)
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Connector != "coingecko" {
		t.Errorf("Connector = %q, want %q", got.Connector, "coingecko")
	}
	if got.Resource != "prices" {
		t.Errorf("Resource = %q, want %q", got.Resource, "prices")
	}
	if got.Layers.Summary != "test summary" {
		t.Errorf("Summary = %q, want %q", got.Layers.Summary, "test summary")
	}
	if got.Meta.ContentHash == "" {
		t.Error("ContentHash is empty")
	}
	if got.Meta.BytesRaw == 0 {
		t.Error("BytesRaw is 0")
	}
}

func TestSaveOverwritesSameKey(t *testing.T) {
	s := testStore(t)
	params := map[string]string{"ids": "bitcoin"}

	obj1 := testObject("coingecko", "prices", params)
	obj1.Layers.Summary = "first"
	id1, err := s.Save(obj1)
	if err != nil {
		t.Fatalf("Save first: %v", err)
	}

	obj2 := testObject("coingecko", "prices", params)
	obj2.Layers.Summary = "second"
	id2, err := s.Save(obj2)
	if err != nil {
		t.Fatalf("Save second: %v", err)
	}

	// IDs differ because timestamp is part of the hash
	if id1 == id2 {
		t.Error("expected different IDs for different saves")
	}

	// Old object should be gone
	_, err = s.Get(id1)
	if err == nil {
		t.Error("expected error getting old object, got nil")
	}

	// New object should exist
	got, err := s.Get(id2)
	if err != nil {
		t.Fatalf("Get new: %v", err)
	}
	if got.Layers.Summary != "second" {
		t.Errorf("Summary = %q, want %q", got.Layers.Summary, "second")
	}
}

func TestLatest(t *testing.T) {
	s := testStore(t)

	// No entries yet
	entry := s.Latest("coingecko", "prices", map[string]string{"ids": "bitcoin"})
	if entry != nil {
		t.Error("expected nil for empty store")
	}

	// Save one
	obj := testObject("coingecko", "prices", map[string]string{"ids": "bitcoin"})
	id, _ := s.Save(obj)

	entry = s.Latest("coingecko", "prices", map[string]string{"ids": "bitcoin"})
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.ID != id {
		t.Errorf("ID = %q, want %q", entry.ID, id)
	}

	// Different params should not match
	entry = s.Latest("coingecko", "prices", map[string]string{"ids": "ethereum"})
	if entry != nil {
		t.Error("expected nil for different params")
	}
}

func TestIsFresh(t *testing.T) {
	s := testStore(t)

	// Nil entry is not fresh
	if s.IsFresh(nil) {
		t.Error("nil entry should not be fresh")
	}

	// Fresh entry
	entry := &IndexEntry{
		CreatedAt:  time.Now(),
		TTLSeconds: 600,
	}
	if !s.IsFresh(entry) {
		t.Error("recent entry with 600s TTL should be fresh")
	}

	// Expired entry
	entry = &IndexEntry{
		CreatedAt:  time.Now().Add(-10 * time.Minute),
		TTLSeconds: 60,
	}
	if s.IsFresh(entry) {
		t.Error("old entry with 60s TTL should not be fresh")
	}
}

func TestQuery(t *testing.T) {
	s := testStore(t)

	// Save several objects
	s.Save(testObject("coingecko", "prices", map[string]string{"ids": "bitcoin"}))
	s.Save(testObject("coingecko", "trending", nil))
	s.Save(testObject("yahoo", "quote", map[string]string{"symbols": "AAPL"}))

	// Query by connector
	results := s.Query(QueryOptions{Connector: "coingecko"})
	if len(results) != 2 {
		t.Errorf("expected 2 coingecko results, got %d", len(results))
	}

	// Query by resource
	results = s.Query(QueryOptions{Resource: "prices"})
	if len(results) != 1 {
		t.Errorf("expected 1 prices result, got %d", len(results))
	}

	// Query by params
	results = s.Query(QueryOptions{Params: map[string]string{"ids": "bitcoin"}})
	if len(results) != 1 {
		t.Errorf("expected 1 result with ids=bitcoin, got %d", len(results))
	}

	// Query with limit
	results = s.Query(QueryOptions{Limit: 1})
	if len(results) != 1 {
		t.Errorf("expected 1 result with limit=1, got %d", len(results))
	}

	// Query all
	results = s.Query(QueryOptions{})
	if len(results) != 3 {
		t.Errorf("expected 3 total results, got %d", len(results))
	}
}

func TestPrune(t *testing.T) {
	s := testStore(t)

	obj := testObject("coingecko", "prices", map[string]string{"ids": "bitcoin"})
	id, _ := s.Save(obj)

	// Prune with large maxAge should remove nothing
	count, err := s.Prune(24 * time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 pruned, got %d", count)
	}

	// Object should still exist
	_, err = s.Get(id)
	if err != nil {
		t.Fatalf("Get after prune: %v", err)
	}

	// Prune with zero maxAge should remove everything
	count, err = s.Prune(0)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 pruned, got %d", count)
	}

	// Object should be gone
	_, err = s.Get(id)
	if err == nil {
		t.Error("expected error after pruning, got nil")
	}
}

func TestDefaultTTL(t *testing.T) {
	tests := []struct {
		connector string
		resource  string
		want      time.Duration
	}{
		{"coingecko", "prices", 5 * time.Minute},
		{"yahoo", "quote", 5 * time.Minute},
		{"coingecko", "trending", 15 * time.Minute},
		{"coingecko", "coin", 30 * time.Minute},
		{"yahoo", "search", 30 * time.Minute},
		{"custom", "data", 10 * time.Minute}, // fallback
	}

	for _, tt := range tests {
		got := DefaultTTL(tt.connector, tt.resource)
		if got != tt.want {
			t.Errorf("DefaultTTL(%q, %q) = %v, want %v", tt.connector, tt.resource, got, tt.want)
		}
	}
}

func TestGetNonExistent(t *testing.T) {
	s := testStore(t)

	_, err := s.Get("mem_doesnotexist")
	if err == nil {
		t.Error("expected error for non-existent object, got nil")
	}
}

func TestSearchByKeyword(t *testing.T) {
	s := testStore(t)

	// Save several objects with different connectors/resources/summaries
	obj1 := testObject("coingecko", "prices", map[string]string{"ids": "bitcoin"})
	obj1.Layers.Summary = "Bitcoin price is $50000"
	s.Save(obj1)

	obj2 := testObject("coingecko", "trending", nil)
	obj2.Layers.Summary = "Top trending coins today"
	s.Save(obj2)

	obj3 := testObject("yahoo", "quote", map[string]string{"symbols": "AAPL"})
	obj3.Layers.Summary = "Apple stock at $175"
	s.Save(obj3)

	// Search for "bitcoin" — should match obj1 via params and summary
	results := s.Search("bitcoin", QueryOptions{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'bitcoin', got %d", len(results))
	}
	if results[0].Connector != "coingecko" || results[0].Resource != "prices" {
		t.Errorf("unexpected result: %s.%s", results[0].Connector, results[0].Resource)
	}

	// Search for "coingecko" — should match both coingecko objects
	results = s.Search("coingecko", QueryOptions{})
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'coingecko', got %d", len(results))
	}

	// Search for "apple" — should match via summary
	results = s.Search("apple", QueryOptions{})
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'apple', got %d", len(results))
	}

	// Search for "nonexistent" — should return 0
	results = s.Search("nonexistent", QueryOptions{})
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'nonexistent', got %d", len(results))
	}

	// Empty query — should return all entries
	results = s.Search("", QueryOptions{})
	if len(results) != 3 {
		t.Errorf("expected 3 results for empty query, got %d", len(results))
	}
}

func TestSearchWithConnectorFilter(t *testing.T) {
	s := testStore(t)

	s.Save(testObject("coingecko", "prices", map[string]string{"ids": "bitcoin"}))
	s.Save(testObject("yahoo", "quote", map[string]string{"symbols": "AAPL"}))

	// Search all with connector filter
	results := s.Search("", QueryOptions{Connector: "coingecko"})
	if len(results) != 1 {
		t.Errorf("expected 1 result with connector filter, got %d", len(results))
	}
	if results[0].Connector != "coingecko" {
		t.Errorf("expected coingecko, got %s", results[0].Connector)
	}
}

func TestSearchWithLimit(t *testing.T) {
	s := testStore(t)

	s.Save(testObject("a", "r1", nil))
	s.Save(testObject("b", "r2", nil))
	s.Save(testObject("c", "r3", nil))

	results := s.Search("", QueryOptions{Limit: 2})
	if len(results) != 2 {
		t.Errorf("expected 2 results with limit=2, got %d", len(results))
	}
}

func TestSearchScoring(t *testing.T) {
	s := testStore(t)

	// obj1 has "bitcoin" in both connector name indirectly and summary
	obj1 := testObject("crypto", "prices", map[string]string{"ids": "bitcoin"})
	obj1.Layers.Summary = "Bitcoin price data"
	s.Save(obj1)

	// obj2 has "bitcoin" only in summary
	obj2 := testObject("news", "articles", nil)
	obj2.Layers.Summary = "Some random article"
	s.Save(obj2)

	// Search "bitcoin prices" — obj1 should score higher (matches both tokens)
	results := s.Search("bitcoin prices", QueryOptions{})
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].Resource != "prices" {
		t.Errorf("expected 'prices' to rank first, got %q", results[0].Resource)
	}
}
