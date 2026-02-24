package pipeline

import "testing"

func TestParseConnectorResourceRejectsInvalidConnector(t *testing.T) {
	_, _, err := ParseConnectorResource("../evil.quote")
	if err == nil {
		t.Fatal("expected ParseConnectorResource to reject invalid connector name")
	}
}

func TestParseConnectorResourceAcceptsValidConnector(t *testing.T) {
	connector, resource, err := ParseConnectorResource("coingecko.prices")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if connector != "coingecko" || resource != "prices" {
		t.Fatalf("unexpected parse result: connector=%q resource=%q", connector, resource)
	}
}
