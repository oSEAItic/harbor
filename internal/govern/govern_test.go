package govern

import (
	"reflect"
	"sort"
	"testing"
)

// ── FilterItems tests (Layer 3) ──────────────────────────────────

func TestFilterItems_EmptyMaxVisibility(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "btc", "secret": "x"},
	}
	fv := map[string]string{"secret": "restricted"}

	got, result := FilterItems(items, fv, "")
	if result.RemovedCount != 0 {
		t.Fatalf("expected 0 removed, got %d", result.RemovedCount)
	}
	if !reflect.DeepEqual(got, items) {
		t.Fatalf("items should be unchanged")
	}
}

func TestFilterItems_EmptyFieldVisibility(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "btc", "price": 100},
	}
	got, result := FilterItems(items, nil, "public")
	if result.RemovedCount != 0 {
		t.Fatalf("expected 0 removed, got %d", result.RemovedCount)
	}
	if !reflect.DeepEqual(got, items) {
		t.Fatalf("items should be unchanged")
	}
}

func TestFilterItems_UnknownLevel(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "btc", "secret": "x"},
	}
	fv := map[string]string{"secret": "restricted"}

	got, result := FilterItems(items, fv, "bogus")
	if result.RemovedCount != 0 {
		t.Fatalf("unknown level should not filter, got %d removed", result.RemovedCount)
	}
	if !reflect.DeepEqual(got, items) {
		t.Fatalf("items should be unchanged")
	}
}

func TestFilterItems_InternalRemovesHigher(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "btc", "price": 100, "market_data": "big", "api_key": "secret"},
	}
	fv := map[string]string{
		"id":          "public",
		"price":       "internal",
		"market_data": "confidential",
		"api_key":     "restricted",
	}

	got, result := FilterItems(items, fv, "internal")

	// Should keep id (public) and price (internal), remove market_data and api_key.
	if result.RemovedCount != 2 {
		t.Fatalf("expected 2 removed, got %d", result.RemovedCount)
	}

	item := got[0]
	if _, ok := item["id"]; !ok {
		t.Error("id should be kept (public)")
	}
	if _, ok := item["price"]; !ok {
		t.Error("price should be kept (internal)")
	}
	if _, ok := item["market_data"]; ok {
		t.Error("market_data should be removed (confidential > internal)")
	}
	if _, ok := item["api_key"]; ok {
		t.Error("api_key should be removed (restricted > internal)")
	}
}

func TestFilterItems_UndeclaredFieldsDefaultPublic(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "btc", "extra": "kept"},
	}
	fv := map[string]string{
		"id": "public",
		// "extra" not declared → defaults to public
	}

	got, _ := FilterItems(items, fv, "public")
	item := got[0]
	if _, ok := item["extra"]; !ok {
		t.Error("undeclared field 'extra' should default to public and be kept")
	}
}

func TestFilterItems_PublicRemovesEverythingAbove(t *testing.T) {
	items := []map[string]interface{}{
		{"name": "Bitcoin", "internal_id": "123", "pii": "secret"},
	}
	fv := map[string]string{
		"name":        "public",
		"internal_id": "internal",
		"pii":         "confidential",
	}

	got, result := FilterItems(items, fv, "public")
	if result.RemovedCount != 2 {
		t.Fatalf("expected 2 removed, got %d", result.RemovedCount)
	}
	if _, ok := got[0]["name"]; !ok {
		t.Error("name should be kept")
	}
	if _, ok := got[0]["internal_id"]; ok {
		t.Error("internal_id should be removed")
	}
}

func TestFilterItems_MultipleItems(t *testing.T) {
	items := []map[string]interface{}{
		{"id": "btc", "secret": "a"},
		{"id": "eth", "secret": "b"},
	}
	fv := map[string]string{"secret": "restricted"}

	got, result := FilterItems(items, fv, "public")
	if result.RemovedCount != 1 {
		t.Fatalf("expected 1 field type removed, got %d", result.RemovedCount)
	}
	for i, item := range got {
		if _, ok := item["secret"]; ok {
			t.Errorf("item %d: secret should be removed", i)
		}
		if _, ok := item["id"]; !ok {
			t.Errorf("item %d: id should be kept", i)
		}
	}
}

