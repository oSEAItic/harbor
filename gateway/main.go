// Package main is the harbor-gateway HTTP server.
// It executes connectors and serves the /run endpoint to harbor-cloud (or any authorized caller).
//
// Required environment variables in production:
//
//	HARBOR_GATEWAY_API_KEYS    comma-separated API keys (e.g. "key1,key2")
//	HARBOR_GATEWAY_JWT_SECRET  HS256 signing secret
//
// WARNING: if both variables are unset, the gateway allows unauthenticated access.
// Always configure at least one in production deployments.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oseaitic/harbor/internal/connector"
	harborctx "github.com/oseaitic/harbor/internal/context"
	"github.com/oseaitic/harbor/internal/executor"
	"github.com/oseaitic/harbor/internal/govern"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/pipeline"
	"github.com/oseaitic/harbor/internal/protocol"
)

type gatewayConfig struct {
	APIKeys      map[string]struct{}
	JWTSecret    string
	RatePerMin   int
	AuditLogPath string
}

type authContext struct {
	Actor string
	Type  string
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]rateWindow
}

type rateWindow struct {
	Minute int64
	Count  int
}

type auditEvent struct {
	TimeMs              int64  `json:"time_ms"`
	Actor               string `json:"actor"`
	AuthType            string `json:"auth_type"`
	Connector           string `json:"connector"`
	Resource            string `json:"resource"`
	StatusCode          int    `json:"status_code"`
	DurationMs          int64  `json:"duration_ms"`
	FromMemory          bool   `json:"from_memory"`
	Error               string `json:"error,omitempty"`
	RemoteAddr          string `json:"remote_addr,omitempty"`
	RequestPath         string `json:"request_path"`
	MaxVisibility       string `json:"max_visibility,omitempty"`
	GovernFieldsRemoved int    `json:"govern_fields_removed,omitempty"`
	PolicyApplied       bool   `json:"policy_applied,omitempty"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	cfg := loadConfig()
	limiter := newRateLimiter(cfg.RatePerMin)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "harbor-gateway",
		})
	})

	// Tool discovery — returns all installed connector schemas.
	// Used by harbor-cloud MCP server to register tools dynamically.
	mux.HandleFunc("GET /tools", func(w http.ResponseWriter, r *http.Request) {
		auth, authErr := authorize(r, cfg)
		if authErr != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = auth

		schemas, err := connector.ExportToolSchemas()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Govern Layer 1: filter schemas by allowed connectors.
		if connFilter := r.URL.Query().Get("connectors"); connFilter != "" {
			allowed := strings.Split(connFilter, ",")
			schemas = filterSchemasByConnector(schemas, allowed)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schemas)
	})

	// Memory recall — search and retrieve from the gateway's memory store.
	// Used by harbor-cloud MCP server for the harbor_recall tool.
	memStore, _ := memory.NewStore()
	mux.HandleFunc("POST /recall", func(w http.ResponseWriter, r *http.Request) {
		auth, authErr := authorize(r, cfg)
		if authErr != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = auth

		var req struct {
			ID        string `json:"id"`
			Query     string `json:"query"`
			Layer     string `json:"layer"`
			Connector string `json:"connector"`
			Since     string `json:"since"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if memStore == nil {
			json.NewEncoder(w).Encode(map[string]string{"error": "memory store not available"})
			return
		}

		// Retrieve mode: return a specific memory object
		if req.ID != "" {
			obj, err := memStore.Get(req.ID)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("memory %q not found", req.ID)})
				return
			}
			layer := req.Layer
			if layer == "" {
				layer = "compact"
			}
			var content json.RawMessage
			switch layer {
			case "raw":
				content = obj.Layers.Raw
			case "normalized":
				content = obj.Layers.Normalized
			case "summary":
				content, _ = json.Marshal(obj.Layers.Summary)
			default:
				content = obj.Layers.Compact
			}
			if len(content) == 0 {
				content = obj.Layers.Compact
			}
			if len(content) == 0 {
				content = obj.Layers.Raw
			}
			json.NewEncoder(w).Encode(map[string]json.RawMessage{"content": content})
			return
		}

		// Search/browse mode
		opts := memory.QueryOptions{
			Connector: req.Connector,
			Limit:     30,
		}
		if req.Since != "" {
			if d, err := time.ParseDuration(req.Since); err == nil {
				opts.Since = d
			}
		}

		results := memStore.Search(req.Query, opts)
		json.NewEncoder(w).Encode(results)
	})

	// Memory remember — save an agent's analysis note for a connector.
	// Used by harbor-cloud MCP server for the harbor_remember tool.
	mux.HandleFunc("POST /remember", func(w http.ResponseWriter, r *http.Request) {
		auth, authErr := authorize(r, cfg)
		if authErr != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = auth

		var req struct {
			Connector string `json:"connector"`
			Note      string `json:"note"`
			Author    string `json:"author"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Connector == "" || req.Note == "" {
			http.Error(w, `{"error":"connector and note are required"}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if memStore == nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "memory store not available"})
			return
		}

		id, err := memStore.SaveNote(req.Connector, req.Note, req.Author)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("save failed: %v", err)})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status":    "saved",
			"connector": req.Connector,
			"memory_id": id,
		})
	})

	mux.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		auth, authErr := authorize(r, cfg)
		if authErr != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			logAudit(cfg, auditEvent{
				TimeMs:      time.Now().UnixMilli(),
				Actor:       "anonymous",
				AuthType:    "none",
				StatusCode:  http.StatusUnauthorized,
				DurationMs:  time.Since(start).Milliseconds(),
				Error:       authErr.Error(),
				RemoteAddr:  r.RemoteAddr,
				RequestPath: r.URL.Path,
			})
			return
		}

		// Parse request body as Request + optional Policy.
		var body struct {
			protocol.Request
			Policy *govern.Policy `json:"policy,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			logAudit(cfg, auditEvent{
				TimeMs:      time.Now().UnixMilli(),
				Actor:       auth.Actor,
				AuthType:    auth.Type,
				StatusCode:  http.StatusBadRequest,
				DurationMs:  time.Since(start).Milliseconds(),
				Error:       "invalid request body",
				RemoteAddr:  r.RemoteAddr,
				RequestPath: r.URL.Path,
			})
			return
		}
		req := body.Request
		policy := body.Policy

		if req.Connector == "" || req.Resource == "" {
			http.Error(w, `{"error":"connector and resource are required"}`, http.StatusBadRequest)
			logAudit(cfg, auditEvent{
				TimeMs:      time.Now().UnixMilli(),
				Actor:       auth.Actor,
				AuthType:    auth.Type,
				Connector:   req.Connector,
				Resource:    req.Resource,
				StatusCode:  http.StatusBadRequest,
				DurationMs:  time.Since(start).Milliseconds(),
				Error:       "missing connector/resource",
				RemoteAddr:  r.RemoteAddr,
				RequestPath: r.URL.Path,
			})
			return
		}

		// Govern Layer 1+2: check connector/resource access.
		if policy != nil && !policy.CanAccess(req.Connector, req.Resource) {
			http.Error(w, `{"error":"forbidden: connector/resource not allowed by policy"}`, http.StatusForbidden)
			logAudit(cfg, auditEvent{
				TimeMs:        time.Now().UnixMilli(),
				Actor:         auth.Actor,
				AuthType:      auth.Type,
				Connector:     req.Connector,
				Resource:      req.Resource,
				StatusCode:    http.StatusForbidden,
				DurationMs:    time.Since(start).Milliseconds(),
				Error:         "policy denied connector/resource",
				PolicyApplied: true,
				RemoteAddr:    r.RemoteAddr,
				RequestPath:   r.URL.Path,
			})
			return
		}

		if !limiter.Allow(auth.Actor + ":" + req.Connector) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			logAudit(cfg, auditEvent{
				TimeMs:      time.Now().UnixMilli(),
				Actor:       auth.Actor,
				AuthType:    auth.Type,
				Connector:   req.Connector,
				Resource:    req.Resource,
				StatusCode:  http.StatusTooManyRequests,
				DurationMs:  time.Since(start).Milliseconds(),
				Error:       "rate limit exceeded",
				RemoteAddr:  r.RemoteAddr,
				RequestPath: r.URL.Path,
			})
			return
		}

		// Govern Layer 3: pass max_visibility + field overrides to pipeline.
		var maxVis string
		var fieldOverrides map[string]string
		if policy != nil {
			maxVis = policy.MaxVisibility
			fieldOverrides = policy.FieldOverrides
		}

		exec := executor.NewLocalExecutor()
		result, err := pipeline.Execute(exec, req.Connector, req.Resource, req.Params, nil, pipeline.Options{
			Compile:        harborctx.DefaultOptions(),
			MaxVisibility:  maxVis,
			FieldOverrides: fieldOverrides,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			errResp := protocol.NewErrorResponse(req.Connector, "", protocol.ErrExecution, err.Error())
			json.NewEncoder(w).Encode(errResp)
			logAudit(cfg, auditEvent{
				TimeMs:      time.Now().UnixMilli(),
				Actor:       auth.Actor,
				AuthType:    auth.Type,
				Connector:   req.Connector,
				Resource:    req.Resource,
				StatusCode:  http.StatusBadGateway,
				DurationMs:  time.Since(start).Milliseconds(),
				Error:       err.Error(),
				RemoteAddr:  r.RemoteAddr,
				RequestPath: r.URL.Path,
			})
			return
		}

		respBytes, _ := json.Marshal(result.Response)

		w.Header().Set("Content-Type", "application/json")
		if result.FromMem {
			w.Header().Set("X-Harbor-Cache", "hit")
		} else {
			w.Header().Set("X-Harbor-Cache", "miss")
		}
		if result.GovernFieldsRemoved > 0 {
			w.Header().Set("X-Harbor-Govern-Removed", strconv.Itoa(result.GovernFieldsRemoved))
		}
		w.Write(respBytes)

		logAudit(cfg, auditEvent{
			TimeMs:              time.Now().UnixMilli(),
			Actor:               auth.Actor,
			AuthType:            auth.Type,
			Connector:           req.Connector,
			Resource:            req.Resource,
			StatusCode:          http.StatusOK,
			DurationMs:          time.Since(start).Milliseconds(),
			FromMemory:          result.FromMem,
			RemoteAddr:          r.RemoteAddr,
			RequestPath:         r.URL.Path,
			MaxVisibility:       maxVis,
			GovernFieldsRemoved: result.GovernFieldsRemoved,
			PolicyApplied:       policy != nil,
		})
	})

	log.Printf("Harbor Gateway listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() gatewayConfig {
	cfg := gatewayConfig{
		APIKeys:      map[string]struct{}{},
		JWTSecret:    strings.TrimSpace(os.Getenv("HARBOR_GATEWAY_JWT_SECRET")),
		AuditLogPath: strings.TrimSpace(os.Getenv("HARBOR_GATEWAY_AUDIT_LOG")),
		RatePerMin:   120,
	}
	if v := strings.TrimSpace(os.Getenv("HARBOR_GATEWAY_RATE_PER_MINUTE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.RatePerMin = n
		}
	}

	for _, k := range strings.Split(strings.TrimSpace(os.Getenv("HARBOR_GATEWAY_API_KEYS")), ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			cfg.APIKeys[k] = struct{}{}
		}
	}
	return cfg
}

func authorize(r *http.Request, cfg gatewayConfig) (authContext, error) {
	rawKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
	if rawKey == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			rawKey = strings.TrimSpace(auth[7:])
		}
	}

	if len(cfg.APIKeys) == 0 && cfg.JWTSecret == "" {
		return authContext{Actor: "anonymous", Type: "none"}, nil
	}

	if rawKey == "" {
		return authContext{}, fmt.Errorf("missing credentials")
	}

	if _, ok := cfg.APIKeys[rawKey]; ok {
		return authContext{
			Actor: "apikey:" + hashToken(rawKey),
			Type:  "api_key",
		}, nil
	}

	if cfg.JWTSecret == "" {
		return authContext{}, fmt.Errorf("invalid api key")
	}

	sub, err := parseHS256JWT(rawKey, cfg.JWTSecret)
	if err != nil {
		return authContext{}, fmt.Errorf("invalid jwt")
	}
	if strings.TrimSpace(sub) == "" {
		sub = "anonymous"
	}
	return authContext{Actor: "jwt:" + sub, Type: "jwt"}, nil
}

func hashToken(in string) string {
	sum := sha256.Sum256([]byte(in))
	return hex.EncodeToString(sum[:])[:8]
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		windows: map[string]rateWindow{},
	}
}

func (rl *rateLimiter) Allow(key string) bool {
	if rl == nil {
		return true
	}

	nowMinute := time.Now().Unix() / 60
	rl.mu.Lock()
	defer rl.mu.Unlock()

	w := rl.windows[key]
	if w.Minute != nowMinute {
		w = rateWindow{Minute: nowMinute, Count: 0}
	}
	if w.Count >= rl.limit {
		rl.windows[key] = w
		return false
	}
	w.Count++
	rl.windows[key] = w
	return true
}

func logAudit(cfg gatewayConfig, ev auditEvent) {
	line, err := json.Marshal(ev)
	if err != nil {
		return
	}
	if cfg.AuditLogPath == "" {
		log.Printf("audit %s", line)
		return
	}
	f, err := os.OpenFile(cfg.AuditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("audit log open failed: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// filterSchemasByConnector returns only schemas whose connector prefix matches
// one of the allowed names. Connector name is derived from the "connector.resource" tool name.
func filterSchemasByConnector(schemas []protocol.ToolSchema, allowed []string) []protocol.ToolSchema {
	set := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		set[strings.TrimSpace(name)] = true
	}
	var filtered []protocol.ToolSchema
	for _, s := range schemas {
		if parts := strings.SplitN(s.Function.Name, ".", 2); len(parts) == 2 && set[parts[0]] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func parseHS256JWT(token, secret string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed jwt")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decode header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return "", fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "HS256" {
		return "", fmt.Errorf("unsupported alg %q", header.Alg)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}
	if !hmac.Equal(sig, expected) {
		return "", fmt.Errorf("signature mismatch")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return "", fmt.Errorf("parse payload: %w", err)
	}

	if expVal, ok := claims["exp"]; ok {
		expFloat, ok := expVal.(float64)
		if !ok {
			return "", fmt.Errorf("invalid exp claim")
		}
		if time.Now().Unix() >= int64(expFloat) {
			return "", fmt.Errorf("token expired")
		}
	}

	sub, _ := claims["sub"].(string)
	return sub, nil
}
