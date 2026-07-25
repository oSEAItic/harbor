package farm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
)

const (
	telemetryHeader   = "x-harbor-receiver-token"
	maxTelemetryBytes = 2 << 20
)

type telemetryConfig struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

type TelemetryStatus struct {
	Endpoint            string `json:"endpoint"`
	ConfigPath          string `json:"config_path"`
	ConfigSnippet       string `json:"config_snippet"`
	ClaudeConfigPath    string `json:"claude_config_path"`
	ClaudeConfigSnippet string `json:"claude_config_snippet"`
	ReceivedEvents      int    `json:"received_events"`
	LastEventAt         string `json:"last_event_at,omitempty"`
}

type TelemetryReceiver struct {
	client   *Client
	mu       sync.Mutex
	status   TelemetryStatus
	onStatus func(TelemetryStatus)
}

func NewTelemetryReceiver(client *Client) *TelemetryReceiver {
	return &TelemetryReceiver{client: client}
}

func (r *TelemetryReceiver) Serve(ctx context.Context, ready func(TelemetryStatus)) error {
	cfg, err := loadTelemetryConfig()
	if err != nil {
		return err
	}
	address := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil && cfg.Port != 0 {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		return fmt.Errorf("starting farm telemetry receiver: %w", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if port != cfg.Port {
		cfg.Port = port
		if err := saveTelemetryConfig(cfg); err != nil {
			return err
		}
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/v1/logs", port)
	r.status = TelemetryStatus{
		Endpoint:            endpoint,
		ConfigPath:          filepath.Join(userHome(), ".codex", "config.toml"),
		ConfigSnippet:       fmt.Sprintf("[otel]\nlog_user_prompt = false\nexporter = { otlp-http = { endpoint = %q, protocol = \"json\", headers = { %q = %q } } }", endpoint, telemetryHeader, cfg.Token),
		ClaudeConfigPath:    filepath.Join(userHome(), ".claude", "settings.json"),
		ClaudeConfigSnippet: claudeTelemetrySnippet(endpoint, cfg.Token),
	}
	r.onStatus = ready
	if ready != nil {
		ready(r.status)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/logs", func(w http.ResponseWriter, req *http.Request) {
		r.handleLogs(w, req, cfg.Token)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (r *TelemetryReceiver) handleLogs(w http.ResponseWriter, req *http.Request, expectedToken string) {
	provided := req.Header.Get(telemetryHeader)
	if len(provided) != len(expectedToken) || subtle.ConstantTimeCompare([]byte(provided), []byte(expectedToken)) != 1 {
		writeTelemetryJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid receiver token"})
		return
	}
	if !strings.Contains(strings.ToLower(req.Header.Get("Content-Type")), "application/json") {
		writeTelemetryJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Harbor accepts OTLP JSON only"})
		return
	}
	limited := http.MaxBytesReader(w, req.Body, maxTelemetryBytes)
	var payload otelLogsPayload
	if err := json.NewDecoder(limited).Decode(&payload); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "request body too large") {
			status = http.StatusRequestEntityTooLarge
		}
		writeTelemetryJSON(w, status, map[string]string{"error": "invalid OTLP JSON"})
		return
	}
	events := parseOTel(payload)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, event := range events {
		if err := r.client.Record(req.Context(), event); err != nil {
			// Record queues before uploading, so a cloud outage is not an ingest failure.
			if !isQueuedFarmError(err) {
				writeTelemetryJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to queue event"})
				return
			}
		}
	}
	r.status.ReceivedEvents += len(events)
	if len(events) > 0 {
		r.status.LastEventAt = time.Now().UTC().Format(time.RFC3339)
		if r.onStatus != nil {
			r.onStatus(r.status)
		}
	}
	writeTelemetryJSON(w, http.StatusOK, map[string]any{"partialSuccess": map[string]any{}})
}

type otelValue struct {
	StringValue string          `json:"stringValue"`
	IntValue    json.RawMessage `json:"intValue"`
	DoubleValue *float64        `json:"doubleValue"`
	BoolValue   *bool           `json:"boolValue"`
}

type otelAttribute struct {
	Key   string    `json:"key"`
	Value otelValue `json:"value"`
}

type otelLogRecord struct {
	EventName            string          `json:"eventName"`
	TimeUnixNano         string          `json:"timeUnixNano"`
	ObservedTimeUnixNano string          `json:"observedTimeUnixNano"`
	TraceID              string          `json:"traceId"`
	SpanID               string          `json:"spanId"`
	Body                 otelValue       `json:"body"`
	Attributes           []otelAttribute `json:"attributes"`
}

type otelScopeLog struct {
	LogRecords []otelLogRecord `json:"logRecords"`
}
type otelResourceLog struct {
	Resource struct {
		Attributes []otelAttribute `json:"attributes"`
	} `json:"resource"`
	ScopeLogs []otelScopeLog `json:"scopeLogs"`
}
type otelLogsPayload struct {
	ResourceLogs []otelResourceLog `json:"resourceLogs"`
}

func parseOTel(payload otelLogsPayload) []Event {
	var events []Event
	for _, resourceLog := range payload.ResourceLogs {
		resourceAttrs := otelAttributes(resourceLog.Resource.Attributes)
		for _, scope := range resourceLog.ScopeLogs {
			for _, record := range scope.LogRecords {
				attrs := make(map[string]any, len(resourceAttrs)+len(record.Attributes))
				for key, value := range resourceAttrs {
					attrs[key] = value
				}
				for key, value := range otelAttributes(record.Attributes) {
					attrs[key] = value
				}
				name := record.EventName
				if name == "" {
					name = stringAttr(attrs, "event.name")
				}
				if name == "" {
					name, _ = otelValueOf(record.Body).(string)
				}
				serviceName := stringAttr(attrs, "service.name")
				if name != "" && !strings.Contains(name, ".") && strings.HasPrefix(serviceName, "claude-code") {
					name = "claude_code." + name
				}
				if strings.HasPrefix(name, "codex.") {
					events = append(events, normalizeCodex(name, attrs, record)...)
				} else if strings.HasPrefix(name, "claude_code.") {
					events = append(events, normalizeClaude(name, attrs, record)...)
				}
			}
		}
	}
	return events
}

func normalizeCodex(name string, attrs map[string]any, record otelLogRecord) []Event {
	sessionID := firstStringAttr(attrs, "conversation.id", "conversation_id")
	if sessionID == "" {
		return nil
	}
	modelName := firstStringAttr(attrs, "model", "server_model")
	eventKind := firstStringAttr(attrs, "event.kind", "event_kind")
	toolName := firstStringAttr(attrs, "tool_name", "tool.name")
	input := numberAttr(attrs, "input_token_count", "input_tokens")
	output := numberAttr(attrs, "output_token_count", "output_tokens")
	base := eventBase("codex-otel", sessionID, modelName, record, name, eventKind, toolName, input, output)
	switch {
	case name == "codex.conversation_starts":
		return []Event{statusEvent(base, "idle", "status")}
	case name == "codex.user_prompt" || name == "codex.api_request" || name == "codex.websocket_request":
		return []Event{statusEvent(base, "running_model", "status")}
	case name == "codex.tool_decision" && toolName != "":
		return []Event{statusEvent(base, "running_tool", "status"), toolEvent(base, toolName, "tool")}
	case name == "codex.tool_result":
		return []Event{statusEvent(base, "running_model", "status")}
	case (name == "codex.sse_event" || name == "codex.websocket_event") && eventKind == "response.completed":
		result := make([]Event, 0, 2)
		if input > 0 || output > 0 {
			result = append(result, usageEvent(base, input, output, "usage"))
		}
		return append(result, statusEvent(base, "waiting_input", "status"))
	default:
		return nil
	}
}

func normalizeClaude(name string, attrs map[string]any, record otelLogRecord) []Event {
	sessionID := firstStringAttr(attrs, "session.id", "session_id")
	if sessionID == "" {
		return nil
	}
	desktop := stringAttr(attrs, "service.name") == "claude-code-desktop"
	source := "claude-code-otel"
	if desktop {
		source = "claude-desktop-otel"
	}
	modelName := stringAttr(attrs, "model")
	toolName := stringAttr(attrs, "tool_name")
	decision := stringAttr(attrs, "decision")
	input := numberAttr(attrs, "input_tokens") + numberAttr(attrs, "cache_read_tokens") + numberAttr(attrs, "cache_creation_tokens")
	output := numberAttr(attrs, "output_tokens")
	base := eventBase(source, sessionID, modelName, record, name, stringAttr(attrs, "event.sequence"), stringAttr(attrs, "request_id"), toolName, input, output)
	switch {
	case name == "claude_code.user_prompt":
		return []Event{statusEvent(base, "running_model", "status")}
	case name == "claude_code.api_request":
		result := make([]Event, 0, 2)
		if input > 0 || output > 0 {
			result = append(result, usageEvent(base, input, output, "usage"))
		}
		return append(result, statusEvent(base, "waiting_input", "status"))
	case name == "claude_code.assistant_response":
		return []Event{statusEvent(base, "waiting_input", "status")}
	case name == "claude_code.tool_decision" && toolName != "":
		if decision == "reject" {
			return []Event{statusEvent(base, "running_model", "status")}
		}
		return []Event{statusEvent(base, "running_tool", "status"), toolEvent(base, toolName, "tool")}
	case name == "claude_code.tool_result":
		return []Event{statusEvent(base, "running_model", "status")}
	default:
		return nil
	}
}

func eventBase(source, sessionID, modelName string, record otelLogRecord, identity ...any) Event {
	parts := []string{record.TimeUnixNano, record.ObservedTimeUnixNano, record.TraceID, record.SpanID, sessionID}
	for _, value := range identity {
		parts = append(parts, fmt.Sprint(value))
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return Event{
		EventID: source + ":" + hex.EncodeToString(digest[:]), ExternalSessionID: sessionID,
		Source: source, Surface: "external", Model: modelName, OccurredAt: recordTime(record),
		UsageQuality: "unavailable", Metadata: EventMetadata{RuntimeID: source, Protocol: "external"},
	}
}

func statusEvent(base Event, state, suffix string) Event {
	base.EventID += ":" + suffix
	base.EventType = "status"
	base.State = state
	return base
}

func toolEvent(base Event, toolName, suffix string) Event {
	base.EventID += ":" + suffix
	base.EventType = "tool"
	base.State = "running_tool"
	base.Metadata.ToolName = toolName
	return base
}

func usageEvent(base Event, input, output int, suffix string) Event {
	base.EventID += ":" + suffix
	base.EventType = "usage"
	base.InputTokensDelta = input
	base.OutputTokensDelta = output
	base.UsageQuality = "provider_reported"
	return base
}

func otelAttributes(values []otelAttribute) map[string]any {
	result := make(map[string]any, len(values))
	for _, attribute := range values {
		if attribute.Key == "" {
			continue
		}
		if value := otelValueOf(attribute.Value); value != nil {
			result[attribute.Key] = value
		}
	}
	return result
}

func otelValueOf(value otelValue) any {
	if value.StringValue != "" {
		return value.StringValue
	}
	if len(value.IntValue) > 0 {
		var number json.Number
		if value.IntValue[0] == '"' {
			var text string
			if json.Unmarshal(value.IntValue, &text) == nil {
				number = json.Number(text)
			}
		} else {
			number = json.Number(string(value.IntValue))
		}
		if parsed, err := number.Int64(); err == nil {
			return parsed
		}
	}
	if value.DoubleValue != nil {
		return *value.DoubleValue
	}
	if value.BoolValue != nil {
		return *value.BoolValue
	}
	return nil
}

func stringAttr(attrs map[string]any, key string) string {
	value, _ := attrs[key].(string)
	return value
}

func firstStringAttr(attrs map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringAttr(attrs, key); value != "" {
			return value
		}
	}
	return ""
}

func numberAttr(attrs map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := attrs[key].(type) {
		case int64:
			return max(0, int(value))
		case float64:
			return max(0, int(value))
		case string:
			if parsed, err := strconv.Atoi(value); err == nil {
				return max(0, parsed)
			}
		}
	}
	return 0
}

func recordTime(record otelLogRecord) time.Time {
	for _, raw := range []string{record.TimeUnixNano, record.ObservedTimeUnixNano} {
		if nanos, err := strconv.ParseInt(raw, 10, 64); err == nil && nanos > 0 {
			return time.Unix(0, nanos).UTC()
		}
	}
	return time.Now().UTC()
}

func loadTelemetryConfig() (telemetryConfig, error) {
	path := harborhome.Path("farm-telemetry.json")
	data, err := os.ReadFile(path)
	if err == nil {
		var cfg telemetryConfig
		if json.Unmarshal(data, &cfg) == nil && len(cfg.Token) >= 32 {
			return cfg, nil
		}
	}
	bytes := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return telemetryConfig{}, err
	}
	cfg := telemetryConfig{Token: hex.EncodeToString(bytes)}
	return cfg, saveTelemetryConfig(cfg)
}

func saveTelemetryConfig(cfg telemetryConfig) error {
	path := harborhome.Path("farm-telemetry.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func claudeTelemetrySnippet(endpoint, token string) string {
	payload := map[string]any{"env": map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY": "1", "OTEL_METRICS_EXPORTER": "none",
		"OTEL_LOGS_EXPORTER": "otlp", "OTEL_EXPORTER_OTLP_LOGS_PROTOCOL": "http/json",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": endpoint, "OTEL_EXPORTER_OTLP_HEADERS": telemetryHeader + "=" + token,
		"OTEL_LOG_USER_PROMPTS": "0", "OTEL_LOG_ASSISTANT_RESPONSES": "0",
		"OTEL_LOG_TOOL_DETAILS": "0", "OTEL_LOG_TOOL_CONTENT": "0", "OTEL_LOG_RAW_API_BODIES": "0",
	}}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}

func writeTelemetryJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func userHome() string {
	home, _ := os.UserHomeDir()
	return home
}

func isQueuedFarmError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "contacting Harbor Cloud") || strings.Contains(err.Error(), "farm is offline"))
}