// ── Allowed tests ────────────────────────────────────────────────

func TestAllowed(t *testing.T) {
	tests := []struct {
		field, max Visibility
		want       bool
	}{
		{Public, Public, true},
		{Public, Restricted, true},
		{Internal, Internal, true},
		{Confidential, Internal, false},
		{Restricted, Internal, false},
		{Restricted, Restricted, true},
	}
	for _, tt := range tests {
		got := Allowed(tt.field, tt.max)
		if got != tt.want {
			t.Errorf("Allowed(%s, %s) = %v, want %v", tt.field, tt.max, got, tt.want)
		}
	}
}

// ── Policy.CanAccess tests (Layer 1+2) ──────────────────────────

func TestCanAccess_NilPolicy(t *testing.T) {
	var p *Policy
	if !p.CanAccess("any", "thing") {
		t.Error("nil policy should allow everything")
	}
}

func TestCanAccess_EmptyConnectors(t *testing.T) {
	p := &Policy{}
	if !p.CanAccess("any", "thing") {
		t.Error("empty connectors should allow everything")
	}
}

func TestCanAccess_SpecificConnectorAndResource(t *testing.T) {
	p := &Policy{
		Connectors: map[string][]string{
			"coingecko": {"prices", "trending"},
		},
	}

	if !p.CanAccess("coingecko", "prices") {
		t.Error("should allow coingecko.prices")
	}
	if !p.CanAccess("coingecko", "trending") {
		t.Error("should allow coingecko.trending")
	}
	if p.CanAccess("coingecko", "coin") {
		t.Error("should deny coingecko.coin (not in resource list)")
	}
	if p.CanAccess("stripe", "charges") {
		t.Error("should deny stripe (not in connector list)")
	}
}

func TestCanAccess_WildcardResources(t *testing.T) {
	p := &Policy{
		Connectors: map[string][]string{
			"yahoo": {"*"},
		},
	}

	if !p.CanAccess("yahoo", "quote") {
		t.Error("should allow yahoo.* (any resource)")
	}
	if p.CanAccess("coingecko", "prices") {
		t.Error("should deny coingecko (not in list)")
	}
}

func TestCanAccess_WildcardConnector(t *testing.T) {
	p := &Policy{
		Connectors: map[string][]string{
			"*": {"*"},
		},
	}

	if !p.CanAccess("anything", "at_all") {
		t.Error("wildcard connector+resource should allow everything")
	}
}

func TestCanAccess_WildcardConnectorSpecificResources(t *testing.T) {
	p := &Policy{
		Connectors: map[string][]string{
			"*": {"prices", "quote"},
		},
	}

	if !p.CanAccess("coingecko", "prices") {
		t.Error("should allow any connector with 'prices' resource")
	}
	if p.CanAccess("coingecko", "coin") {
		t.Error("should deny 'coin' resource even with wildcard connector")
	}
}

// ── AllowedConnectorNames tests ─────────────────────────────────

func TestAllowedConnectorNames_NilPolicy(t *testing.T) {
	var p *Policy
	if names := p.AllowedConnectorNames(); names != nil {
		t.Errorf("nil policy should return nil, got %v", names)
	}
}

func TestAllowedConnectorNames_Wildcard(t *testing.T) {
	p := &Policy{
		Connectors: map[string][]string{"*": {"*"}},
	}
	if names := p.AllowedConnectorNames(); names != nil {
		t.Errorf("wildcard should return nil, got %v", names)
	}
}

func TestAllowedConnectorNames_Specific(t *testing.T) {
	p := &Policy{
		Connectors: map[string][]string{
			"coingecko": {"*"},
			"yahoo":     {"quote"},
		},
	}
	names := p.AllowedConnectorNames()
	sort.Strings(names)
	want := []string{"coingecko", "yahoo"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}
