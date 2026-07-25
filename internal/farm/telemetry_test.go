package farm

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelemetryReceiverRejectsMissingToken(t *testing.T) {
	receiver := NewTelemetryReceiver(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader([]byte(`{"resourceLogs":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	receiver.handleLogs(w, req, "secret-token")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestParseOTelCodexUsageIsMetadataOnly(t *testing.T) {
	raw := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"codex"}}]},"scopeLogs":[{"logRecords":[{"eventName":"codex.sse_event","timeUnixNano":"1750000000000000000","attributes":[{"key":"conversation.id","value":{"stringValue":"session-1"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"input_token_count","value":{"intValue":"120"}},{"key":"output_token_count","value":{"intValue":"30"}},{"key":"prompt","value":{"stringValue":"private"}}]}]}]}]}`
	var payload otelLogsPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	events := parseOTel(payload)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].EventType != "usage" || events[0].InputTokensDelta != 120 || events[0].OutputTokensDelta != 30 {
		t.Fatalf("unexpected usage event: %#v", events[0])
	}
	encoded, _ := json.Marshal(events)
	if strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "prompt") {
		t.Fatalf("sensitive field escaped normalization: %s", encoded)
	}
	if events[1].State != "waiting_input" {
		t.Fatalf("got state %q", events[1].State)
	}
}

func TestParseOTelClaudeDesktop(t *testing.T) {
	raw := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"claude-code-desktop"}}]},"scopeLogs":[{"logRecords":[{"eventName":"api_request","attributes":[{"key":"session.id","value":{"stringValue":"claude-1"}},{"key":"input_tokens","value":{"intValue":"10"}},{"key":"cache_read_tokens","value":{"intValue":"4"}},{"key":"output_tokens","value":{"intValue":"3"}}]}]}]}]}`
	var payload otelLogsPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	events := parseOTel(payload)
	if len(events) != 2 || events[0].Source != "claude-desktop-otel" || events[0].InputTokensDelta != 14 {
		t.Fatalf("unexpected events: %#v", events)
	}
}
