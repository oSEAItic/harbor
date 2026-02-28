package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/protocol"
)

// RemoteExecutor calls the Harbor Cloud gateway API to execute connectors.
type RemoteExecutor struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

// NewRemoteExecutor creates an executor that calls a remote Harbor gateway.
func NewRemoteExecutor(endpoint, apiKey string) *RemoteExecutor {
	return &RemoteExecutor{
		endpoint: endpoint,
		apiKey:   apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute sends the request to the remote gateway and returns the response.
func (e *RemoteExecutor) Execute(ctx context.Context, req protocol.Request) (*protocol.Response, error) {
	if req.Auth == "" {
		if token, err := auth.Retrieve(req.Connector); err == nil {
			req.Auth = token
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", e.apiKey)

	httpResp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling harbor cloud: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 10<<20)) // 10 MB max
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	switch httpResp.StatusCode {
	case http.StatusOK:
		var resp protocol.Response
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}
		return &resp, nil

	case http.StatusUnauthorized:
		return nil, fmt.Errorf("harbor cloud: unauthorized (check your API key with 'harbor login')")

	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("harbor cloud: rate limit exceeded")

	case http.StatusBadGateway:
		// Gateway may return a structured protocol.Response with error details
		var resp protocol.Response
		if err := json.Unmarshal(respBody, &resp); err == nil && len(resp.Errors) > 0 {
			return &resp, nil
		}
		return nil, fmt.Errorf("harbor cloud: gateway error: %s", clip(string(respBody), 200))

	default:
		return nil, fmt.Errorf("harbor cloud: unexpected status %d: %s", httpResp.StatusCode, clip(string(respBody), 200))
	}
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
